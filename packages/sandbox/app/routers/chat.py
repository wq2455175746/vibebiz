from pathlib import Path

from fastapi import APIRouter, Query, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from app.errors import NotFoundError, WorkspaceError
from app.observability import SANDBOX_REQUESTS, SSE_EVENTS, get_logger, get_tracer
from app.response import envelope
from app.services.chat_history import ChatHistory
from app.services.chat_turns import ChatTurnManager, TurnInProgress, iter_turn_events
from app.services.events import DONE, sse
from app.services.resources import ResourceStore
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["chat"])
logger = get_logger()


class ChatReference(BaseModel):
    kind: str
    id: str | None = None
    path: str | None = None


class ChatRequest(BaseModel):
    message: str = ""
    sessionId: str | None = None
    references: list[ChatReference] = []


def _count(project_id: str, status: str) -> None:
    SANDBOX_REQUESTS.labels(operation="chat.stream", status=status, project_id=project_id).inc()


def _validate_project(project_id: str, workspace: Workspace) -> None:
    if not workspace.exists(project_id):
        _count(project_id, "404")
        raise NotFoundError("project not found")


@router.post("/{project_id}/chat")
async def chat(project_id: str, body: ChatRequest, request: Request) -> StreamingResponse:
    workspace: Workspace = request.app.state.workspace
    manager: ChatTurnManager = request.app.state.chat_turns

    message = body.message.strip()
    if not message:
        _count(project_id, "400")
        raise WorkspaceError("message is required")
    _validate_project(project_id, workspace)

    references = _resolve_references(
        project_id, workspace, request.app.state.resources, body.references
    )
    try:
        turn = await manager.start(project_id, message, workspace, body.sessionId, references)
    except TurnInProgress:
        _count(project_id, "409")
        raise

    async def generate():
        tracer = get_tracer()
        with tracer.start_as_current_span("chat.stream") as span:
            span.set_attribute("project.id", project_id)
            status = "error"
            try:
                async for event in iter_turn_events(manager, turn):
                    if event is None:
                        break
                    SSE_EVENTS.labels(
                        project_id=project_id, type=str(event.get("type", "unknown"))
                    ).inc()
                    yield sse(event)
                status = "success"
            except Exception as exc:
                logger.error(
                    "chat.stream.error",
                    extra={
                        "project_id": project_id,
                        "status": "error",
                        "code": getattr(exc, "code", 50000),
                        "error_message": str(exc),
                    },
                    exc_info=True,
                )
                yield sse({"type": "error", "message": str(exc)})
            finally:
                span.set_attribute("sse.done", True)
                _count(project_id, status)
                yield DONE

    headers = {
        "Cache-Control": "no-cache",
        "Connection": "keep-alive",
        "X-Accel-Buffering": "no",
    }
    return StreamingResponse(generate(), media_type="text/event-stream", headers=headers)


@router.post("/{project_id}/chat/stop")
async def stop_chat(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    manager: ChatTurnManager = request.app.state.chat_turns
    _validate_project(project_id, workspace)
    stopped = await manager.stop(project_id)
    return envelope({"stopped": stopped})


def _resolve_references(
    project_id: str, workspace: Workspace, store: ResourceStore, references: list[ChatReference]
) -> list[dict[str, object]]:
    if len(references) > 10:
        raise WorkspaceError("too many references")
    resources = {item["id"]: item for item in store.list(project_id)["resources"]}
    resolved: list[dict[str, object]] = []
    seen: set[tuple[str, str]] = set()
    for ref in references:
        if ref.kind == "resource":
            item = resources.get(ref.id or "")
            if item is None:
                raise WorkspaceError("resource not found")
            key = ("resource", str(item["id"]))
            if key in seen:
                continue
            seen.add(key)
            resolved.append(
                {
                    "kind": "resource",
                    "id": item["id"],
                    "name": item["displayName"],
                    "path": item["path"],
                    "contentType": item["contentType"],
                    "resourceKind": item["kind"],
                }
            )
        elif ref.kind == "project_file":
            raw = (ref.path or "").strip()
            if not raw:
                raise WorkspaceError("reference path is required")
            target = workspace._resolve(project_id, raw)
            if not target.is_file():
                raise WorkspaceError("reference file not found")
            relative = target.relative_to(workspace.project_dir(project_id)).as_posix()
            key = ("project_file", relative)
            if key in seen:
                continue
            seen.add(key)
            resolved.append(
                {
                    "kind": "project_file",
                    "name": Path(relative).name,
                    "path": relative,
                }
            )
        else:
            raise WorkspaceError("invalid reference kind")
    return resolved


@router.get("/{project_id}/chat/status")
async def chat_status(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    manager: ChatTurnManager = request.app.state.chat_turns
    _validate_project(project_id, workspace)
    return envelope(await manager.status(project_id))


@router.get("/{project_id}/chat/history")
def chat_history(
    project_id: str,
    request: Request,
    limit: int = Query(default=100, ge=1, le=500),
) -> dict:
    workspace: Workspace = request.app.state.workspace
    history: ChatHistory = request.app.state.chat_history
    _validate_project(project_id, workspace)
    tail = history.read_tail(project_id, limit)
    return envelope({"content": tail.content, "truncated": tail.truncated, "version": tail.version})

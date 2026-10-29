from fastapi import APIRouter, Query, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from app.errors import NotFoundError, WorkspaceError
from app.observability import SANDBOX_REQUESTS, SSE_EVENTS, get_logger, get_tracer
from app.response import envelope
from app.services.chat_history import ChatHistory
from app.services.chat_turns import ChatTurnManager, TurnInProgress, iter_turn_events
from app.services.events import DONE, sse
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["chat"])
logger = get_logger()


class ChatRequest(BaseModel):
    message: str = ""
    sessionId: str | None = None


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

    try:
        turn = await manager.start(project_id, message, workspace, body.sessionId)
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

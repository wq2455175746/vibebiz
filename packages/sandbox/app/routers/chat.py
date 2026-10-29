from fastapi import APIRouter, Request
from fastapi.responses import StreamingResponse
from pydantic import BaseModel

from app.errors import NotFoundError, WorkspaceError
from app.observability import SANDBOX_REQUESTS, SSE_EVENTS, get_logger, get_tracer
from app.services.agent import Agent
from app.services.events import DONE, sse
from app.services.versioning import Versioning
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["chat"])
logger = get_logger()


class ChatRequest(BaseModel):
    message: str = ""
    sessionId: str | None = None


def _count(project_id: str, status: str) -> None:
    SANDBOX_REQUESTS.labels(operation="chat.stream", status=status, project_id=project_id).inc()


@router.post("/{project_id}/chat")
async def chat(project_id: str, body: ChatRequest, request: Request) -> StreamingResponse:
    workspace: Workspace = request.app.state.workspace
    versioning: Versioning = request.app.state.versioning
    agent: Agent = request.app.state.agent

    message = body.message.strip()
    if not message:
        _count(project_id, "400")
        raise WorkspaceError("message is required")
    if not workspace.exists(project_id):
        _count(project_id, "404")
        raise NotFoundError("project not found")

    project_dir = workspace.project_dir(project_id)

    async def generate():
        tracer = get_tracer()
        with tracer.start_as_current_span("chat.stream") as span:
            span.set_attribute("project.id", project_id)
            status = "error"
            try:
                async for event in agent.run(project_id, message, workspace, body.sessionId):
                    SSE_EVENTS.labels(
                        project_id=project_id, type=str(event.get("type", "unknown"))
                    ).inc()
                    yield sse(event)
                commit = versioning.commit(project_dir, f"turn: {message[:60]}")
                SSE_EVENTS.labels(project_id=project_id, type="sandbox-status").inc()
                yield sse({"type": "sandbox-status", "state": "ready", "commit": commit})
                status = "success"
            except Exception as exc:
                SSE_EVENTS.labels(project_id=project_id, type="error").inc()
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

            SSE_EVENTS.labels(project_id=project_id, type="done").inc()
            span.set_attribute("sse.done", True)
            _count(project_id, status)
            yield DONE

    headers = {
        "Cache-Control": "no-cache",
        "Connection": "keep-alive",
        "X-Accel-Buffering": "no",
    }
    return StreamingResponse(generate(), media_type="text/event-stream", headers=headers)

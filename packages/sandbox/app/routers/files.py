from fastapi import APIRouter, Query, Request
from pydantic import BaseModel, Field

from app.errors import NotFoundError, WorkspaceError
from app.observability import SANDBOX_REQUESTS, get_tracer
from app.response import envelope
from app.routers.epoch import epoch_from_request
from app.services.chat_turns import ChatTurnManager
from app.services.project_lease import check_epoch
from app.services.project_lock import ProjectLock
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["files"])


class WriteFileRequest(BaseModel):
    path: str = Field(min_length=1)
    content: str = ""
    baseVersion: str = ""


@router.get("/{project_id}/files")
def list_files(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    return envelope({"files": workspace.list_files(project_id)})


@router.get("/{project_id}/file")
def read_file(project_id: str, request: Request, path: str = Query(min_length=1)) -> dict:
    workspace: Workspace = request.app.state.workspace
    tracer = get_tracer()
    with tracer.start_as_current_span("workspace.read") as span:
        span.set_attribute("project.id", project_id)
        span.set_attribute("file.path", path)
        try:
            result = workspace.read_file(project_id, path)
        except WorkspaceError as exc:
            SANDBOX_REQUESTS.labels(
                operation="file.read", status=str(exc.status), project_id=project_id
            ).inc()
            raise
        span.set_attribute("file.version", result["version"])
        SANDBOX_REQUESTS.labels(operation="file.read", status="ok", project_id=project_id).inc()
        return envelope(result)


@router.put("/{project_id}/file")
async def write_file(project_id: str, body: WriteFileRequest, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    turns: ChatTurnManager = request.app.state.chat_turns
    lock: ProjectLock = request.app.state.project_lock
    project_dir = workspace.project_dir(project_id)
    if not workspace.exists(project_id):
        raise NotFoundError("project not found")
    tracer = get_tracer()
    with tracer.start_as_current_span("workspace.write") as span:
        span.set_attribute("project.id", project_id)
        span.set_attribute("file.path", body.path)
        try:
            async with turns.write_guard(project_id), lock.acquire(project_dir):
                check_epoch(
                    project_dir,
                    epoch_from_request(request),
                    request.app.state.settings.sandbox_name,
                )
                result = workspace.write_file(project_id, body.path, body.content, body.baseVersion)
        except WorkspaceError as exc:
            SANDBOX_REQUESTS.labels(
                operation="file.write", status=str(exc.status), project_id=project_id
            ).inc()
            raise
        span.set_attribute("file.version", result["version"])
        SANDBOX_REQUESTS.labels(operation="file.write", status="ok", project_id=project_id).inc()
        return envelope(result)

from fastapi import APIRouter, Query, Request
from pydantic import BaseModel, Field

from app.errors import WorkspaceError
from app.observability import SANDBOX_REQUESTS, get_tracer
from app.response import envelope
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
def write_file(project_id: str, body: WriteFileRequest, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    tracer = get_tracer()
    with tracer.start_as_current_span("workspace.write") as span:
        span.set_attribute("project.id", project_id)
        span.set_attribute("file.path", body.path)
        try:
            result = workspace.write_file(project_id, body.path, body.content, body.baseVersion)
        except WorkspaceError as exc:
            SANDBOX_REQUESTS.labels(
                operation="file.write", status=str(exc.status), project_id=project_id
            ).inc()
            raise
        span.set_attribute("file.version", result["version"])
        SANDBOX_REQUESTS.labels(operation="file.write", status="ok", project_id=project_id).inc()
        return envelope(result)

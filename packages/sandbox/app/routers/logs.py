from fastapi import APIRouter, Query, Request

from app.errors import NotFoundError
from app.response import envelope
from app.services.project_logs import ProjectLogStore
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["logs"])


@router.get("/{project_id}/logs")
def project_logs(
    project_id: str,
    request: Request,
    limit: int = Query(default=100, ge=1, le=500),
) -> dict:
    workspace: Workspace = request.app.state.workspace
    if not workspace.exists(project_id):
        raise NotFoundError("project not found")
    store: ProjectLogStore = request.app.state.project_logs
    return envelope({"logs": store.read_latest(project_id, limit), "limit": limit})

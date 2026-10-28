from fastapi import APIRouter, Request

from app.response import envelope
from app.services.versioning import Versioning
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["projects"])


@router.post("/{project_id}/init")
def init_project(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    versioning: Versioning = request.app.state.versioning
    project_dir = workspace.init_project(project_id)
    versioning.init(project_dir)
    return envelope({"projectId": project_id})


@router.get("/{project_id}")
def get_project(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    return envelope({"projectId": project_id, "exists": workspace.exists(project_id)})

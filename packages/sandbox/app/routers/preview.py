from fastapi import APIRouter, Request
from fastapi.responses import FileResponse

from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["preview"])


@router.get("/{project_id}/preview")
@router.get("/{project_id}/preview/{path:path}")
def preview(project_id: str, request: Request, path: str = "") -> FileResponse:
    workspace: Workspace = request.app.state.workspace
    target = workspace.preview_path(project_id, path)
    return FileResponse(target, headers={"Cache-Control": "no-store"})

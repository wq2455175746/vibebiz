from fastapi import APIRouter, Request, Response

from app.errors import WorkspaceError
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


@router.get("/{project_id}/stats")
def project_stats(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    return envelope(workspace.stats(project_id))


@router.get("/{project_id}/archive")
def export_archive(project_id: str, request: Request) -> Response:
    workspace: Workspace = request.app.state.workspace
    data = workspace.archive(project_id)
    return Response(
        content=data,
        media_type="application/gzip",
        headers={"Content-Disposition": f'attachment; filename="{project_id}.tar.gz"'},
    )


@router.post("/{project_id}/archive/import")
async def import_archive(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    data = await request.body()
    if not data or len(data) > 100 * 1024 * 1024:
        raise WorkspaceError("invalid archive")
    workspace.import_archive(project_id, data)
    return envelope({"projectId": project_id})

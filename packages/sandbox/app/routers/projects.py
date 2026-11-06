from fastapi import APIRouter, Request, Response

from app.errors import NotFoundError, WorkspaceError
from app.response import envelope
from app.routers.epoch import epoch_from_request
from app.services.project_lease import check_epoch
from app.services.project_lock import ProjectLock
from app.services.versioning import Versioning
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["projects"])


@router.post("/{project_id}/init")
def init_project(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    versioning: Versioning = request.app.state.versioning
    lock: ProjectLock = request.app.state.project_lock
    project_dir = workspace.project_dir(project_id)
    with lock.acquire_sync(project_dir):
        check_epoch(
            project_dir, epoch_from_request(request), request.app.state.settings.sandbox_name
        )
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
    lock: ProjectLock = request.app.state.project_lock
    data = await request.body()
    if not data or len(data) > 100 * 1024 * 1024:
        raise WorkspaceError("invalid archive")
    project_dir = workspace.project_dir(project_id)
    if not workspace.exists(project_id):
        raise NotFoundError("project not found")
    async with lock.acquire(project_dir):
        check_epoch(
            project_dir, epoch_from_request(request), request.app.state.settings.sandbox_name
        )
        workspace.import_archive(project_id, data)
    return envelope({"projectId": project_id})

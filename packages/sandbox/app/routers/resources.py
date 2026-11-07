from urllib.parse import unquote_plus

from fastapi import APIRouter, Request

from app.observability import SANDBOX_REQUESTS
from app.response import envelope
from app.routers.epoch import epoch_from_request
from app.services.project_lease import check_epoch
from app.services.project_lock import ProjectLock
from app.services.resources import ResourceStore
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["resources"])


@router.get("/{project_id}/resources")
async def list_resources(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    store: ResourceStore = request.app.state.resources
    workspace._existing_project(project_id)
    return envelope(store.list(project_id))


@router.put("/{project_id}/resources/{resource_id}")
async def write_resource(project_id: str, resource_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    store: ResourceStore = request.app.state.resources
    lock: ProjectLock = request.app.state.project_lock
    workspace._existing_project(project_id)
    project_dir = workspace.project_dir(project_id)
    meta = {
        "id": resource_id,
        "name": unquote_plus(request.headers.get("X-Resource-Name", "")),
        "displayName": unquote_plus(request.headers.get("X-Resource-Display-Name", "")),
        "storageName": request.headers.get("X-Resource-Storage-Name", ""),
        "contentType": request.headers.get("X-Resource-Content-Type", ""),
        "s3Key": request.headers.get("X-Resource-S3-Key", ""),
        "kind": request.headers.get("X-Resource-Kind", ""),
        "purpose": request.headers.get("X-Resource-Purpose", ""),
    }
    body = await request.body()
    try:
        async with lock.acquire(project_dir):
            check_epoch(
                project_dir,
                epoch_from_request(request),
                request.app.state.settings.sandbox_name,
            )
            result = await store.write(project_id, meta, body)
    except Exception as exc:
        SANDBOX_REQUESTS.labels(
            operation="resource.write",
            status=str(getattr(exc, "status", 400)),
            project_id=project_id,
        ).inc()
        raise
    SANDBOX_REQUESTS.labels(operation="resource.write", status="ok", project_id=project_id).inc()
    return envelope(result)


@router.delete("/{project_id}/resources/{resource_id}")
async def delete_resource(project_id: str, resource_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    store: ResourceStore = request.app.state.resources
    lock: ProjectLock = request.app.state.project_lock
    workspace._existing_project(project_id)
    project_dir = workspace.project_dir(project_id)
    async with lock.acquire(project_dir):
        check_epoch(
            project_dir,
            epoch_from_request(request),
            request.app.state.settings.sandbox_name,
        )
        deleted = await store.delete(project_id, resource_id)
    return envelope({"deleted": deleted})

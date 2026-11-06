from fastapi import APIRouter, File, Form, Query, Request, UploadFile

from app.errors import NotFoundError, WorkspaceError
from app.observability import SANDBOX_REQUESTS, get_logger
from app.response import envelope
from app.services.skill_catalog import list_skills
from app.services.skill_store import (
    delete_skill,
    list_stored_skills,
    resolve_skills_root,
    save_skill_package,
)
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal", tags=["skills"])
logger = get_logger()


@router.get("/projects/{project_id}/skills")
def list_project_skills(
    project_id: str,
    request: Request,
    user_id: int = Query(0, alias="userId"),
) -> dict:
    workspace: Workspace = request.app.state.workspace
    settings = request.app.state.settings
    if not workspace.exists(project_id):
        SANDBOX_REQUESTS.labels(operation="skills.list", status="404", project_id=project_id).inc()
        raise NotFoundError("project not found")

    items = list_skills(settings, workspace.project_dir(project_id), user_id=user_id)
    SANDBOX_REQUESTS.labels(operation="skills.list", status="ok", project_id=project_id).inc()
    logger.info(
        "skills.list",
        extra={"project_id": project_id, "status": "ok", "count": len(items)},
    )
    return envelope({"skills": items})


@router.get("/skills")
def list_stored(
    request: Request, scope: str = "all", user_id: int = Query(0, alias="userId")
) -> dict:
    settings = request.app.state.settings
    root = resolve_skills_root(settings)
    items = list_stored_skills(root, _scope(scope, allow_all=True), user_id)
    SANDBOX_REQUESTS.labels(operation="skills.store.list", status="ok", project_id="").inc()
    logger.info(
        "skills.store.list",
        extra={"scope": scope, "user_id": user_id, "status": "ok", "count": len(items)},
    )
    return envelope({"skills": items})


@router.post("/skills/upload")
async def upload(
    request: Request,
    scope: str = Form(...),
    user_id: int = Form(0, alias="userId"),
    overwrite: bool = Form(False),
    file: UploadFile = File(...),
) -> dict:
    settings = request.app.state.settings
    root = resolve_skills_root(settings)
    item = save_skill_package(root, _scope(scope), user_id, file.file, overwrite)
    SANDBOX_REQUESTS.labels(operation="skills.store.upload", status="ok", project_id="").inc()
    logger.info(
        "skills.store.upload",
        extra={"scope": scope, "user_id": user_id, "status": "ok", "skill_name": item["name"]},
    )
    return envelope(item)


@router.delete("/skills/{scope}/{name}")
def delete(
    scope: str,
    name: str,
    request: Request,
    user_id: int = Query(0, alias="userId"),
) -> dict:
    settings = request.app.state.settings
    root = resolve_skills_root(settings)
    delete_skill(root, _scope(scope), user_id, name)
    SANDBOX_REQUESTS.labels(operation="skills.store.delete", status="ok", project_id="").inc()
    logger.info(
        "skills.store.delete",
        extra={"scope": scope, "user_id": user_id, "status": "ok", "skill_name": name},
    )
    return envelope({"deleted": True})


def _scope(value: str, allow_all: bool = False) -> str:
    allowed = {"public", "user"}
    if allow_all:
        allowed.add("all")
    if value not in allowed:
        raise WorkspaceError("invalid scope")
    return value

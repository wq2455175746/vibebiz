from fastapi import APIRouter, Request

from app.errors import NotFoundError
from app.observability import SANDBOX_REQUESTS, get_logger
from app.response import envelope
from app.services.skill_catalog import list_skills
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["skills"])
logger = get_logger()


@router.get("/{project_id}/skills")
def list_project_skills(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    settings = request.app.state.settings
    if not workspace.exists(project_id):
        SANDBOX_REQUESTS.labels(operation="skills.list", status="404", project_id=project_id).inc()
        raise NotFoundError("project not found")

    items = list_skills(settings, workspace.project_dir(project_id))
    SANDBOX_REQUESTS.labels(operation="skills.list", status="ok", project_id=project_id).inc()
    logger.info(
        "skills.list",
        extra={"project_id": project_id, "status": "ok", "count": len(items)},
    )
    return envelope({"skills": items})

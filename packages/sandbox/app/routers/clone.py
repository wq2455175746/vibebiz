from fastapi import APIRouter, Request
from pydantic import BaseModel, Field

from app.response import envelope
from app.services.clone import CloneService
from app.services.workspace import Workspace

router = APIRouter(prefix="/internal/projects", tags=["clone"])


class CloneAsset(BaseModel):
    role: str
    name: str = ""
    storageName: str
    contentType: str = ""
    kind: str = ""
    size: int = 0


class CloneSetup(BaseModel):
    name: str = Field(min_length=1)
    description: str = ""
    sourceUrl: str = ""
    assets: list[CloneAsset] = []


@router.put("/{project_id}/clone")
def sync_clone(project_id: str, body: CloneSetup, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    service: CloneService = request.app.state.clone
    workspace._existing_project(project_id)
    return envelope(service.sync(project_id, body.model_dump()))


@router.post("/{project_id}/clone/tests/run")
async def run_clone_tests(project_id: str, request: Request) -> dict:
    workspace: Workspace = request.app.state.workspace
    service: CloneService = request.app.state.clone
    workspace._existing_project(project_id)
    return envelope(await service.run_tests(project_id))

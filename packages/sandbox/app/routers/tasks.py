from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.errors import WorkspaceError
from app.response import envelope
from app.services.task_parser import parse_task

router = APIRouter(prefix="/internal/tasks", tags=["tasks"])


class ParseRequest(BaseModel):
    prompt: str = ""


@router.post("/parse")
async def parse(body: ParseRequest, request: Request) -> dict:
    prompt = body.prompt.strip()
    if not prompt:
        raise WorkspaceError("prompt is required")
    result = await parse_task(request.app.state.settings, prompt)
    return envelope(result)

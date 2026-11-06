from fastapi import Request

from app.errors import WorkspaceError


def epoch_from_request(request: Request) -> int:
    raw = request.headers.get("x-project-epoch", "").strip()
    if not raw:
        return 0
    try:
        epoch = int(raw)
    except ValueError as exc:
        raise WorkspaceError("invalid project epoch") from exc
    return epoch if epoch > 0 else 0

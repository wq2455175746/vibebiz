import json
import os
import tempfile
from datetime import UTC, datetime
from pathlib import Path

from app.errors import WorkspaceError


class StaleProjectEpoch(WorkspaceError):
    status = 409
    code = 40902


def _lease_path(project_dir: Path) -> Path:
    return project_dir / ".vibebiz" / "lease"


def read_epoch(project_dir: Path) -> int:
    path = _lease_path(project_dir)
    try:
        payload = json.loads(path.read_text(encoding="utf-8"))
        return int(payload.get("epoch", 0))
    except (OSError, ValueError, TypeError, json.JSONDecodeError):
        return 0


def check_epoch(project_dir: Path, epoch: int, sandbox_name: str) -> None:
    if epoch <= 0:
        return
    current = read_epoch(project_dir)
    if epoch < current:
        raise StaleProjectEpoch("stale project epoch")
    if epoch == current:
        return
    path = _lease_path(project_dir)
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {
        "epoch": epoch,
        "sandbox": sandbox_name,
        "updatedAt": datetime.now(UTC).isoformat(),
    }
    fd, tmp = tempfile.mkstemp(dir=path.parent, prefix=".lease.", suffix=".tmp")
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(payload, handle, ensure_ascii=False)
        os.replace(tmp, path)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)

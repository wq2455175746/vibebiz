import json
import re
import threading
from collections import deque
from collections.abc import Callable
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

PROJECT_ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
LOG_DIR = ".logs"


class ProjectLogStore:
    def __init__(
        self,
        root: Path,
        retention_days: int = 30,
        now: Callable[[], datetime] | None = None,
    ) -> None:
        self.root = root.resolve()
        self.retention_days = max(1, retention_days)
        self._now = now or (lambda: datetime.now(UTC))
        self._cleanup_day = ""
        self._lock = threading.Lock()

    def _project_dir(self, project_id: str) -> Path | None:
        if not PROJECT_ID_RE.fullmatch(project_id):
            return None
        project = (self.root / project_id).resolve()
        if not project.is_relative_to(self.root) or not project.is_dir():
            return None
        return project

    def _log_dir(self, project_id: str) -> Path | None:
        project = self._project_dir(project_id)
        return project / LOG_DIR if project else None

    def append(self, project_id: str, entry: dict[str, Any], timestamp: float) -> bool:
        log_dir = self._log_dir(project_id)
        if log_dir is None:
            return False
        moment = datetime.fromtimestamp(timestamp, UTC)
        day = moment.astimezone().strftime("%Y-%m-%d")
        path = log_dir / f"agent_{day}.jsonl"
        try:
            log_dir.mkdir(parents=True, exist_ok=True)
            with self._lock:
                with path.open("a", encoding="utf-8") as stream:
                    stream.write(json.dumps(entry, ensure_ascii=False) + "\n")
                self._cleanup(day)
        except OSError:
            return False
        return True

    def _cleanup(self, day: str) -> None:
        if day == self._cleanup_day:
            return
        self._cleanup_day = day
        cutoff = self._now().timestamp() - self.retention_days * 86400
        for path in self.root.glob(f"*/{LOG_DIR}/agent_*.jsonl"):
            try:
                if path.stat().st_mtime < cutoff:
                    path.unlink()
            except OSError:
                continue

    def read_latest(self, project_id: str, limit: int) -> list[dict[str, Any]]:
        log_dir = self._log_dir(project_id)
        if log_dir is None:
            return []
        entries: deque[dict[str, Any]] = deque(maxlen=max(0, limit))
        for path in sorted(log_dir.glob("agent_*.jsonl")):
            try:
                with path.open(encoding="utf-8") as stream:
                    for line in stream:
                        try:
                            item = json.loads(line)
                        except json.JSONDecodeError:
                            continue
                        if isinstance(item, dict):
                            entries.append(item)
            except OSError:
                continue
        return list(entries)

import asyncio
import contextlib
import hashlib
import json
import os
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from app.errors import TooLargeError, WorkspaceError
from app.services.workspace import Workspace

HISTORY_DIR = ".chathistory"
HISTORY_FILE = "session.jsonl"


@dataclass(frozen=True)
class HistoryTail:
    content: str
    truncated: bool
    version: str


class ChatHistory:
    def __init__(self, workspace: Workspace, max_bytes: int = 10 * 1024 * 1024) -> None:
        self.workspace = workspace
        self.max_bytes = max_bytes
        self._locks: dict[str, asyncio.Lock] = {}

    def path(self, project_id: str) -> Path:
        project_dir = self.workspace.project_dir(project_id)
        return project_dir / HISTORY_DIR / HISTORY_FILE

    async def append_user(
        self,
        project_id: str,
        message: str,
        turn_id: str,
        references: list[dict[str, Any]] | None = None,
    ) -> str:
        async with self._lock(project_id):
            parent_id = self._last_message_id(project_id)
            entry = self._message_entry(
                project_id=project_id,
                parent_id=parent_id,
                turn_id=turn_id,
                role="user",
                content=message,
                status="completed",
                tools=[],
                extra={"references": references} if references else None,
            )
            self._append(project_id, entry)
            return entry["id"]

    async def append_assistant(
        self,
        project_id: str,
        turn_id: str,
        content: str,
        tools: list[dict[str, Any]],
        status: str,
        model: str = "",
        duration_ms: int | None = None,
        num_turns: int | None = None,
        error_message: str = "",
    ) -> str:
        async with self._lock(project_id):
            parent_id = self._last_message_id(project_id)
            message: dict[str, Any] = {
                "role": "assistant",
                "content": content,
                "status": status,
                "tools": tools,
            }
            if model:
                message["model"] = model
            if duration_ms is not None:
                message["durationMs"] = duration_ms
            if num_turns is not None:
                message["numTurns"] = num_turns
            if error_message:
                message["errorMessage"] = error_message
            entry = self._message_entry(
                project_id=project_id,
                parent_id=parent_id,
                turn_id=turn_id,
                role="assistant",
                content=content,
                status=status,
                tools=tools,
                extra=message,
            )
            self._append(project_id, entry)
            return entry["id"]

    def read_tail(self, project_id: str, limit: int) -> HistoryTail:
        path = self.path(project_id)
        if not path.is_file():
            return HistoryTail(content="", truncated=False, version="")
        size = path.stat().st_size
        if size > self.max_bytes:
            raise TooLargeError("chat history is too large")
        raw = path.read_bytes()
        lines = [
            line for line in raw.decode("utf-8", errors="replace").splitlines() if line.strip()
        ]
        truncated = len(lines) > limit
        selected = lines[-limit:] if limit > 0 else []
        content = "\n".join(selected)
        if content:
            content += "\n"
        return HistoryTail(
            content=content,
            truncated=truncated,
            version=hashlib.sha256(raw).hexdigest()[:12],
        )

    def _lock(self, project_id: str) -> asyncio.Lock:
        return self._locks.setdefault(project_id, asyncio.Lock())

    def _last_message_id(self, project_id: str) -> str | None:
        path = self.path(project_id)
        if not path.is_file():
            return None
        try:
            lines = path.read_text(encoding="utf-8").splitlines()
        except OSError as exc:
            raise WorkspaceError("read chat history failed") from exc
        for line in reversed(lines):
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            if entry.get("type") == "message" and isinstance(entry.get("id"), str):
                return entry["id"]
        return None

    def _append(self, project_id: str, entry: dict[str, Any]) -> None:
        path = self.path(project_id)
        self._ensure_gitignored(path.parent.parent)
        path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        with contextlib.suppress(OSError):
            os.chmod(path.parent, 0o700)

        fd = os.open(path, os.O_APPEND | os.O_CREAT | os.O_WRONLY, 0o600)
        try:
            if os.fstat(fd).st_size == 0:
                header = {
                    "type": "session",
                    "version": 1,
                    "projectId": project_id,
                    "createdAt": _timestamp(),
                }
                os.write(fd, _json_line(header))
            os.write(fd, _json_line(entry))
            os.fsync(fd)
        finally:
            os.close(fd)

    def _ensure_gitignored(self, project_dir: Path) -> None:
        ignore = project_dir / ".gitignore"
        existing = ignore.read_text(encoding="utf-8") if ignore.exists() else ""
        if ".chathistory/" in existing.splitlines():
            return
        suffix = "" if not existing or existing.endswith("\n") else "\n"
        ignore.write_text(existing + suffix + ".chathistory/\n", encoding="utf-8")

    def _message_entry(
        self,
        project_id: str,
        parent_id: str | None,
        turn_id: str,
        role: str,
        content: str,
        status: str,
        tools: list[dict[str, Any]],
        extra: dict[str, Any] | None = None,
    ) -> dict[str, Any]:
        del project_id
        message: dict[str, Any] = dict(extra or {})
        message.update(
            {
                "role": role,
                "content": content,
                "status": status,
                "tools": tools,
            }
        )
        return {
            "type": "message",
            "id": uuid.uuid4().hex[:12],
            "parentId": parent_id,
            "turnId": turn_id,
            "timestamp": _timestamp(),
            "message": message,
        }


def _json_line(value: dict[str, Any]) -> bytes:
    return (json.dumps(value, ensure_ascii=False, separators=(",", ":")) + "\n").encode("utf-8")


def _timestamp() -> str:
    return datetime.now(UTC).isoformat(timespec="milliseconds").replace("+00:00", "Z")

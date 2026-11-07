import asyncio
import hashlib
import json
import os
import uuid
from dataclasses import dataclass
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from app.errors import TooLargeError, WorkspaceError
from app.services.safe_paths import (
    UnsafePathError,
    open_append,
    read_bytes,
    read_text,
    resolve_within,
    write_text,
)
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
        try:
            history_dir = resolve_within(project_dir / HISTORY_DIR, project_dir)
        except UnsafePathError as exc:
            raise WorkspaceError("invalid chat history path") from exc
        return history_dir / HISTORY_FILE

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
        project_dir = self.workspace.project_dir(project_id)
        path = self.path(project_id)
        try:
            raw = read_bytes(path, project_dir)
        except (OSError, UnsafePathError) as exc:
            raise WorkspaceError("invalid chat history path") from exc
        if raw is None:
            return HistoryTail(content="", truncated=False, version="")
        if len(raw) > self.max_bytes:
            raise TooLargeError("chat history is too large")
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
        project_dir = self.workspace.project_dir(project_id)
        path = self.path(project_id)
        try:
            content = read_text(path, project_dir)
        except (OSError, UnsafePathError) as exc:
            raise WorkspaceError("read chat history failed") from exc
        if content is None:
            return None
        lines = content.splitlines()
        for line in reversed(lines):
            try:
                entry = json.loads(line)
            except json.JSONDecodeError:
                continue
            if entry.get("type") == "message" and isinstance(entry.get("id"), str):
                return entry["id"]
        return None

    def _append(self, project_id: str, entry: dict[str, Any]) -> None:
        project_dir = self.workspace.project_dir(project_id)
        path = self.path(project_id)
        self._ensure_gitignored(project_dir)
        try:
            fd = open_append(path, project_dir)
        except (OSError, UnsafePathError) as exc:
            raise WorkspaceError("invalid chat history path") from exc
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
        try:
            existing = read_text(ignore, project_dir) or ""
        except (OSError, UnsafePathError):
            return
        if ".chathistory/" in existing.splitlines():
            return
        suffix = "" if not existing or existing.endswith("\n") else "\n"
        try:
            write_text(ignore, existing + suffix + ".chathistory/\n", project_dir, mode=0o644)
        except (OSError, UnsafePathError):
            return

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

import asyncio
import json
import os
import re
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

from app.errors import ConflictError, TooLargeError, WorkspaceError
from app.services.workspace import Workspace

RESOURCE_DIR = ".resources"
MANIFEST_NAME = "manifest.json"
MAX_RESOURCE_BYTES = 20 * 1024 * 1024
MAX_PROJECT_RESOURCE_BYTES = 200 * 1024 * 1024
MAX_CLONE_FILE_BYTES = 100 * 1024 * 1024
MAX_CLONE_PROJECT_BYTES = 500 * 1024 * 1024
RESOURCE_ID_RE = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
STORAGE_NAME_RE = re.compile(r"^[A-Za-z0-9_-]{1,128}(\.[A-Za-z0-9]{1,16})?$")


class ResourceStore:
    def __init__(self, workspace: Workspace) -> None:
        self.workspace = workspace
        self._locks: dict[str, asyncio.Lock] = {}

    def _lock(self, project_id: str) -> asyncio.Lock:
        return self._locks.setdefault(project_id, asyncio.Lock())

    def _root(self, project_id: str) -> Path:
        return self.workspace.project_dir(project_id) / RESOURCE_DIR

    def _manifest_path(self, project_id: str) -> Path:
        return self._root(project_id) / MANIFEST_NAME

    def _read(self, project_id: str) -> dict[str, Any]:
        path = self._manifest_path(project_id)
        if not path.exists():
            return {"version": 1, "files": []}
        try:
            payload = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            return {"version": 1, "files": []}
        if not isinstance(payload, dict) or not isinstance(payload.get("files"), list):
            return {"version": 1, "files": []}
        return payload

    def _write_manifest(self, project_id: str, payload: dict[str, Any]) -> None:
        root = self._root(project_id)
        root.mkdir(parents=True, exist_ok=True, mode=0o700)
        fd, tmp_name = tempfile.mkstemp(dir=root, prefix=".manifest.", suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as tmp:
                json.dump(payload, tmp, ensure_ascii=False, separators=(",", ":"))
                tmp.flush()
                os.fsync(tmp.fileno())
            os.replace(tmp_name, self._manifest_path(project_id))
        finally:
            if os.path.exists(tmp_name):
                os.unlink(tmp_name)

    def list(self, project_id: str) -> dict[str, Any]:
        files = self._read(project_id)["files"]
        return {"resources": files, "totalBytes": sum(int(item.get("size", 0)) for item in files)}

    async def write(self, project_id: str, meta: dict[str, Any], body: bytes) -> dict[str, Any]:
        resource_id = str(meta.get("id", ""))
        if not RESOURCE_ID_RE.fullmatch(resource_id):
            raise WorkspaceError("invalid resource id")
        storage_name = str(meta.get("storageName", ""))
        if not STORAGE_NAME_RE.fullmatch(storage_name):
            raise WorkspaceError("invalid storage name")
        display_name = str(meta.get("displayName", "")).strip()
        original_name = str(meta.get("name", display_name)).strip()
        if not display_name or len(display_name) > 255 or not original_name:
            raise WorkspaceError("invalid resource name")
        purpose = str(meta.get("purpose", "")).strip()
        max_file_bytes, max_project_bytes = purpose_limits(purpose)
        size = len(body)
        if size <= 0 or size > max_file_bytes:
            raise TooLargeError("resource file is too large")

        async with self._lock(project_id):
            payload = self._read(project_id)
            files = payload["files"]
            existing = next((item for item in files if item.get("id") == resource_id), None)
            for item in files:
                if item.get("id") != resource_id and item.get("displayName") == display_name:
                    raise ConflictError("resource name already exists")
            total = sum(int(item.get("size", 0)) for item in files)
            previous = int(existing.get("size", 0)) if existing else 0
            if total - previous + size > max_project_bytes:
                raise TooLargeError("project resource quota exceeded")

            root = self._root(project_id)
            root.mkdir(parents=True, exist_ok=True, mode=0o700)
            target = root / storage_name
            fd, tmp_name = tempfile.mkstemp(dir=root, prefix=f".{resource_id}.", suffix=".tmp")
            try:
                with os.fdopen(fd, "wb") as tmp:
                    tmp.write(body)
                    tmp.flush()
                    os.fsync(tmp.fileno())
                os.replace(tmp_name, target)
            finally:
                if os.path.exists(tmp_name):
                    os.unlink(tmp_name)

            entry = {
                "id": resource_id,
                "name": original_name,
                "displayName": display_name,
                "storageName": storage_name,
                "path": f"{RESOURCE_DIR}/{storage_name}",
                "s3Key": str(meta.get("s3Key", "")),
                "size": size,
                "contentType": str(meta.get("contentType", "")),
                "kind": str(meta.get("kind", "")),
                "purpose": purpose,
                "createdAt": (existing or {}).get("createdAt")
                or datetime.now(UTC).isoformat().replace("+00:00", "Z"),
            }
            if existing is None:
                files.append(entry)
            else:
                files[files.index(existing)] = entry
            payload["version"] = 1
            self._write_manifest(project_id, payload)
            return entry

    async def delete(self, project_id: str, resource_id: str) -> bool:
        if not RESOURCE_ID_RE.fullmatch(resource_id):
            raise WorkspaceError("invalid resource id")
        async with self._lock(project_id):
            payload = self._read(project_id)
            item = next(
                (entry for entry in payload["files"] if entry.get("id") == resource_id), None
            )
            if item is None:
                return False
            target = self._root(project_id) / str(item.get("storageName", ""))
            if target.is_file():
                target.unlink()
            payload["files"] = [
                entry for entry in payload["files"] if entry.get("id") != resource_id
            ]
            self._write_manifest(project_id, payload)
            return True


def purpose_limits(purpose: str) -> tuple[int, int]:
    if purpose == "clone_image":
        return 10 * 1024 * 1024, MAX_CLONE_PROJECT_BYTES
    if purpose in {"clone_unit_test", "clone_docs"}:
        return MAX_CLONE_FILE_BYTES, MAX_CLONE_PROJECT_BYTES
    return MAX_RESOURCE_BYTES, MAX_PROJECT_RESOURCE_BYTES

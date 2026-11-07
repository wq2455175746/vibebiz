import asyncio
import json
import os
import re
import shutil
import sys
import tempfile
import time
import zipfile
from contextlib import suppress
from datetime import UTC, datetime
from pathlib import Path
from urllib.parse import urlparse

from app.errors import NotFoundError, WorkspaceError
from app.services.safe_paths import UnsafePathError, resolve_within
from app.services.workspace import Workspace

CLONE_DIR = ".clone"
SOURCE_NAME = "source.json"
TESTS_DIR = "tests"
REPORT_PATH = Path("docs") / "clone" / "test-report.md"

MAX_SOURCE_URL = 1024
MAX_NAME = 200
MAX_DESCRIPTION = 2000
MAX_ZIP_ENTRIES = 5000
MAX_UNCOMPRESSED_BYTES = 500 * 1024 * 1024
MAX_OUTPUT_BYTES = 64 * 1024
DEFAULT_TIMEOUT_SECONDS = 120
STORAGE_NAME_RE = re.compile(r"^[A-Za-z0-9_-]{1,128}(\.[A-Za-z0-9]{1,16})?$")
ASSET_ROLES = {"cover", "gallery", "unit_test", "docs"}
SAFE_ENV_KEYS = {
    "PATH",
    "HOME",
    "LANG",
    "LC_ALL",
    "LC_CTYPE",
    "TMPDIR",
    "TMP",
    "TEMP",
    "SYSTEMROOT",
    "USER",
}


class CloneService:
    def __init__(
        self,
        workspace: Workspace,
        timeout_seconds: int = DEFAULT_TIMEOUT_SECONDS,
        max_output_bytes: int = MAX_OUTPUT_BYTES,
        enabled: bool = True,
    ) -> None:
        self.workspace = workspace
        self.timeout_seconds = max(1, timeout_seconds)
        self.max_output_bytes = max(1024, max_output_bytes)
        self.enabled = enabled

    def _project(self, project_id: str) -> Path:
        return self.workspace.project_dir(project_id)

    def _existing_project(self, project_id: str) -> Path:
        project = self._project(project_id)
        if not project.is_dir():
            raise NotFoundError("project not found")
        return project

    def _root(self, project_id: str) -> Path:
        project = self._existing_project(project_id)
        try:
            return resolve_within(project / CLONE_DIR, project)
        except UnsafePathError as exc:
            raise WorkspaceError("invalid clone path") from exc

    def _write_atomic(self, path: Path, content: str, root: Path) -> None:
        try:
            path = resolve_within(path, root)
        except UnsafePathError as exc:
            raise WorkspaceError("invalid clone path") from exc
        path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
        path = resolve_within(path, root)
        fd, tmp_name = tempfile.mkstemp(dir=path.parent, prefix=f".{path.name}.", suffix=".tmp")
        try:
            with os.fdopen(fd, "w", encoding="utf-8") as handle:
                handle.write(content)
                handle.flush()
                os.fsync(handle.fileno())
            os.replace(tmp_name, path)
        finally:
            if os.path.exists(tmp_name):
                os.unlink(tmp_name)

    def _validate(self, payload: dict) -> dict:
        name = str(payload.get("name", "")).strip()
        description = str(payload.get("description", "")).strip()
        source_url = str(payload.get("sourceUrl", "")).strip()
        if not name or len(name) > MAX_NAME:
            raise WorkspaceError("invalid clone name")
        if not description or len(description) > MAX_DESCRIPTION:
            raise WorkspaceError("invalid clone description")
        if len(source_url) > MAX_SOURCE_URL:
            raise WorkspaceError("invalid clone source url")
        if source_url:
            parsed = urlparse(source_url)
            if parsed.scheme not in {"http", "https"} or not parsed.netloc:
                raise WorkspaceError("invalid clone source url")
        assets = payload.get("assets") or []
        if not isinstance(assets, list):
            raise WorkspaceError("invalid clone assets")
        normalized = []
        for item in assets:
            if not isinstance(item, dict):
                raise WorkspaceError("invalid clone asset")
            role = str(item.get("role", "")).strip()
            if role not in ASSET_ROLES:
                raise WorkspaceError("invalid clone asset role")
            storage_name = str(item.get("storageName", "")).strip()
            if not STORAGE_NAME_RE.fullmatch(storage_name):
                raise WorkspaceError("invalid clone asset storage name")
            normalized.append(
                {
                    "role": role,
                    "name": str(item.get("name", "")).strip(),
                    "storageName": storage_name,
                    "contentType": str(item.get("contentType", "")).strip(),
                    "kind": str(item.get("kind", "")).strip(),
                    "size": int(item.get("size", 0) or 0),
                }
            )
        return {
            "name": name,
            "description": description,
            "sourceUrl": source_url,
            "assets": normalized,
            "updatedAt": datetime.now(UTC).isoformat().replace("+00:00", "Z"),
        }

    def sync(self, project_id: str, payload: dict) -> dict:
        project = self._existing_project(project_id)
        normalized = self._validate(payload)
        root = self._root(project_id)
        root.mkdir(parents=True, exist_ok=True, mode=0o700)
        self._write_atomic(
            root / SOURCE_NAME,
            json.dumps(normalized, ensure_ascii=False, indent=2),
            project,
        )
        unit_test = next(
            (asset for asset in normalized["assets"] if asset["role"] == "unit_test"), None
        )
        if unit_test is None:
            target = root / TESTS_DIR
            if target.is_symlink():
                target.unlink()
            else:
                shutil.rmtree(target, ignore_errors=True)
        else:
            self._extract_tests(project, root, unit_test["storageName"])
        return normalized

    def _extract_tests(self, project: Path, root: Path, storage_name: str) -> None:
        try:
            archive_path = resolve_within(project / ".resources" / storage_name, project)
            target = resolve_within(root / TESTS_DIR, project)
        except UnsafePathError as exc:
            raise WorkspaceError("invalid clone test path") from exc
        if not archive_path.is_file():
            raise NotFoundError("unit test archive not found")
        temp_dir = Path(tempfile.mkdtemp(dir=root, prefix=".tests."))
        try:
            self._safe_extract(archive_path, temp_dir)
            if target.is_symlink():
                target.unlink()
            else:
                shutil.rmtree(target, ignore_errors=True)
            os.replace(temp_dir, target)
        finally:
            if temp_dir.exists():
                shutil.rmtree(temp_dir, ignore_errors=True)

    def _safe_extract(self, archive_path: Path, target: Path) -> None:
        try:
            with zipfile.ZipFile(archive_path) as archive:
                entries = archive.infolist()
                if len(entries) > MAX_ZIP_ENTRIES:
                    raise WorkspaceError("unit test archive has too many entries")
                total = 0
                for info in entries:
                    name = info.filename.replace("\\", "/")
                    if not name or name.startswith("/") or name.startswith("../") or "/../" in name:
                        raise WorkspaceError("invalid unit test archive path")
                    mode = info.external_attr >> 16
                    if mode & 0o170000 == 0o120000:
                        raise WorkspaceError("unit test archive contains symlink")
                    total += info.file_size
                    if total > MAX_UNCOMPRESSED_BYTES:
                        raise WorkspaceError("unit test archive is too large")
                    destination = (target / name).resolve()
                    if not destination.is_relative_to(target.resolve()):
                        raise WorkspaceError("invalid unit test archive path")
                    if info.is_dir():
                        destination.mkdir(parents=True, exist_ok=True)
                        continue
                    destination.parent.mkdir(parents=True, exist_ok=True)
                    with archive.open(info) as source, destination.open("wb") as sink:
                        shutil.copyfileobj(source, sink)
        except zipfile.BadZipFile as exc:
            raise WorkspaceError("invalid unit test archive") from exc

    async def run_tests(self, project_id: str) -> dict:
        project = self._existing_project(project_id)
        try:
            tests_dir = resolve_within(project / CLONE_DIR / TESTS_DIR, project)
        except UnsafePathError:
            return self._finish(project, "skipped", [], -1, "测试目录不安全", 0)
        if tests_dir.is_dir() and any(path.is_symlink() for path in tests_dir.rglob("*")):
            return self._finish(project, "skipped", [], -1, "测试目录包含符号链接", 0)
        command, cwd = self._detect_command(project, tests_dir)
        if not self.enabled:
            return self._finish(project, "skipped", [], -1, "测试执行已关闭", 0)
        if command is None:
            return self._finish(project, "skipped", [], -1, "未找到可执行的测试", 0)

        started = time.monotonic()
        env = {key: value for key, value in os.environ.items() if key in SAFE_ENV_KEYS}
        env["PYTHONPATH"] = str(project)
        env["PYTHONDONTWRITEBYTECODE"] = "1"
        process = await asyncio.create_subprocess_exec(
            *command,
            cwd=str(cwd),
            env=env,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.STDOUT,
        )
        timed_out = False
        try:
            output_bytes, _ = await asyncio.wait_for(
                process.communicate(), timeout=self.timeout_seconds
            )
        except asyncio.CancelledError:
            with suppress(ProcessLookupError):
                process.kill()
            with suppress(ProcessLookupError):
                await process.wait()
            raise
        except TimeoutError:
            timed_out = True
            process.kill()
            output_bytes, _ = await process.communicate()
        duration_ms = int((time.monotonic() - started) * 1000)
        output = output_bytes[-self.max_output_bytes :].decode("utf-8", errors="replace")
        if timed_out:
            return self._finish(
                project, "failed", command, -1, f"测试超时\n\n{output}", duration_ms
            )
        status = "passed" if process.returncode == 0 else "failed"
        return self._finish(project, status, command, process.returncode or 0, output, duration_ms)

    def _detect_command(self, project: Path, tests_dir: Path) -> tuple[list[str] | None, Path]:
        if tests_dir.is_dir() and any(
            path.is_file() and not path.is_symlink() for path in tests_dir.rglob("*.py")
        ):
            return [
                sys.executable,
                "-m",
                "pytest",
                "-q",
                "-rA",
                str(tests_dir.relative_to(project)),
            ], project
        package_json = project / "package.json"
        if package_json.is_file() and not package_json.is_symlink():
            return ["npm", "test", "--", "--run"], project
        tests_package = tests_dir / "package.json"
        if tests_package.is_file() and not tests_package.is_symlink():
            return ["npm", "test", "--", "--run"], tests_dir
        return None, project

    def _finish(
        self,
        project: Path,
        status: str,
        command: list[str],
        exit_code: int,
        output: str,
        duration_ms: int,
    ) -> dict:
        labels = {
            "passed": "通过",
            "failed": "失败",
            "skipped": "跳过",
        }
        label = labels.get(status, status)
        report = (
            "# 测试报告\n\n"
            f"- 执行时间：{datetime.now(UTC).isoformat().replace('+00:00', 'Z')}\n"
            f"- 命令：{' '.join(command) if command else '—'}\n"
            f"- 状态：{label}\n"
            f"- 退出码：{exit_code}\n"
            f"- 耗时：{duration_ms}ms\n\n"
            "## 输出\n\n"
            "```text\n"
            f"{output.strip() or '（无输出）'}\n"
            "```\n"
        )
        report_path = project / REPORT_PATH
        self._write_atomic(report_path, report, project)
        return {
            "status": status,
            "command": command,
            "exitCode": exit_code,
            "output": output,
            "durationMs": duration_ms,
            "reportPath": REPORT_PATH.as_posix(),
        }

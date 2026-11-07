import os
import shutil
import subprocess
from pathlib import Path

from app.services.safe_paths import UnsafePathError, read_text, resolve_within, write_text

GIT_CONFIG_FLAGS = [
    "-c",
    "core.hooksPath=/dev/null",
    "-c",
    "core.fsmonitor=false",
    "-c",
    "core.attributesFile=/dev/null",
]
GIT_ENV_KEYS = ("PATH", "LANG", "LC_ALL", "LC_CTYPE")


class Versioning:
    def __init__(self) -> None:
        self.git = shutil.which("git")

    def init(self, project_dir: Path) -> None:
        self._ensure_gitignore(project_dir)
        if not self.git:
            return
        git_dir = project_dir / ".git"
        if git_dir.is_symlink() or git_dir.exists():
            return
        self._run(project_dir, "init", "-q")
        self._run(project_dir, "config", "user.email", "agent@vibebiz.local")
        self._run(project_dir, "config", "user.name", "Vibebiz Agent")

    def _ensure_gitignore(self, project_dir: Path) -> None:
        ignore = project_dir / ".gitignore"
        try:
            existing = read_text(ignore, project_dir) or ""
        except (OSError, UnsafePathError):
            return
        lines = existing.splitlines()
        if not existing:
            updated = ".agent/\nnode_modules/\n__pycache__/\n.chathistory/\n.resources/\n.logs/\n"
        else:
            updated = existing
            for item in (".chathistory/", ".resources/", ".logs/"):
                if item not in lines:
                    suffix = "" if updated.endswith("\n") else "\n"
                    updated += suffix + item + "\n"
        if updated == existing:
            return
        try:
            write_text(ignore, updated, project_dir, mode=0o644)
        except (OSError, UnsafePathError):
            return

    def commit(self, project_dir: Path, message: str) -> str | None:
        if not self.git or self._git_dir(project_dir) is None:
            return None
        self._ensure_gitignore(project_dir)
        self._run(project_dir, "add", "-A")
        self._run(project_dir, "commit", "-q", "--no-verify", "-m", message[:120], "--allow-empty")
        result = self._run(project_dir, "rev-parse", "--short", "HEAD")
        if result is None:
            return None
        return result.stdout.strip() or None

    def _git_dir(self, project_dir: Path) -> Path | None:
        try:
            git_dir = resolve_within(project_dir / ".git", project_dir)
        except UnsafePathError:
            return None
        return git_dir if git_dir.is_dir() else None

    def _run(self, cwd: Path, *args: str) -> subprocess.CompletedProcess | None:
        if self.git is None:
            return None
        env = {key: os.environ[key] for key in GIT_ENV_KEYS if key in os.environ}
        env.update(
            {
                "GIT_CONFIG_NOSYSTEM": "1",
                "GIT_CONFIG_GLOBAL": "/dev/null",
                "GIT_ATTR_NOSYSTEM": "1",
                "GIT_TERMINAL_PROMPT": "0",
                "GIT_OPTIONAL_LOCKS": "0",
            }
        )
        try:
            return subprocess.run(
                [self.git, *GIT_CONFIG_FLAGS, *args],
                cwd=cwd,
                env=env,
                capture_output=True,
                text=True,
                timeout=15,
                check=False,
            )
        except (OSError, subprocess.SubprocessError):
            return None

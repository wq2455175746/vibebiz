import shutil
import subprocess
from pathlib import Path


class Versioning:
    def __init__(self) -> None:
        self.git = shutil.which("git")

    def init(self, project_dir: Path) -> None:
        self._ensure_gitignore(project_dir)
        if not self.git:
            return
        if (project_dir / ".git").exists():
            return
        self._run(project_dir, "init", "-q")
        self._run(project_dir, "config", "user.email", "agent@vibebiz.local")
        self._run(project_dir, "config", "user.name", "Vibebiz Agent")

    def _ensure_gitignore(self, project_dir: Path) -> None:
        ignore = project_dir / ".gitignore"
        existing = ignore.read_text(encoding="utf-8") if ignore.exists() else ""
        lines = existing.splitlines()
        if not existing:
            ignore.write_text(
                ".agent/\nnode_modules/\n__pycache__/\n.chathistory/\n.resources/\n.logs/\n",
                encoding="utf-8",
            )
            return
        for item in (".chathistory/", ".resources/", ".logs/"):
            if item not in lines:
                suffix = "" if existing.endswith("\n") else "\n"
                existing += suffix + item + "\n"
                ignore.write_text(existing, encoding="utf-8")

    def commit(self, project_dir: Path, message: str) -> str | None:
        if not self.git or not (project_dir / ".git").exists():
            return None
        self._ensure_gitignore(project_dir)
        self._run(project_dir, "add", "-A")
        self._run(project_dir, "commit", "-q", "-m", message[:120], "--allow-empty")
        result = self._run(project_dir, "rev-parse", "--short", "HEAD")
        if result is None:
            return None
        return result.stdout.strip() or None

    def _run(self, cwd: Path, *args: str) -> subprocess.CompletedProcess | None:
        try:
            return subprocess.run(
                [self.git, *args],
                cwd=cwd,
                capture_output=True,
                text=True,
                timeout=15,
                check=False,
            )
        except (OSError, subprocess.SubprocessError):
            return None

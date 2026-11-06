from __future__ import annotations

import fcntl
import re
import shutil
import uuid
import zipfile
from collections.abc import Iterator
from contextlib import contextmanager
from pathlib import Path, PurePosixPath
from typing import BinaryIO

from app.errors import ConflictError, NotFoundError, TooLargeError, WorkspaceError
from app.services.skill_catalog import iter_skill_files, read_skill

SKILL_NAME_RE = re.compile(r"^[a-z0-9][a-z0-9_-]{0,63}$")
MAX_ZIP_BYTES = 20 * 1024 * 1024
MAX_EXTRACTED_BYTES = 100 * 1024 * 1024
MAX_FILES = 500
MAX_DEPTH = 8
MAX_SKILL_MD_BYTES = 64 * 1024
AGENT_HOME_ROOT = Path("/tmp/vibebiz-sandbox-home")


def resolve_skills_root(settings) -> Path:
    if settings.skills_root is not None:
        return Path(settings.skills_root).resolve()
    return (Path(settings.workspaces_dir).parent / "skills").resolve()


def ensure_layout(root: Path) -> None:
    root = Path(root).resolve()
    for name in ("public", "users", ".tmp", ".trash", ".locks"):
        (root / name).mkdir(parents=True, exist_ok=True)


def _user_config_dir(root: Path, user_id: int) -> Path:
    if user_id <= 0:
        raise WorkspaceError("userId is required")
    base = (Path(root).resolve() / "users").resolve()
    target = (base / str(user_id) / ".claude").resolve()
    if not target.is_relative_to(base):
        raise WorkspaceError("invalid userId")
    return target


def _target_dir(root: Path, scope: str, user_id: int) -> Path:
    if scope == "public":
        return (Path(root).resolve() / "public").resolve()
    if scope == "user":
        return (_user_config_dir(root, user_id) / "skills").resolve()
    raise WorkspaceError("invalid scope")


def list_stored_skills(root: Path, scope: str, user_id: int = 0) -> list[dict[str, str]]:
    if scope == "all":
        items: list[dict[str, str]] = []
        if user_id > 0:
            items.extend(list_stored_skills(root, "user", user_id))
        items.extend(list_stored_skills(root, "public", 0))
        return _dedupe_and_sort(items)
    if scope not in {"public", "user"}:
        raise WorkspaceError("invalid scope")
    directory = _target_dir(root, scope, user_id)
    if not directory.is_dir():
        return []
    items: list[dict[str, str]] = []
    for skill_file in iter_skill_files(directory):
        item = read_skill(skill_file, scope)
        if item is not None:
            items.append(item)
    return _dedupe_and_sort(items)


def _dedupe_and_sort(items: list[dict[str, str]]) -> list[dict[str, str]]:
    found: dict[str, dict[str, str]] = {}
    for item in items:
        found.setdefault(item["name"], item)
    return sorted(found.values(), key=lambda item: item["name"])


def save_skill_package(
    root: Path,
    scope: str,
    user_id: int,
    source: BinaryIO,
    overwrite: bool,
) -> dict[str, str]:
    target_root = _target_dir(root, scope, user_id)
    ensure_layout(Path(root))
    temp_zip = Path(root).resolve() / ".tmp" / f"{uuid.uuid4().hex}.zip"
    try:
        _copy_limited(source, temp_zip)
        with zipfile.ZipFile(temp_zip) as archive:
            name = _validate_archive(archive)
            with _skill_lock(Path(root), scope, user_id, name):
                return _extract_skill(archive, Path(root), target_root, name, overwrite)
    except zipfile.BadZipFile as exc:
        raise WorkspaceError("invalid zip archive") from exc
    finally:
        temp_zip.unlink(missing_ok=True)


def _copy_limited(source: BinaryIO, target: Path) -> None:
    total = 0
    with target.open("wb") as out:
        while True:
            chunk = source.read(1024 * 1024)
            if not chunk:
                break
            total += len(chunk)
            if total > MAX_ZIP_BYTES:
                raise TooLargeError("zip is too large")
            out.write(chunk)


def _validate_archive(archive: zipfile.ZipFile) -> str:
    infos = archive.infolist()
    if not infos or len(infos) > MAX_FILES:
        raise WorkspaceError("invalid skill package")

    names: set[str] = set()
    top_levels: set[str] = set()
    total = 0
    for info in infos:
        raw = info.filename
        if not raw or "\\" in raw or "\x00" in raw:
            raise WorkspaceError("invalid archive path")
        if info.flag_bits & 0x1:
            raise WorkspaceError("encrypted zip is not supported")
        path = PurePosixPath(raw)
        if path.is_absolute() or any(part in {"", ".", ".."} for part in path.parts):
            raise WorkspaceError("invalid archive path")
        if len(path.parts) > MAX_DEPTH + 1:
            raise WorkspaceError("archive path is too deep")
        mode = (info.external_attr >> 16) & 0o170000
        if mode not in {0, 0o040000, 0o100000}:
            raise WorkspaceError("archive contains unsupported file type")
        key = path.as_posix().rstrip("/")
        if key in names:
            raise WorkspaceError("archive contains duplicate entries")
        names.add(key)
        if len(path.parts) == 1 and not info.is_dir():
            raise WorkspaceError("skill must be inside a top-level directory")
        if len(path.parts) > 1:
            top_levels.add(path.parts[0])
        total += info.file_size
        if total > MAX_EXTRACTED_BYTES:
            raise TooLargeError("skill package is too large")

    if len(top_levels) != 1:
        raise WorkspaceError("skill package must contain exactly one top-level directory")
    name = next(iter(top_levels))
    if not SKILL_NAME_RE.fullmatch(name):
        raise WorkspaceError("invalid skill name")
    if f"{name}/SKILL.md" not in names:
        raise WorkspaceError("SKILL.md is required")
    skill_md = archive.getinfo(f"{name}/SKILL.md")
    if skill_md.file_size > MAX_SKILL_MD_BYTES:
        raise WorkspaceError("SKILL.md is too large")
    return name


def _extract_skill(
    archive: zipfile.ZipFile,
    root: Path,
    target_root: Path,
    name: str,
    overwrite: bool,
) -> dict[str, str]:
    temp_dir = root / ".tmp" / uuid.uuid4().hex
    staging = temp_dir / name
    temp_dir.mkdir(parents=True)
    try:
        for info in archive.infolist():
            path = PurePosixPath(info.filename)
            destination = (temp_dir / Path(*path.parts)).resolve()
            if not destination.is_relative_to(temp_dir.resolve()):
                raise WorkspaceError("invalid archive path")
            if info.is_dir():
                destination.mkdir(parents=True, exist_ok=True)
                continue
            destination.parent.mkdir(parents=True, exist_ok=True)
            with archive.open(info) as src, destination.open("wb") as dst:
                shutil.copyfileobj(src, dst, length=1024 * 1024)
        if not (staging / "SKILL.md").is_file():
            raise WorkspaceError("SKILL.md is required")

        target_root.mkdir(parents=True, exist_ok=True)
        target = target_root / name
        backup = root / ".trash" / uuid.uuid4().hex
        if target.is_symlink():
            resolved = target.resolve()
            if resolved.is_relative_to((root / "public").resolve()):
                target.unlink()
        if target.exists():
            if not overwrite:
                raise ConflictError("skill already exists")
            target.rename(backup)
        try:
            staging.rename(target)
        except Exception:
            if backup.exists():
                backup.rename(target)
            raise
        shutil.rmtree(backup, ignore_errors=True)
        item = read_skill(target / "SKILL.md", "public" if target_root.name == "public" else "user")
        if item is None:
            raise WorkspaceError("invalid SKILL.md")
        return item
    finally:
        shutil.rmtree(temp_dir, ignore_errors=True)


@contextmanager
def _skill_lock(root: Path, scope: str, user_id: int, name: str) -> Iterator[None]:
    ensure_layout(root)
    lock_dir = root / ".locks"
    identity = str(user_id) if scope == "user" else "public"
    lock_path = lock_dir / f"{scope}-{identity}-{name}.lock"
    with lock_path.open("w") as handle:
        fcntl.flock(handle.fileno(), fcntl.LOCK_EX)
        try:
            yield
        finally:
            fcntl.flock(handle.fileno(), fcntl.LOCK_UN)


def delete_skill(root: Path, scope: str, user_id: int, name: str) -> None:
    if not SKILL_NAME_RE.fullmatch(name):
        raise WorkspaceError("invalid skill name")
    target_root = _target_dir(root, scope, user_id)
    target = (target_root / name).resolve()
    if not target.is_relative_to(target_root.resolve()) or not target.is_dir():
        raise NotFoundError("skill not found")
    with _skill_lock(Path(root), scope, user_id, name):
        shutil.rmtree(target)


def sync_public_skills(root: Path, user_id: int) -> None:
    root = Path(root).resolve()
    public_root = root / "public"
    skills_dir = _user_config_dir(root, user_id) / "skills"
    skills_dir.mkdir(parents=True, exist_ok=True)
    public_names: set[str] = set()
    if public_root.is_dir():
        for child in public_root.iterdir():
            if not child.is_dir() or not (child / "SKILL.md").is_file():
                continue
            if not SKILL_NAME_RE.fullmatch(child.name):
                continue
            public_names.add(child.name)
            target = skills_dir / child.name
            if target.is_dir() and not target.is_symlink():
                continue
            if target.is_symlink():
                if target.resolve() == child.resolve():
                    continue
                target.unlink()
            elif target.exists():
                continue
            target.symlink_to(child.resolve(), target_is_directory=True)

    for entry in skills_dir.iterdir():
        if not entry.is_symlink():
            continue
        try:
            resolved = entry.resolve()
        except OSError:
            entry.unlink(missing_ok=True)
            continue
        if entry.name not in public_names and resolved.is_relative_to(public_root.resolve()):
            entry.unlink(missing_ok=True)


def prepare_agent_config(root: Path, user_id: int) -> tuple[Path, Path]:
    root = Path(root).resolve()
    ensure_layout(root)
    config = _user_config_dir(root, user_id)
    (config / "skills").mkdir(parents=True, exist_ok=True)
    home = AGENT_HOME_ROOT / str(user_id)
    home.mkdir(parents=True, exist_ok=True)
    link = home / ".claude"
    if link.is_symlink():
        if link.resolve() != config:
            link.unlink()
            link.symlink_to(config, target_is_directory=True)
    elif link.exists():
        shutil.rmtree(link)
        link.symlink_to(config, target_is_directory=True)
    else:
        link.symlink_to(config, target_is_directory=True)
    sync_public_skills(root, user_id)
    return home, config

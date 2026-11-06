from __future__ import annotations

from collections.abc import Iterable
from datetime import UTC, datetime
from pathlib import Path

from app.config import Settings

MAX_SKILLS = 200
MAX_DESCRIPTION = 300
MAX_READ_BYTES = 16 * 1024


def list_skills(
    settings: Settings,
    project_dir: Path,
    user_id: int = 0,
    home: Path | None = None,
) -> list[dict[str, str]]:
    allowed = _allowed_names(settings.agent_skills)
    if allowed == set():
        return []

    found: dict[str, dict[str, str]] = {}
    for source, root in _roots(settings, project_dir, home or Path.home(), user_id):
        for skill_file in iter_skill_files(root):
            item = read_skill(skill_file, source)
            if item is None:
                continue
            if allowed is not None and item["name"] not in allowed:
                continue
            found.setdefault(item["name"], item)

    items = sorted(found.values(), key=lambda item: item["name"])
    return items[:MAX_SKILLS]


def _allowed_names(raw: str) -> set[str] | None:
    value = raw.strip()
    if not value or value == "all":
        return None
    if value == "none":
        return set()
    return {item.strip() for item in value.split(",") if item.strip()}


def _roots(
    settings: Settings,
    project_dir: Path,
    home: Path,
    user_id: int,
) -> Iterable[tuple[str, Path]]:
    yield "project", project_dir / ".claude" / "skills"
    if user_id > 0:
        yield "user", _skills_root(settings) / "users" / str(user_id) / ".claude" / "skills"
    else:
        yield "user", home / ".claude" / "skills"
    yield "public", _skills_root(settings) / "public"
    for raw in settings.agent_plugins.split(","):
        path = raw.strip()
        if not path:
            continue
        plugin = Path(path).expanduser()
        yield "plugin", plugin
        yield "plugin", plugin / "skills"


def _skills_root(settings: Settings) -> Path:
    if settings.skills_root is not None:
        return settings.skills_root.resolve()
    return (settings.workspaces_dir.parent / "skills").resolve()


def iter_skill_files(root: Path) -> Iterable[Path]:
    try:
        resolved_root = root.resolve()
    except OSError:
        return
    if not resolved_root.is_dir():
        return

    candidates = [resolved_root / "SKILL.md"]
    for child in sorted(resolved_root.iterdir()):
        if child.is_dir():
            candidates.append(child / "SKILL.md")

    for candidate in candidates:
        try:
            resolved = candidate.resolve()
        except OSError:
            continue
        if resolved.is_relative_to(resolved_root) and resolved.is_file():
            yield resolved


def read_skill(path: Path, source: str) -> dict[str, str] | None:
    try:
        text = path.read_text(encoding="utf-8", errors="replace")[:MAX_READ_BYTES]
    except OSError:
        return None

    meta = _parse_frontmatter(text)
    name = (meta.get("name") or path.parent.name).strip()
    if not name:
        return None

    description = (meta.get("description") or "").strip()
    if len(description) > MAX_DESCRIPTION:
        description = f"{description[: MAX_DESCRIPTION - 3]}..."

    try:
        updated_at = datetime.fromtimestamp(path.stat().st_mtime, tz=UTC).isoformat()
    except OSError:
        updated_at = ""
    return {
        "name": name,
        "description": description,
        "source": source,
        "updatedAt": updated_at,
    }


def _parse_frontmatter(text: str) -> dict[str, str]:
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}
    end = next((index for index in range(1, len(lines)) if lines[index].strip() == "---"), None)
    if end is None:
        return {}

    result: dict[str, str] = {}
    key: str | None = None
    block: list[str] = []

    def flush() -> None:
        nonlocal key, block
        if key and block:
            result[key] = " ".join(block).strip()
        key = None
        block = []

    for raw in lines[1:end]:
        if raw.startswith((" ", "\t")) and key:
            block.append(raw.strip())
            continue
        flush()
        if ":" not in raw:
            continue
        raw_key, raw_value = raw.split(":", 1)
        raw_key = raw_key.strip()
        raw_value = raw_value.strip()
        if not raw_key:
            continue
        if raw_value in ("|", ">"):
            key = raw_key
            block = []
            continue
        result[raw_key] = _unquote(raw_value)

    flush()
    return result


def _unquote(value: str) -> str:
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ("'", '"'):
        return value[1:-1]
    return value

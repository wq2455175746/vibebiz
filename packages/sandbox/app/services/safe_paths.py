import os
from pathlib import Path


class UnsafePathError(ValueError):
    pass


def resolve_within(path: Path, root: Path) -> Path:
    base = root.resolve()
    if path.is_symlink():
        raise UnsafePathError(f"symlink is not allowed: {path}")
    resolved = path.resolve()
    if not resolved.is_relative_to(base):
        raise UnsafePathError(f"path escapes root: {path}")
    return resolved


def read_bytes(path: Path, root: Path) -> bytes | None:
    resolved = resolve_within(path, root)
    try:
        fd = os.open(resolved, os.O_RDONLY | os.O_NOFOLLOW)
    except FileNotFoundError:
        return None
    with os.fdopen(fd, "rb") as handle:
        return handle.read()


def read_text(path: Path, root: Path, encoding: str = "utf-8") -> str | None:
    data = read_bytes(path, root)
    if data is None:
        return None
    return data.decode(encoding, errors="replace")


def write_text(
    path: Path,
    content: str,
    root: Path,
    encoding: str = "utf-8",
    mode: int = 0o600,
) -> None:
    resolved = resolve_within(path, root)
    resolved.parent.mkdir(parents=True, exist_ok=True)
    resolved = resolve_within(resolved, root)
    fd = os.open(
        resolved,
        os.O_WRONLY | os.O_CREAT | os.O_TRUNC | os.O_NOFOLLOW,
        mode,
    )
    with os.fdopen(fd, "w", encoding=encoding) as handle:
        handle.write(content)


def open_append(path: Path, root: Path, mode: int = 0o600) -> int:
    resolved = resolve_within(path, root)
    resolved.parent.mkdir(parents=True, exist_ok=True)
    resolved = resolve_within(resolved, root)
    return os.open(
        resolved,
        os.O_APPEND | os.O_CREAT | os.O_WRONLY | os.O_NOFOLLOW,
        mode,
    )

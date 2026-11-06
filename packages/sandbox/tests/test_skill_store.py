import io
import zipfile
from pathlib import Path

import pytest

from app.errors import ConflictError, WorkspaceError
from app.services.skill_store import (
    delete_skill,
    ensure_layout,
    list_stored_skills,
    prepare_agent_config,
    save_skill_package,
    sync_public_skills,
)


def skill_zip(name: str = "demo", description: str = "demo skill") -> io.BytesIO:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr(f"{name}/SKILL.md", f"---\nname: {name}\ndescription: {description}\n---\n")
        zf.writestr(f"{name}/scripts/run.sh", "echo hi\n")
    buf.seek(0)
    return buf


def test_save_and_list_public_and_user(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("public-one"), False)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)

    assert [s["name"] for s in list_stored_skills(root, "public")] == ["public-one"]
    assert [s["name"] for s in list_stored_skills(root, "user", 7)] == ["mine"]
    assert (root / "users/7/.claude/skills/mine/SKILL.md").is_file()


def test_conflict_and_overwrite(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("demo", "v1"), False)
    with pytest.raises(ConflictError):
        save_skill_package(root, "public", 0, skill_zip("demo", "v2"), False)
    save_skill_package(root, "public", 0, skill_zip("demo", "v2"), True)
    assert list_stored_skills(root, "public")[0]["description"] == "v2"


def test_rejects_path_traversal(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("../escape/SKILL.md", "x")
    buf.seek(0)
    with pytest.raises(WorkspaceError):
        save_skill_package(root, "public", 0, buf, False)


def test_rejects_symlink_entry(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        info = zipfile.ZipInfo("demo/link")
        info.external_attr = (0o120777 << 16) | 0o777
        zf.writestr(info, "../../outside")
    buf.seek(0)
    with pytest.raises(WorkspaceError):
        save_skill_package(root, "public", 0, buf, False)


def test_rejects_multiple_top_level_dirs(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("one/SKILL.md", "x")
        zf.writestr("two/SKILL.md", "x")
    buf.seek(0)
    with pytest.raises(WorkspaceError):
        save_skill_package(root, "public", 0, buf, False)


def test_rejects_invalid_name(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr("Bad Name/SKILL.md", "x")
    buf.seek(0)
    with pytest.raises(WorkspaceError):
        save_skill_package(root, "public", 0, buf, False)


def test_sync_public_and_user_wins(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("shared", "public"), False)
    sync_public_skills(root, 7)
    assert (root / "users/7/.claude/skills/shared").is_symlink()

    save_skill_package(root, "user", 7, skill_zip("shared", "user"), False)
    sync_public_skills(root, 7)
    assert not (root / "users/7/.claude/skills/shared").is_symlink()


def test_sync_removes_stale_link(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("shared"), False)
    sync_public_skills(root, 7)
    delete_skill(root, "public", 0, "shared")
    sync_public_skills(root, 7)
    assert not (root / "users/7/.claude/skills/shared").exists()


def test_prepare_agent_config_uses_persistent_claude_dir(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)
    home, config = prepare_agent_config(root, 7)
    assert home.is_dir()
    assert (home / ".claude").is_symlink()
    assert config == root / "users/7/.claude"
    assert (home / ".claude/skills/mine/SKILL.md").is_file()


def test_delete_user_and_public(tmp_path: Path):
    root = tmp_path / "skills"
    ensure_layout(root)
    save_skill_package(root, "public", 0, skill_zip("shared"), False)
    save_skill_package(root, "user", 7, skill_zip("mine"), False)
    delete_skill(root, "user", 7, "mine")
    delete_skill(root, "public", 0, "shared")
    assert list_stored_skills(root, "user", 7) == []
    assert list_stored_skills(root, "public") == []

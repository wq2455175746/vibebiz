from pathlib import Path

import pytest

from app.errors import ConflictError, WorkspaceError
from app.services.workspace import Workspace


def make_workspace(tmp_path: Path) -> Workspace:
    return Workspace(tmp_path / "workspaces")


def test_init_creates_starter_files(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.init_project("p1")

    assert (project / "index.html").is_file()
    assert (project / "styles.css").is_file()
    assert "Vibebiz" in (project / "index.html").read_text(encoding="utf-8")


def test_invalid_project_id(tmp_path):
    workspace = make_workspace(tmp_path)
    with pytest.raises(WorkspaceError):
        workspace.project_dir("../escape")


def test_path_traversal_is_rejected(tmp_path):
    workspace = make_workspace(tmp_path)
    workspace.init_project("p1")

    with pytest.raises(WorkspaceError):
        workspace.read_file("p1", "../../etc/passwd")


def test_symlink_escape_is_rejected(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.init_project("p1")
    secret = tmp_path / "secret.txt"
    secret.write_text("top secret", encoding="utf-8")
    (project / "link.txt").symlink_to(secret)

    with pytest.raises(WorkspaceError):
        workspace.read_file("p1", "link.txt")


def test_write_uses_base_version(tmp_path):
    workspace = make_workspace(tmp_path)
    workspace.init_project("p1")
    current = workspace.read_file("p1", "index.html")

    updated = workspace.write_file("p1", "index.html", "<h1>v2</h1>", current["version"])
    assert updated["version"] != current["version"]
    assert workspace.read_file("p1", "index.html")["content"] == "<h1>v2</h1>"

    with pytest.raises(ConflictError):
        workspace.write_file("p1", "index.html", "<h1>stale</h1>", current["version"])


def test_binary_detection(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.init_project("p1")
    (project / "blob.bin").write_bytes(b"\x00\x01\x02")

    result = workspace.read_file("p1", "blob.bin")
    assert result["isBinary"] is True
    assert result["content"] == ""


def test_list_files_ignores_internal_dirs(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.init_project("p1")
    (project / ".git").mkdir()
    (project / ".git" / "config").write_text("x", encoding="utf-8")

    paths = {entry["path"] for entry in workspace.list_files("p1")}
    assert "index.html" in paths
    assert all(not path.startswith(".git") for path in paths)

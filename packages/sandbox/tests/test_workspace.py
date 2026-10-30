import os
from pathlib import Path

import pytest

from app.errors import ConflictError, NotFoundError, WorkspaceError
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


def test_stats_for_empty_project(tmp_path):
    workspace = make_workspace(tmp_path)
    workspace.project_dir("empty").mkdir()

    stats = workspace.stats("empty")

    assert stats == {
        "fileSizeBytes": 0,
        "fileCount": 0,
        "dirCount": 0,
        "lastModifiedAt": 0,
    }


def test_stats_counts_nested_files_and_dirs(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.project_dir("p1")
    project.mkdir()
    (project / "src").mkdir()
    (project / "src" / "components").mkdir()
    target = project / "src" / "components" / "button.vue"
    target.write_bytes(b"12345")
    (project / "index.html").write_bytes(b"123")
    os.utime(target, (1793191707, 1793191707))
    os.utime(project / "index.html", (1793191700, 1793191700))

    stats = workspace.stats("p1")

    assert stats["fileSizeBytes"] == 8
    assert stats["fileCount"] == 2
    assert stats["dirCount"] == 2
    assert stats["lastModifiedAt"] == 1793191707000


def test_stats_ignores_internal_dirs(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.project_dir("p1")
    project.mkdir()
    (project / "app.js").write_bytes(b"abc")
    for ignored in (".git", ".agent", "node_modules", "__pycache__", ".venv", ".pytest_cache"):
        ignored_dir = project / ignored
        ignored_dir.mkdir()
        (ignored_dir / "junk.bin").write_bytes(b"x" * 1000)

    stats = workspace.stats("p1")

    assert stats["fileSizeBytes"] == 3
    assert stats["fileCount"] == 1
    assert stats["dirCount"] == 0


def test_stats_missing_project_raises_not_found(tmp_path):
    workspace = make_workspace(tmp_path)

    with pytest.raises(NotFoundError):
        workspace.stats("missing")


def test_stats_invalid_project_id(tmp_path):
    workspace = make_workspace(tmp_path)

    with pytest.raises(WorkspaceError):
        workspace.stats("bad id")


def test_chathistory_is_internal(tmp_path):
    workspace = make_workspace(tmp_path)
    project = workspace.init_project("p1")
    history = project / ".chathistory"
    history.mkdir()
    (history / "session.jsonl").write_text("{}", encoding="utf-8")

    paths = {entry["path"] for entry in workspace.list_files("p1")}
    assert all(not path.startswith(".chathistory") for path in paths)
    with pytest.raises(WorkspaceError):
        workspace.read_file("p1", ".chathistory/session.jsonl")


def test_archive_round_trip_replaces_files(tmp_path):
    source = make_workspace(tmp_path / "source")
    source_project = source.init_project("p1")
    (source_project / "styles.css").write_text("body{}", encoding="utf-8")
    (source_project / "nested").mkdir()
    (source_project / "nested" / "app.js").write_text("console.log(1)", encoding="utf-8")
    data = source.archive("p1")

    target = make_workspace(tmp_path / "target")
    target.init_project("p1")
    (target.project_dir("p1") / "stale.txt").write_text("stale", encoding="utf-8")
    target.import_archive("p1", data)

    project = target.project_dir("p1")
    assert (project / "styles.css").read_text(encoding="utf-8") == "body{}"
    assert (project / "nested" / "app.js").read_text(encoding="utf-8") == "console.log(1)"
    assert not (project / "stale.txt").exists()


def test_import_archive_rejects_path_traversal(tmp_path):
    import io
    import tarfile

    workspace = make_workspace(tmp_path)
    workspace.init_project("p1")
    buffer = io.BytesIO()
    with tarfile.open(fileobj=buffer, mode="w:gz") as archive:
        info = tarfile.TarInfo("../evil.txt")
        payload = b"bad"
        info.size = len(payload)
        archive.addfile(info, io.BytesIO(payload))

    with pytest.raises(WorkspaceError):
        workspace.import_archive("p1", buffer.getvalue())

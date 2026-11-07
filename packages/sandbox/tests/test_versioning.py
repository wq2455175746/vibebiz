import shutil

import pytest

from app.services.versioning import Versioning


def test_commit_rejects_symlinked_git_dir(tmp_path):
    project = tmp_path / "project"
    project.mkdir()
    evil = tmp_path / "evil"
    evil.mkdir()
    (project / ".git").symlink_to(evil)

    versioning = Versioning()

    assert versioning.commit(project, "test") is None


def test_commit_skips_project_hooks(tmp_path):
    if shutil.which("git") is None:
        pytest.skip("git is not installed")
    project = tmp_path / "project"
    project.mkdir()
    (project / "a.txt").write_text("a", encoding="utf-8")
    versioning = Versioning()
    versioning.init(project)

    marker = tmp_path / "hook-ran"
    hook = project / ".git" / "hooks" / "pre-commit"
    hook.write_text(f"#!/bin/sh\ntouch {marker}\n", encoding="utf-8")
    hook.chmod(0o755)

    commit = versioning.commit(project, "test")

    assert commit is not None
    assert not marker.exists()


def test_gitignore_symlink_is_not_followed(tmp_path):
    project = tmp_path / "project"
    project.mkdir()
    victim = tmp_path / "victim.txt"
    victim.write_text("keep", encoding="utf-8")
    (project / ".gitignore").symlink_to(victim)

    Versioning()._ensure_gitignore(project)

    assert victim.read_text(encoding="utf-8") == "keep"

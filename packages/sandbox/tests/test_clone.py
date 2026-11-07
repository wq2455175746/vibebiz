import asyncio
import json
import sys
import zipfile
from pathlib import Path

import pytest

from app.errors import WorkspaceError
from app.services.clone import CloneService
from app.services.workspace import Workspace


def make_workspace(tmp_path: Path) -> tuple[Workspace, Path]:
    workspace = Workspace(tmp_path / "workspaces")
    project = workspace.init_project("p1")
    return workspace, project


def write_zip(project: Path, storage_name: str, files: dict[str, str]) -> None:
    root = project / ".resources"
    root.mkdir(parents=True, exist_ok=True)
    target = root / storage_name
    with zipfile.ZipFile(target, "w") as archive:
        for name, content in files.items():
            archive.writestr(name, content)


def setup_payload(storage_name: str = "tests.zip") -> dict:
    return {
        "name": "Acme",
        "description": "clone acme",
        "sourceUrl": "https://acme.example.com",
        "assets": [
            {
                "role": "unit_test",
                "name": "tests.zip",
                "storageName": storage_name,
                "contentType": "application/zip",
                "kind": "document",
                "size": 100,
            }
        ],
    }


def test_sync_writes_source_and_extracts_tests(tmp_path):
    workspace, project = make_workspace(tmp_path)
    write_zip(project, "r1.zip", {"test_ok.py": "def test_ok():\n    assert True\n"})
    service = CloneService(workspace)

    service.sync("p1", setup_payload("r1.zip"))

    source = json.loads((project / ".clone" / "source.json").read_text(encoding="utf-8"))
    assert source["name"] == "Acme"
    assert source["sourceUrl"] == "https://acme.example.com"
    assert source["assets"][0]["role"] == "unit_test"
    assert (project / ".clone" / "tests" / "test_ok.py").is_file()
    paths = {entry["path"] for entry in workspace.list_files("p1")}
    assert all(not path.startswith(".clone") for path in paths)


def test_sync_rejects_zip_traversal(tmp_path):
    workspace, project = make_workspace(tmp_path)
    write_zip(project, "r1.zip", {"../evil.txt": "bad"})
    service = CloneService(workspace)

    with pytest.raises(WorkspaceError):
        service.sync("p1", setup_payload("r1.zip"))
    assert not (project.parent / "evil.txt").exists()


def test_sync_clears_tests_when_unit_test_removed(tmp_path):
    workspace, project = make_workspace(tmp_path)
    write_zip(project, "r1.zip", {"test_ok.py": "def test_ok():\n    assert True\n"})
    service = CloneService(workspace)
    service.sync("p1", setup_payload("r1.zip"))
    assert (project / ".clone" / "tests" / "test_ok.py").is_file()

    payload = setup_payload()
    payload["assets"] = []
    service.sync("p1", payload)

    assert not (project / ".clone" / "tests").exists()


def test_run_tests_passes_and_writes_report(tmp_path):
    workspace, project = make_workspace(tmp_path)
    write_zip(project, "r1.zip", {"test_ok.py": "def test_ok():\n    assert True\n"})
    service = CloneService(workspace, timeout_seconds=30)
    service.sync("p1", setup_payload("r1.zip"))

    result = asyncio.run(service.run_tests("p1"))

    assert result["status"] == "passed"
    assert result["exitCode"] == 0
    report = (project / "docs" / "clone" / "test-report.md").read_text(encoding="utf-8")
    assert "通过" in report
    assert "test_ok" in report


def test_run_tests_fails_and_writes_report(tmp_path):
    workspace, project = make_workspace(tmp_path)
    write_zip(project, "r1.zip", {"test_bad.py": "def test_bad():\n    assert False\n"})
    service = CloneService(workspace, timeout_seconds=30)
    service.sync("p1", setup_payload("r1.zip"))

    result = asyncio.run(service.run_tests("p1"))

    assert result["status"] == "failed"
    assert result["exitCode"] != 0
    report = (project / "docs" / "clone" / "test-report.md").read_text(encoding="utf-8")
    assert "失败" in report


def test_run_tests_cancellation_kills_subprocess(tmp_path, monkeypatch):
    workspace, project = make_workspace(tmp_path)
    service = CloneService(workspace, timeout_seconds=30)
    create_subprocess = asyncio.create_subprocess_exec
    processes = []

    async def capture(*args, **kwargs):
        process = await create_subprocess(*args, **kwargs)
        processes.append(process)
        return process

    monkeypatch.setattr(asyncio, "create_subprocess_exec", capture)
    monkeypatch.setattr(
        service,
        "_detect_command",
        lambda _project, _tests_dir: (
            [sys.executable, "-c", "import time; time.sleep(5)"],
            project,
        ),
    )

    async def scenario() -> tuple[int | None, bool]:
        process = None
        try:
            task = asyncio.create_task(service.run_tests("p1"))
            while not processes:
                await asyncio.sleep(0.01)
            process = processes[0]
            task.cancel()
            with pytest.raises(asyncio.CancelledError):
                await task
            return process.returncode, (project / "docs" / "clone" / "test-report.md").exists()
        finally:
            if process is not None and process.returncode is None:
                process.kill()
                await process.wait()

    returncode, report_written = asyncio.run(scenario())

    assert returncode is not None
    assert report_written is False


def test_run_tests_skips_without_tests(tmp_path):
    workspace, _ = make_workspace(tmp_path)
    service = CloneService(workspace)

    result = asyncio.run(service.run_tests("p1"))

    assert result["status"] == "skipped"

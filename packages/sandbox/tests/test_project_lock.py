import asyncio
import subprocess
import sys

import pytest

from app.config import Settings, load_settings
from app.main import create_app
from app.observability import PROJECT_LOCK_TIMEOUT
from app.services.project_lock import ProjectLock, ProjectLockBusy


def test_project_lock_rejects_second_writer(tmp_path):
    project = tmp_path / "p1"
    project.mkdir()
    lock = ProjectLock(timeout=0.2)

    async def run() -> None:
        async with lock.acquire(project):
            with pytest.raises(ProjectLockBusy):
                async with lock.acquire(project):
                    pass

    asyncio.run(run())


def test_different_projects_do_not_block(tmp_path):
    lock = ProjectLock(timeout=0.2)

    async def run() -> None:
        async with lock.acquire(tmp_path / "p1"), lock.acquire(tmp_path / "p2"):
            pass

    asyncio.run(run())


def test_sync_lock_creates_vibebiz_dir_and_rejects_second_writer(tmp_path):
    project = tmp_path / "p1"
    lock = ProjectLock(timeout=0.2)

    with lock.acquire_sync(project):
        assert (project / ".vibebiz" / "lock").exists()
        with pytest.raises(ProjectLockBusy), lock.acquire_sync(project):
            pass


def test_lock_is_busy_across_processes(tmp_path):
    project = tmp_path / "p1"
    lock_path = project / ".vibebiz" / "lock"
    lock_path.parent.mkdir(parents=True)
    script = (
        "import fcntl, sys, time\n"
        "handle = open(sys.argv[1], 'a+')\n"
        "fcntl.flock(handle.fileno(), fcntl.LOCK_EX)\n"
        "print('ready', flush=True)\n"
        "time.sleep(30)\n"
    )
    child = subprocess.Popen(
        [sys.executable, "-c", script, str(lock_path)],
        stdout=subprocess.PIPE,
        text=True,
    )
    try:
        assert child.stdout is not None
        assert child.stdout.readline().strip() == "ready"
        with pytest.raises(ProjectLockBusy), ProjectLock(timeout=0.2).acquire_sync(project):
            pass
    finally:
        child.terminate()
        child.wait(timeout=5)
        assert child.stdout is not None
        child.stdout.close()


def test_lock_timeout_increments_metric(tmp_path):
    project = tmp_path / "p1"
    lock = ProjectLock(timeout=0.1)
    before = PROJECT_LOCK_TIMEOUT.labels(project_id="p1")._value.get()

    with lock.acquire_sync(project), pytest.raises(ProjectLockBusy), lock.acquire_sync(project):
        pass

    assert PROJECT_LOCK_TIMEOUT.labels(project_id="p1")._value.get() == before + 1


def test_lock_timeout_is_parsed_from_env(monkeypatch, tmp_path):
    monkeypatch.setenv("WORKSPACES_DIR", str(tmp_path / "workspaces"))
    monkeypatch.setenv("SANDBOX_LOCK_TIMEOUT", "1.5s")
    assert load_settings().sandbox_lock_timeout == 1.5


def test_app_exposes_project_lock_with_configured_timeout(tmp_path):
    settings = Settings(workspaces_dir=tmp_path / "workspaces", sandbox_lock_timeout=0.75)
    app = create_app(settings)
    assert isinstance(app.state.project_lock, ProjectLock)
    assert app.state.project_lock.timeout == 0.75

import asyncio
import fcntl
import time
from collections.abc import AsyncIterator, Iterator
from contextlib import asynccontextmanager, contextmanager
from pathlib import Path

from app.errors import WorkspaceError
from app.observability import PROJECT_LOCK_TIMEOUT, PROJECT_LOCK_WAIT


class ProjectLockBusy(WorkspaceError):
    status = 423
    code = 42300


class ProjectLock:
    def __init__(self, timeout: float = 2.0) -> None:
        self.timeout = max(timeout, 0.05)

    def _open(self, project_dir: Path):
        lock_dir = project_dir / ".vibebiz"
        lock_dir.mkdir(parents=True, exist_ok=True)
        return (lock_dir / "lock").open("a+")

    @contextmanager
    def acquire_sync(self, project_dir: Path) -> Iterator[None]:
        started = time.monotonic()
        handle = self._open(project_dir)
        deadline = started + self.timeout
        try:
            while True:
                try:
                    fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        PROJECT_LOCK_TIMEOUT.labels(project_id=project_dir.name).inc()
                        raise ProjectLockBusy("project is busy") from None
                    time.sleep(0.05)
            PROJECT_LOCK_WAIT.labels(operation="sync", project_id=project_dir.name).observe(
                time.monotonic() - started
            )
            yield
        finally:
            try:
                fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
            finally:
                handle.close()

    @asynccontextmanager
    async def acquire(self, project_dir: Path) -> AsyncIterator[None]:
        started = time.monotonic()
        handle = self._open(project_dir)
        deadline = started + self.timeout
        try:
            while True:
                try:
                    fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
                    break
                except BlockingIOError:
                    if time.monotonic() >= deadline:
                        PROJECT_LOCK_TIMEOUT.labels(project_id=project_dir.name).inc()
                        raise ProjectLockBusy("project is busy") from None
                    await asyncio.sleep(0.05)
            PROJECT_LOCK_WAIT.labels(operation="async", project_id=project_dir.name).observe(
                time.monotonic() - started
            )
            yield
        finally:
            try:
                fcntl.flock(handle.fileno(), fcntl.LOCK_UN)
            finally:
                handle.close()

import asyncio
import json
from contextlib import suppress
from pathlib import Path

import pytest

from app.services.chat_history import ChatHistory
from app.services.chat_turns import ChatTurnManager, TurnLocked, iter_turn_events
from app.services.project_lease import StaleProjectEpoch, read_epoch
from app.services.project_lock import ProjectLock, ProjectLockBusy
from app.services.workspace import Workspace


class FakeVersioning:
    def commit(self, _project_dir: Path, _message: str) -> str:
        return "commit-1"


class SlowAgent:
    mode = "stub"

    async def run(self, *_args, **_kwargs):
        await asyncio.sleep(0.01)
        yield {
            "type": "agent",
            "payload": {
                "type": "assistant",
                "message": {"content": [{"type": "text", "text": "done"}]},
            },
        }
        yield {
            "type": "agent",
            "payload": {
                "type": "result",
                "subtype": "success",
                "durationMs": 12,
                "numTurns": 1,
            },
        }


class GateAgent:
    mode = "stub"

    def __init__(self) -> None:
        self.started = asyncio.Event()
        self.release = asyncio.Event()

    async def run(self, *_args, **_kwargs):
        self.started.set()
        await self.release.wait()
        yield {"type": "agent", "payload": {"type": "result", "subtype": "success"}}


class InstantAgent:
    mode = "stub"

    async def run(self, *_args, **_kwargs):
        yield {"type": "agent", "payload": {"type": "result", "subtype": "success"}}


class BlockingAssistantHistory(ChatHistory):
    def __init__(self, workspace: Workspace) -> None:
        super().__init__(workspace)
        self.in_finally = asyncio.Event()

    async def append_assistant(self, *_args, **_kwargs) -> None:
        self.in_finally.set()
        await asyncio.Event().wait()


async def wait_until_inactive(manager: ChatTurnManager, project_id: str = "p1") -> None:
    for _ in range(100):
        if await manager.status(project_id) == {"active": False, "turnId": None}:
            return
        await asyncio.sleep(0.01)
    assert await manager.status(project_id) == {"active": False, "turnId": None}


async def collect_turn_events(manager: ChatTurnManager, turn) -> list:
    return [event async for event in iter_turn_events(manager, turn)]


def write_lease(project: Path, epoch: int) -> None:
    (project / ".vibebiz").mkdir(parents=True, exist_ok=True)
    (project / ".vibebiz" / "lease").write_text(
        json.dumps({"epoch": epoch, "sandbox": "sandbox-a"}), encoding="utf-8"
    )


def test_turn_holds_project_lock_until_finished(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    workspace.init_project("p1")
    history = ChatHistory(workspace)
    lock = ProjectLock(timeout=0.1)
    agent = GateAgent()
    manager = ChatTurnManager(
        agent, history, FakeVersioning(), project_lock=lock, sandbox_name="sandbox-a"
    )
    project_dir = workspace.project_dir("p1")

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace)
        await agent.started.wait()
        with pytest.raises(ProjectLockBusy):
            async with lock.acquire(project_dir):
                pass
        agent.release.set()
        await turn.task
        async with lock.acquire(project_dir):
            pass

    asyncio.run(run())


def test_turn_rejects_stale_epoch_and_releases_lock(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    project_dir = workspace.init_project("p1")
    write_lease(project_dir, 5)
    history = ChatHistory(workspace)
    lock = ProjectLock(timeout=0.1)
    manager = ChatTurnManager(
        GateAgent(), history, FakeVersioning(), project_lock=lock, sandbox_name="sandbox-b"
    )

    async def run() -> None:
        with pytest.raises(StaleProjectEpoch):
            await manager.start("p1", "hello", workspace, epoch=4)
        async with lock.acquire(project_dir):
            pass
        assert await manager.status("p1") == {"active": False, "turnId": None}

    asyncio.run(run())


def test_turn_updates_lease_with_newer_epoch(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    project_dir = workspace.init_project("p1")
    write_lease(project_dir, 4)
    history = ChatHistory(workspace)
    agent = GateAgent()
    manager = ChatTurnManager(
        agent,
        history,
        FakeVersioning(),
        project_lock=ProjectLock(timeout=0.1),
        sandbox_name="sandbox-a",
    )

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace, epoch=5)
        await agent.started.wait()
        assert read_epoch(project_dir) == 5
        agent.release.set()
        await turn.task

    asyncio.run(run())


def test_stop_before_task_runs_releases_lock(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    project_dir = workspace.init_project("p1")
    history = ChatHistory(workspace)
    lock = ProjectLock(timeout=0.1)
    agent = GateAgent()
    manager = ChatTurnManager(
        agent, history, FakeVersioning(), project_lock=lock, sandbox_name="sandbox-a"
    )

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace)
        assert not agent.started.is_set()
        assert await manager.stop("p1") is True
        with suppress(asyncio.CancelledError):
            await turn.task
        await wait_until_inactive(manager)
        assert turn.lock_cm is None
        events = await asyncio.wait_for(collect_turn_events(manager, turn), 0.5)
        assert events == []
        async with lock.acquire(project_dir):
            pass

    asyncio.run(run())


def test_stop_during_finally_releases_lock(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    project_dir = workspace.init_project("p1")
    history = BlockingAssistantHistory(workspace)
    lock = ProjectLock(timeout=0.1)
    manager = ChatTurnManager(
        InstantAgent(), history, FakeVersioning(), project_lock=lock, sandbox_name="sandbox-a"
    )

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace)
        await history.in_finally.wait()
        assert await manager.stop("p1") is True
        with suppress(asyncio.CancelledError):
            await turn.task
        await wait_until_inactive(manager)
        assert turn.lock_cm is None
        events = await asyncio.wait_for(collect_turn_events(manager, turn), 0.5)
        assert events
        async with lock.acquire(project_dir):
            pass

    asyncio.run(run())


def test_cancelled_turn_emits_terminal_event(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    workspace.init_project("p1")
    history = ChatHistory(workspace)
    lock = ProjectLock(timeout=0.1)
    agent = GateAgent()
    manager = ChatTurnManager(
        agent, history, FakeVersioning(), project_lock=lock, sandbox_name="sandbox-a"
    )

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace)
        assert await manager.stop("p1") is True
        with suppress(asyncio.CancelledError):
            await turn.task
        events = await asyncio.wait_for(collect_turn_events(manager, turn), 0.5)
        assert events == []
        async with lock.acquire(workspace.project_dir("p1")):
            pass

    asyncio.run(run())


def test_turn_completes_without_sse_subscriber(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    workspace.init_project("p1")
    history = ChatHistory(workspace)
    manager = ChatTurnManager(SlowAgent(), history, FakeVersioning(), model="deepseek-flash")

    async def run() -> None:
        await manager.start("p1", "hello", workspace)
        await asyncio.sleep(0.05)

    asyncio.run(run())

    path = history.path("p1")
    entries = [json.loads(line) for line in path.read_text(encoding="utf-8").splitlines()]
    messages = [entry for entry in entries if entry["type"] == "message"]
    assert [item["message"]["role"] for item in messages] == ["user", "assistant"]
    assert messages[1]["message"]["content"] == "done"
    assert messages[1]["message"]["status"] == "completed"
    assert messages[1]["parentId"] == messages[0]["id"]


def test_turn_guard_blocks_workspace_writes_while_active(tmp_path):
    workspace = Workspace(tmp_path / "workspaces")
    workspace.init_project("p1")
    history = ChatHistory(workspace)
    manager = ChatTurnManager(SlowAgent(), history, FakeVersioning(), model="deepseek-flash")

    async def run() -> None:
        turn = await manager.start("p1", "hello", workspace)
        assert await manager.status("p1") == {"active": True, "turnId": turn.id}
        with pytest.raises(TurnLocked):
            async with manager.write_guard("p1"):
                pass
        await turn.task
        assert await manager.status("p1") == {"active": False, "turnId": None}

    asyncio.run(run())

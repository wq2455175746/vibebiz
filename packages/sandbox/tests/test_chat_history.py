import asyncio
import json
from pathlib import Path

import pytest

from app.services.chat_history import ChatHistory
from app.services.chat_turns import ChatTurnManager, TurnLocked
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

from pathlib import Path

from claude_agent_sdk import (
    AssistantMessage,
    ResultMessage,
    TextBlock,
    ToolResultBlock,
    ToolUseBlock,
    UserMessage,
)

from app.config import Settings
from app.services.agent import build_agent
from app.services.claude_agent import build_options, serialize_sdk_message


def settings(tmp_path: Path) -> Settings:
    return Settings(
        workspaces_dir=tmp_path / "workspaces",
        agent_mode="claude",
        anthropic_base_url="http://127.0.0.1:4000",
        anthropic_auth_token="test-token",
        agent_model="deepseek-flash",
    )


def test_build_agent_uses_claude_when_configured(tmp_path):
    agent = build_agent(settings(tmp_path))
    assert agent.mode == "claude"


def test_build_options_uses_anthropic_gateway_and_tools(tmp_path):
    options = build_options(settings(tmp_path), tmp_path / "project", None)

    assert options.model == "deepseek-flash"
    assert options.cwd == str(tmp_path / "project")
    assert options.env["ANTHROPIC_BASE_URL"] == "http://127.0.0.1:4000"
    assert options.env["ANTHROPIC_AUTH_TOKEN"] == "test-token"
    assert options.permission_mode == "acceptEdits"
    assert "Read" in options.allowed_tools
    assert "Write" in options.allowed_tools
    assert options.skills == "all"


def test_build_options_supports_official_anthropic_without_gateway(tmp_path):
    direct = Settings(
        workspaces_dir=tmp_path / "workspaces",
        agent_mode="claude",
        anthropic_base_url="",
        anthropic_api_key="sk-ant-test",
        agent_model="claude-sonnet-5",
    )
    options = build_options(direct, tmp_path / "project", None)

    assert "ANTHROPIC_BASE_URL" not in options.env
    assert options.env["ANTHROPIC_API_KEY"] == "sk-ant-test"
    assert options.model == "claude-sonnet-5"


def test_serialize_assistant_text_and_tool_use():
    message = AssistantMessage(
        content=[
            TextBlock(text="先看一下项目"),
            ToolUseBlock(id="tool-1", name="Read", input={"file_path": "index.html"}),
        ],
        model="deepseek-chat",
        session_id="session-1",
    )

    event = serialize_sdk_message(message)

    assert event["type"] == "agent"
    assert event["payload"]["type"] == "assistant"
    assert event["payload"]["sessionId"] == "session-1"
    content = event["payload"]["message"]["content"]
    assert content[0] == {"type": "text", "text": "先看一下项目"}
    assert content[1] == {
        "type": "tool_use",
        "id": "tool-1",
        "name": "Read",
        "input": {"file_path": "index.html"},
    }


def test_serialize_tool_result_user_message():
    message = UserMessage(
        content=[ToolResultBlock(tool_use_id="tool-1", content="ok", is_error=False)],
    )

    event = serialize_sdk_message(message)

    assert event["payload"]["type"] == "user"
    block = event["payload"]["message"]["content"][0]
    assert block == {"type": "tool_result", "tool_use_id": "tool-1", "content": "ok"}


def test_serialize_result_message():
    message = ResultMessage(
        subtype="success",
        duration_ms=1200,
        duration_api_ms=900,
        is_error=False,
        num_turns=2,
        session_id="session-1",
        result="done",
    )

    event = serialize_sdk_message(message)

    assert event["payload"]["type"] == "result"
    assert event["payload"]["sessionId"] == "session-1"
    assert event["payload"]["subtype"] == "success"
    assert event["payload"]["isError"] is False


def test_claude_agent_reuses_session_between_turns(tmp_path, monkeypatch):
    import asyncio

    from claude_agent_sdk import ResultMessage

    from app.services import claude_agent as module
    from app.services.workspace import Workspace

    seen: list[str | None] = []

    async def fake_query(prompt: str, options):
        seen.append(options.resume)
        yield ResultMessage(
            subtype="success",
            duration_ms=1,
            duration_api_ms=1,
            is_error=False,
            num_turns=1,
            session_id="session-1",
            result="done",
        )

    monkeypatch.setattr(module, "query", fake_query)
    agent = build_agent(settings(tmp_path))
    workspace = Workspace(tmp_path / "workspaces")
    workspace.init_project("p1")

    async def run_turn() -> None:
        async for _ in agent.run("p1", "继续", workspace):
            pass

    asyncio.run(run_turn())
    asyncio.run(run_turn())

    assert seen == [None, "session-1"]

from collections.abc import AsyncIterator
from pathlib import Path
from typing import Any

from claude_agent_sdk import (
    AssistantMessage,
    ClaudeAgentOptions,
    ResultMessage,
    SystemMessage,
    TextBlock,
    ToolResultBlock,
    ToolUseBlock,
    UserMessage,
    query,
)

from app.config import Settings
from app.observability import AGENT_RUNS, get_logger, get_tracer
from app.services.agent_roles import ALLOWED_TOOLS, build_agent_definitions, build_role_prompt
from app.services.workspace import Workspace

logger = get_logger()


def build_options(
    settings: Settings,
    cwd: str | Path,
    session_id: str | None = None,
    agent_roles: list[dict[str, Any]] | None = None,
    team_mode: bool = False,
) -> ClaudeAgentOptions:
    env: dict[str, str] = {}
    if settings.anthropic_base_url:
        env["ANTHROPIC_BASE_URL"] = settings.anthropic_base_url
    if settings.anthropic_api_key:
        env["ANTHROPIC_API_KEY"] = settings.anthropic_api_key
    if settings.anthropic_auth_token:
        env["ANTHROPIC_AUTH_TOKEN"] = settings.anthropic_auth_token

    skills: list[str] | str | None = None
    if settings.agent_skills and settings.agent_skills != "none":
        skills = (
            "all"
            if settings.agent_skills == "all"
            else [item.strip() for item in settings.agent_skills.split(",") if item.strip()]
        )

    plugins = [
        {"type": "local", "path": path}
        for path in (item.strip() for item in settings.agent_plugins.split(","))
        if path
    ]

    role_prompt = build_role_prompt(agent_roles or [], team_mode)
    agent_defs = build_agent_definitions(agent_roles or [], team_mode)
    system_prompt = None
    if role_prompt:
        system_prompt = {
            "type": "preset",
            "preset": "claude_code",
            "append": role_prompt,
        }
    allowed_tools = list(ALLOWED_TOOLS)
    if agent_defs:
        allowed_tools.append("Agent")

    return ClaudeAgentOptions(
        cwd=str(cwd),
        model=settings.agent_model,
        env=env,
        max_turns=settings.agent_max_turns,
        permission_mode="acceptEdits",
        allowed_tools=allowed_tools,
        setting_sources=["user", "project"],
        skills=skills,
        plugins=plugins,
        system_prompt=system_prompt,
        agents=agent_defs or None,
        forward_subagent_text=False,
        resume=session_id,
    )


class ClaudeAgent:
    mode = "claude"

    def __init__(self, settings: Settings) -> None:
        self.settings = settings
        self._sessions: dict[str, str] = {}

    async def run(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        agent_roles: list[dict[str, Any]] | None = None,
        team_mode: bool = False,
    ) -> AsyncIterator[dict]:
        tracer = get_tracer()
        with tracer.start_as_current_span("agent.run") as span:
            span.set_attribute("project.id", project_id)
            span.set_attribute("agent.mode", self.mode)
            span.set_attribute("agent.model", self.settings.agent_model)
            logger.info(
                "agent.run.start",
                extra={
                    "project_id": project_id,
                    "mode": self.mode,
                    "model": self.settings.agent_model,
                    "status": "running",
                },
            )
            status = "error"
            try:
                async for event in self._run(
                    project_id, message, workspace, session_id, agent_roles, team_mode
                ):
                    yield event
                status = "success"
            except Exception as exc:
                logger.error(
                    "agent.run.error",
                    extra={
                        "project_id": project_id,
                        "mode": self.mode,
                        "status": "error",
                        "code": getattr(exc, "code", 50000),
                        "error_message": str(exc),
                    },
                    exc_info=True,
                )
                raise
            finally:
                span.set_attribute("agent.status", status)
                logger.info(
                    "agent.run.end",
                    extra={
                        "project_id": project_id,
                        "mode": self.mode,
                        "status": status,
                    },
                )
                AGENT_RUNS.labels(project_id=project_id, mode=self.mode, status=status).inc()

    async def _run(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        agent_roles: list[dict[str, Any]] | None = None,
        team_mode: bool = False,
    ) -> AsyncIterator[dict]:
        project_dir = workspace.project_dir(project_id)
        session = session_id or self._sessions.get(project_id)
        options = build_options(self.settings, project_dir, session, agent_roles, team_mode)
        async for sdk_message in query(prompt=message, options=options):
            if isinstance(sdk_message, ResultMessage) and sdk_message.session_id:
                self._sessions[project_id] = sdk_message.session_id
            event = serialize_sdk_message(sdk_message)
            if event:
                yield event


def serialize_sdk_message(message: Any) -> dict | None:
    if isinstance(message, AssistantMessage):
        return agent_event(
            {
                "type": "assistant",
                "sessionId": message.session_id,
                "message": {"content": serialize_blocks(message.content)},
                "error": message.error,
            }
        )

    if isinstance(message, UserMessage):
        content = message.content
        blocks = serialize_blocks(content) if isinstance(content, list) else []
        if isinstance(content, str):
            blocks = [{"type": "text", "text": content}]
        return agent_event(
            {
                "type": "user",
                "message": {"content": blocks},
            }
        )

    if isinstance(message, ResultMessage):
        return agent_event(
            {
                "type": "result",
                "subtype": message.subtype,
                "sessionId": message.session_id,
                "isError": message.is_error,
                "durationMs": message.duration_ms,
                "durationApiMs": message.duration_api_ms,
                "numTurns": message.num_turns,
                "totalCostUsd": message.total_cost_usd,
                "result": message.result,
            }
        )

    if isinstance(message, SystemMessage):
        return agent_event(
            {
                "type": "system",
                "subtype": message.subtype,
                "data": message.data,
            }
        )

    return None


def serialize_blocks(blocks: list[Any]) -> list[dict]:
    result: list[dict] = []
    for block in blocks:
        serialized = serialize_block(block)
        if serialized:
            result.append(serialized)
    return result


def serialize_block(block: Any) -> dict | None:
    if isinstance(block, TextBlock):
        return {"type": "text", "text": block.text}
    if isinstance(block, ToolUseBlock):
        return {
            "type": "tool_use",
            "id": block.id,
            "name": block.name,
            "input": block.input,
        }
    if isinstance(block, ToolResultBlock):
        return {
            "type": "tool_result",
            "tool_use_id": block.tool_use_id,
            "content": block.content,
        }
    return None


def agent_event(payload: dict) -> dict:
    return {"type": "agent", "payload": payload}

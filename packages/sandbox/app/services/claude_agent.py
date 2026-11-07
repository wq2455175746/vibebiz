import json
import os
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
from app.services.web_tools import build_web_mcp_server, split_domains
from app.services.workspace import Workspace

logger = get_logger()

SENSITIVE_ENV_NAMES = {
    "ANTHROPIC_API_KEY",
    "ANTHROPIC_AUTH_TOKEN",
    "LLM_API_KEY",
    "SANDBOX_REGISTRY_TOKEN",
    "SMTP_PASSWORD",
    "NOTIFICATION_SECRET_KEY",
}
SENSITIVE_ENV_SUFFIXES = ("_KEY", "_TOKEN", "_SECRET", "_PASSWORD")
DENIED_SANDBOX_DOMAINS = [
    "localhost",
    "127.0.0.1",
    "::1",
    "169.254.169.254",
    "metadata.google.internal",
    "*.internal",
    "*.local",
]
PERMISSION_DENY = [
    "Read(./.env)",
    "Read(./.env.*)",
    "Edit(./.claude/**)",
    "Write(./.claude/**)",
    "Edit(./.mcp.json)",
    "Write(./.mcp.json)",
    "Edit(./.git/config)",
    "Write(./.git/config)",
    "Edit(./.git/hooks/**)",
    "Write(./.git/hooks/**)",
    "Edit(./.gitconfig)",
    "Write(./.gitconfig)",
]


def _sensitive_env_names() -> list[str]:
    names = set(SENSITIVE_ENV_NAMES)
    names.update(key for key in os.environ if key.upper().endswith(SENSITIVE_ENV_SUFFIXES))
    return sorted(names)


def build_agent_settings(
    settings: Settings,
    project_dir: str | Path,
    config_dir: str | Path | None = None,
    skills_root: str | Path | None = None,
) -> str:
    project = Path(project_dir).resolve()
    workspaces = settings.workspaces_dir.resolve()
    deny_read = [
        str(workspaces),
        "/proc/*/environ",
        "/proc/*/cmdline",
        "/root",
        "/etc/shadow",
    ]
    allow_read = [str(project)]
    deny_write = ["/etc", "/root", "/var/run", "/run"]

    if config_dir is not None:
        allow_read.append(str(Path(config_dir).resolve()))
    if skills_root is not None:
        root = Path(skills_root).resolve()
        deny_read.append(str(root))
        deny_write.append(str(root))
        allow_read.append(str((root / "public").resolve()))

    payload: dict[str, Any] = {
        "permissions": {
            "blockReadsOutsideWorkingDirectories": True,
            "blockWritesOutsideWorkingDirectories": True,
            "deny": list(PERMISSION_DENY),
        }
    }
    if settings.agent_sandbox_enabled and settings.agent_network_restricted:
        payload["sandbox"] = {
            "enabled": True,
            "failIfUnavailable": True,
            "allowUnsandboxedCommands": False,
            "autoAllowBashIfSandboxed": True,
            "enableWeakerNestedSandbox": settings.agent_sandbox_weaker_nested,
            "filesystem": {
                "denyRead": deny_read,
                "allowRead": allow_read,
                "denyWrite": deny_write,
            },
            "network": {
                "allowedDomains": split_domains(settings.agent_sandbox_allowed_domains),
                "deniedDomains": list(DENIED_SANDBOX_DOMAINS),
            },
            "credentials": {
                "envVars": [{"name": name, "mode": "deny"} for name in _sensitive_env_names()]
            },
        }
    return json.dumps(payload, ensure_ascii=False)


def build_options(
    settings: Settings,
    cwd: str | Path,
    session_id: str | None = None,
    agent_roles: list[dict[str, Any]] | None = None,
    team_mode: bool = False,
    system_prompt_text: str = "",
    user_id: int = 0,
) -> ClaudeAgentOptions:
    env: dict[str, str] = {"CLAUDE_CODE_SUBPROCESS_ENV_SCRUB": "1"}
    if settings.anthropic_base_url:
        env["ANTHROPIC_BASE_URL"] = settings.anthropic_base_url
    if settings.anthropic_api_key:
        env["ANTHROPIC_API_KEY"] = settings.anthropic_api_key
    if settings.anthropic_auth_token:
        env["ANTHROPIC_AUTH_TOKEN"] = settings.anthropic_auth_token
    config_dir: Path | None = None
    skills_root = settings.skills_root.resolve() if settings.skills_root else None
    if user_id > 0:
        try:
            from app.services.skill_store import prepare_agent_config, resolve_skills_root

            skills_root = resolve_skills_root(settings)
            home, config = prepare_agent_config(skills_root, user_id)
            config_dir = config
            env["HOME"] = str(home)
            env["CLAUDE_CONFIG_DIR"] = str(config)
        except OSError:
            logger.warning(
                "skills.config.unavailable",
                extra={"user_id": user_id, "status": "fallback"},
            )
    else:
        anonymous_home = Path("/tmp/vibebiz-sandbox-home/anonymous")
        config_dir = anonymous_home / ".claude"
        try:
            config_dir.mkdir(parents=True, exist_ok=True)
            env["HOME"] = str(anonymous_home)
            env["CLAUDE_CONFIG_DIR"] = str(config_dir)
        except OSError:
            config_dir = None
            logger.warning("agent.config.anonymous_unavailable", extra={"status": "fallback"})

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
    web_server, web_tools = build_web_mcp_server(settings)
    agent_defs = build_agent_definitions(agent_roles or [], team_mode, extra_tools=web_tools)
    prompt_parts = [part for part in (role_prompt, system_prompt_text) if part]
    system_prompt = None
    if prompt_parts:
        system_prompt = {
            "type": "preset",
            "preset": "claude_code",
            "append": "\n\n".join(prompt_parts),
        }
    allowed_tools = list(ALLOWED_TOOLS)
    allowed_tools.extend(web_tools)
    if agent_defs:
        allowed_tools.append("Agent")
    mcp_servers = {"web": web_server} if web_server else {}

    return ClaudeAgentOptions(
        cwd=str(cwd),
        model=settings.agent_model,
        env=env,
        max_turns=settings.agent_max_turns,
        permission_mode="acceptEdits",
        allowed_tools=allowed_tools,
        disallowed_tools=["WebFetch", "WebSearch"],
        settings=build_agent_settings(settings, cwd, config_dir, skills_root),
        setting_sources=["user"],
        mcp_servers=mcp_servers,
        strict_mcp_config=True,
        skills=skills,
        plugins=plugins,
        system_prompt=system_prompt,
        agents=agent_defs or None,
        forward_subagent_text=False,
        resume=session_id,
        max_buffer_size=settings.agent_max_buffer_bytes,
    )


def _missing_session(exc: Exception) -> bool:
    return "no conversation found" in str(exc).lower()


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
        system_prompt: str = "",
        context: str = "",
        user_id: int = 0,
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
                    project_id,
                    message,
                    workspace,
                    session_id,
                    agent_roles,
                    team_mode,
                    system_prompt,
                    context,
                    user_id=user_id,
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
        system_prompt: str = "",
        context: str = "",
        user_id: int = 0,
    ) -> AsyncIterator[dict]:
        project_dir = workspace.project_dir(project_id)
        session = session_id or self._sessions.get(project_id)
        produced = False
        try:
            async for event in self._stream(
                project_id,
                message,
                project_dir,
                session,
                agent_roles,
                team_mode,
                system_prompt,
                context,
                user_id=user_id,
            ):
                if event["payload"]["type"] != "system":
                    produced = True
                yield event
        except Exception as exc:
            if session and not produced and _missing_session(exc):
                self._sessions.pop(project_id, None)
                async for event in self._stream(
                    project_id,
                    message,
                    project_dir,
                    None,
                    agent_roles,
                    team_mode,
                    system_prompt,
                    context,
                    user_id=user_id,
                ):
                    yield event
                return
            raise

    async def _stream(
        self,
        project_id: str,
        message: str,
        project_dir: Path,
        session: str | None,
        agent_roles: list[dict[str, Any]] | None,
        team_mode: bool,
        system_prompt: str,
        context: str,
        user_id: int = 0,
    ) -> AsyncIterator[dict]:
        if context:
            message = f"最近对话上下文：\n{context}\n\n当前任务：\n{message}"
        options = build_options(
            self.settings,
            project_dir,
            session,
            agent_roles,
            team_mode,
            system_prompt,
            user_id=user_id,
        )
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
                "usage": message.usage,
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
                "usage": message.usage,
                "modelUsage": message.model_usage,
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

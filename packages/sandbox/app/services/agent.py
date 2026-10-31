import html
import uuid
from collections.abc import AsyncIterator
from datetime import UTC, datetime
from typing import Protocol

from app.config import Settings
from app.observability import AGENT_RUNS, get_logger, get_tracer
from app.services.agent_roles import build_role_prompt
from app.services.workspace import Workspace

logger = get_logger()


class Agent(Protocol):
    mode: str

    def run(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        agent_roles: list[dict] | None = None,
        team_mode: bool = False,
        system_prompt: str = "",
        context: str = "",
    ) -> AsyncIterator[dict]: ...


def build_agent(settings: Settings) -> Agent:
    if settings.agent_mode == "claude":
        from app.services.claude_agent import ClaudeAgent

        return ClaudeAgent(settings)
    llm_ready = bool(settings.llm_api_key) and settings.agent_mode == "llm"
    return StubAgent(llm_ready=llm_ready, mode="llm" if llm_ready else "stub")


class StubAgent:
    """Deterministic agent used by the harness until the LLM loop lands."""

    def __init__(self, llm_ready: bool = False, mode: str = "stub") -> None:
        self.llm_ready = llm_ready
        self.mode = mode

    async def run(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        agent_roles: list[dict] | None = None,
        team_mode: bool = False,
        system_prompt: str = "",
        context: str = "",
    ) -> AsyncIterator[dict]:
        tracer = get_tracer()
        with tracer.start_as_current_span("agent.run") as span:
            span.set_attribute("project.id", project_id)
            span.set_attribute("agent.mode", self.mode)
            logger.info(
                "agent.run.start",
                extra={"project_id": project_id, "mode": self.mode, "status": "running"},
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
                    extra={"project_id": project_id, "mode": self.mode, "status": status},
                )
                AGENT_RUNS.labels(project_id=project_id, mode=self.mode, status=status).inc()

    async def _run(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        agent_roles: list[dict] | None = None,
        team_mode: bool = False,
        system_prompt: str = "",
        context: str = "",
    ) -> AsyncIterator[dict]:
        session = session_id or str(uuid.uuid4())
        prompt = message.strip()
        if context:
            prompt = f"最近对话上下文：\n{context}\n\n当前任务：\n{prompt}"
        if system_prompt:
            prompt = f"{system_prompt}\n\n{prompt}"
        role_text = build_role_prompt(agent_roles or [], team_mode)
        if role_text:
            prompt = f"{role_text}\n\n用户请求：\n{prompt}"

        yield agent_event(
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "text",
                            "text": "收到需求，我先看一下当前页面结构，然后生成一版可运行的结果。",
                        }
                    ]
                },
            }
        )

        current = workspace.read_file(project_id, "index.html")
        yield agent_event(
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "tool_read_1",
                            "name": "Read",
                            "input": {"file_path": "index.html"},
                        }
                    ]
                },
            }
        )
        yield agent_event(
            {
                "type": "user",
                "message": {
                    "content": [
                        {
                            "type": "tool_result",
                            "tool_use_id": "tool_read_1",
                            "content": f"index.html ({current['size']} bytes)",
                        }
                    ]
                },
            }
        )

        page = render_page(prompt)
        result = workspace.write_file(project_id, "index.html", page, current["version"])
        yield agent_event(
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "tool_use",
                            "id": "tool_write_1",
                            "name": "Write",
                            "input": {"file_path": "index.html", "bytes": result["size"]},
                        }
                    ]
                },
            }
        )
        yield agent_event(
            {
                "type": "user",
                "message": {
                    "content": [
                        {
                            "type": "tool_result",
                            "tool_use_id": "tool_write_1",
                            "content": f"written version={result['version']}",
                        }
                    ]
                },
            }
        )

        yield agent_event(
            {
                "type": "assistant",
                "message": {
                    "content": [
                        {
                            "type": "text",
                            "text": "第一版已经写好，右侧预览会自动刷新。继续告诉我要调整什么即可。"
                            if not self.llm_ready
                            else "页面已更新。",
                        }
                    ]
                },
            }
        )
        yield agent_event(
            {
                "type": "result",
                "subtype": "success",
                "sessionId": session,
                "version": result["version"],
            }
        )


def agent_event(payload: dict) -> dict:
    return {"type": "agent", "payload": payload}


def render_page(prompt: str) -> str:
    safe = html.escape(prompt)
    title = html.escape(prompt if len(prompt) <= 32 else prompt[:32] + "...")
    stamp = datetime.now(UTC).strftime("%Y-%m-%d %H:%M UTC")
    return f"""<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>{title}</title>
    <link rel="stylesheet" href="styles.css" />
  </head>
  <body>
    <main class="shell">
      <p class="eyebrow">GENERATED BY VIBEBIZ</p>
      <h1 id="hero-title">{title}</h1>
      <p id="hero-subtitle" class="subtitle">{safe}</p>
      <div class="row">
        <button id="counter" class="action" type="button">点击试试</button>
        <span class="badge">harness demo</span>
      </div>
      <p class="meta">生成时间 {stamp}</p>
    </main>
    <script src="app.js"></script>
  </body>
</html>
"""

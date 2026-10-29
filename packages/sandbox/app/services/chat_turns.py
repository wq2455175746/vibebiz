import asyncio
import uuid
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from dataclasses import dataclass, field
from typing import Any

from app.errors import WorkspaceError
from app.observability import get_logger
from app.services.agent import Agent
from app.services.chat_history import ChatHistory
from app.services.versioning import Versioning
from app.services.workspace import Workspace

logger = get_logger()


class TurnInProgress(WorkspaceError):
    status = 409
    code = 40901


class TurnLocked(WorkspaceError):
    status = 423
    code = 42300


@dataclass
class Turn:
    id: str
    project_id: str
    message: str
    references: list[dict[str, Any]]
    queue: asyncio.Queue[dict[str, Any] | None]
    task: asyncio.Task | None = None
    subscribers: set[asyncio.Queue[dict[str, Any] | None]] = field(default_factory=set)


class TurnCollector:
    def __init__(self) -> None:
        self.texts: list[str] = []
        self.tools: dict[str, dict[str, Any]] = {}
        self.result_text = ""
        self.duration_ms: int | None = None
        self.num_turns: int | None = None
        self.result_error = False

    def consume(self, event: dict[str, Any]) -> None:
        if event.get("type") != "agent":
            return
        payload = event.get("payload")
        if not isinstance(payload, dict):
            return
        kind = payload.get("type")
        if kind == "assistant":
            self._consume_assistant(payload)
        elif kind == "user":
            self._consume_tool_results(payload)
        elif kind == "result":
            self._consume_result(payload)

    def _consume_assistant(self, payload: dict[str, Any]) -> None:
        message = payload.get("message")
        if not isinstance(message, dict):
            return
        blocks = message.get("content")
        if not isinstance(blocks, list):
            return
        for block in blocks:
            if not isinstance(block, dict):
                continue
            if block.get("type") == "text" and isinstance(block.get("text"), str):
                text = block["text"].strip()
                if text:
                    self.texts.append(text)
            if block.get("type") == "tool_use":
                tool_id = str(block.get("id") or "")
                name = block.get("name")
                if tool_id and isinstance(name, str):
                    self.tools[tool_id] = {"id": tool_id, "name": name, "status": "running"}

    def _consume_tool_results(self, payload: dict[str, Any]) -> None:
        message = payload.get("message")
        if not isinstance(message, dict):
            return
        blocks = message.get("content")
        if not isinstance(blocks, list):
            return
        for block in blocks:
            if not isinstance(block, dict) or block.get("type") != "tool_result":
                continue
            tool_id = str(block.get("tool_use_id") or "")
            tool = self.tools.get(tool_id)
            if tool is not None:
                tool["status"] = "error" if block.get("is_error") else "done"

    def _consume_result(self, payload: dict[str, Any]) -> None:
        if isinstance(payload.get("result"), str):
            self.result_text = payload["result"].strip()
        duration = payload.get("durationMs", payload.get("duration_ms"))
        if isinstance(duration, int):
            self.duration_ms = duration
        turns = payload.get("numTurns", payload.get("num_turns"))
        if isinstance(turns, int):
            self.num_turns = turns
        self.result_error = bool(payload.get("isError") or payload.get("is_error"))

    def content(self) -> str:
        if self.texts:
            return "\n\n".join(self.texts)
        return self.result_text

    def finalize(self, status: str) -> list[dict[str, Any]]:
        if status != "completed":
            for tool in self.tools.values():
                if tool["status"] == "running":
                    tool["status"] = "error"
        else:
            for tool in self.tools.values():
                if tool["status"] == "running":
                    tool["status"] = "done"
        return list(self.tools.values())


class ChatTurnManager:
    def __init__(
        self,
        agent: Agent,
        history: ChatHistory,
        versioning: Versioning,
        model: str = "",
        timeout_seconds: int = 600,
    ) -> None:
        self.agent = agent
        self.history = history
        self.versioning = versioning
        self.model = model
        self.timeout_seconds = timeout_seconds
        self._active: dict[str, Turn] = {}
        self._lock = asyncio.Lock()

    async def start(
        self,
        project_id: str,
        message: str,
        workspace: Workspace,
        session_id: str | None = None,
        references: list[dict[str, Any]] | None = None,
    ) -> Turn:
        async with self._lock:
            if project_id in self._active:
                raise TurnInProgress("turn in progress")
            turn_id = uuid.uuid4().hex
            await self.history.append_user(project_id, message, turn_id, references)
            turn = Turn(
                id=turn_id,
                project_id=project_id,
                message=message,
                references=references or [],
                queue=asyncio.Queue(),
            )
            turn.subscribers.add(turn.queue)
            self._active[project_id] = turn
            turn.task = asyncio.create_task(self._run(turn, workspace, session_id))
            return turn

    async def stop(self, project_id: str) -> bool:
        async with self._lock:
            turn = self._active.get(project_id)
        if turn is None or turn.task is None or turn.task.done():
            return False
        turn.task.cancel()
        return True

    async def status(self, project_id: str) -> dict[str, Any]:
        async with self._lock:
            turn = self._active.get(project_id)
            return {"active": turn is not None, "turnId": turn.id if turn is not None else None}

    @asynccontextmanager
    async def write_guard(self, project_id: str) -> AsyncIterator[None]:
        async with self._lock:
            if project_id in self._active:
                raise TurnLocked("agent is running")
            yield

    def unsubscribe(self, turn: Turn, queue: asyncio.Queue[dict[str, Any] | None]) -> None:
        turn.subscribers.discard(queue)

    async def _run(self, turn: Turn, workspace: Workspace, session_id: str | None) -> None:
        collector = TurnCollector()
        status = "completed"
        error_message = ""
        cancelled = False
        try:
            async with asyncio.timeout(self.timeout_seconds):
                async for event in self.agent.run(
                    turn.project_id,
                    build_prompt(turn.message, turn.references),
                    workspace,
                    session_id,
                ):
                    collector.consume(event)
                    self._broadcast(turn, event)
            if collector.result_error:
                status = "error"
                error_message = "agent result reported an error"
            else:
                commit = self.versioning.commit(
                    workspace.project_dir(turn.project_id), f"turn: {turn.message[:60]}"
                )
                self._broadcast(
                    turn, {"type": "sandbox-status", "state": "ready", "commit": commit}
                )
        except asyncio.CancelledError:
            cancelled = True
            status = "stopped"
            error_message = "turn stopped"
        except TimeoutError:
            status = "stopped"
            error_message = "turn timed out"
        except Exception as exc:
            status = "error"
            error_message = str(exc)
            logger.error(
                "chat.stream.error",
                extra={
                    "project_id": turn.project_id,
                    "turn_id": turn.id,
                    "status": "error",
                    "code": 50000,
                    "error_message": error_message,
                },
                exc_info=True,
            )
            self._broadcast(turn, {"type": "error", "message": error_message})
        finally:
            tools = collector.finalize(status)
            content = collector.content()
            if status == "error" and not content:
                content = error_message
            try:
                await self.history.append_assistant(
                    turn.project_id,
                    turn.id,
                    content,
                    tools,
                    status,
                    model=self.model,
                    duration_ms=collector.duration_ms,
                    num_turns=collector.num_turns,
                    error_message=error_message,
                )
            except Exception:
                logger.exception(
                    "chat.history.write_failed",
                    extra={"project_id": turn.project_id, "turn_id": turn.id},
                )
            self._broadcast(turn, None)
            async with self._lock:
                if self._active.get(turn.project_id) is turn:
                    self._active.pop(turn.project_id, None)
            if cancelled:
                raise asyncio.CancelledError

    def _broadcast(self, turn: Turn, event: dict[str, Any] | None) -> None:
        for queue in list(turn.subscribers):
            try:
                queue.put_nowait(event)
            except asyncio.QueueFull:
                turn.subscribers.discard(queue)


def sse_done() -> dict[str, Any] | None:
    return None


async def iter_turn_events(
    manager: ChatTurnManager, turn: Turn
) -> AsyncIterator[dict[str, Any] | None]:
    try:
        while True:
            event = await turn.queue.get()
            if event is None:
                return
            yield event
    finally:
        manager.unsubscribe(turn, turn.queue)


def build_prompt(message: str, references: list[dict[str, Any]]) -> str:
    if not references:
        return message
    lines = ["用户消息：", message, "", "本轮引用文件（位于当前项目工作区）："]
    for item in references:
        path = str(item.get("path", ""))
        if item.get("kind") == "project_file":
            lines.append(f"- {path}（代码文件：直接读取）")
            continue
        name = str(item.get("name", path))
        content_type = str(item.get("contentType", ""))
        resource_kind = str(item.get("resourceKind", ""))
        if resource_kind == "document":
            action = "文档：使用 document-skills 读取"
        elif resource_kind == "image":
            action = "图片：按需 Read，不自动 OCR"
        else:
            action = "文本：直接读取"
        lines.append(f"- {path}（原始名：{name}，{content_type}，{action}）")
    lines.extend(["", "资源文件只读，不要修改或删除，除非用户明确要求。"])
    return "\n".join(lines)

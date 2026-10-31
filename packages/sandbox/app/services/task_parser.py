import json
from typing import Any

import httpx

from app.config import Settings
from app.errors import WorkspaceError
from app.observability import get_logger

logger = get_logger()


class TaskParseUnavailable(WorkspaceError):
    status = 503
    code = 50301


class TaskParseFailed(WorkspaceError):
    status = 502
    code = 50201


SYSTEM_PROMPT = """你是定时任务解析器。请把用户需求解析成 JSON，只输出 JSON，不要输出解释。
字段：
name: 任务名称，最多 128 字
description: 实际执行时发给 Agent 的任务描述
scheduleType: daily、weekly 或 cron
time: 每日/每周时间，HH:MM
weekdays: 每周的星期数组，0=周日，1=周一，最多 7 个
cronExpression: 5 段 cron；daily/weekly 也要给出等价表达式
timezone: IANA 时区，默认 Asia/Shanghai
"""


async def parse_task(settings: Settings, prompt: str) -> dict[str, Any]:
    if not settings.llm_api_key:
        raise TaskParseUnavailable("LLM_API_KEY is not configured")
    body = {
        "model": settings.llm_model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0.1,
    }
    try:
        async with httpx.AsyncClient(timeout=30) as client:
            response = await client.post(
                settings.llm_base_url.rstrip("/") + "/chat/completions",
                headers={"Authorization": f"Bearer {settings.llm_api_key}"},
                json=body,
            )
        response.raise_for_status()
    except Exception as exc:
        logger.error(
            "task.parse.llm_error",
            extra={"status": "error", "error_message": str(exc)},
        )
        raise TaskParseFailed("task parse failed") from exc
    content = response.json()["choices"][0]["message"]["content"]
    if not isinstance(content, str):
        raise TaskParseFailed("empty parse result")
    raw = content.strip()
    if raw.startswith("```"):
        raw = raw.strip("`")
        if raw.startswith("json"):
            raw = raw[4:]
    start, end = raw.find("{"), raw.rfind("}")
    if start < 0 or end <= start:
        raise TaskParseFailed("invalid parse result")
    try:
        data = json.loads(raw[start : end + 1])
    except json.JSONDecodeError as exc:
        raise TaskParseFailed("invalid parse result") from exc
    if not isinstance(data, dict):
        raise TaskParseFailed("invalid parse result")
    return normalize_draft(data, prompt)


def normalize_draft(data: dict[str, Any], prompt: str) -> dict[str, Any]:
    schedule_type = str(data.get("scheduleType") or "daily")
    if schedule_type not in {"daily", "weekly", "cron"}:
        raise TaskParseFailed("invalid schedule type")
    weekdays = data.get("weekdays") or []
    if not isinstance(weekdays, list):
        weekdays = []
    weekdays = [int(item) for item in weekdays if isinstance(item, int) and 0 <= item <= 6]
    description = str(data.get("description") or prompt).strip()[:8000]
    name = str(data.get("name") or description[:24]).strip()[:128]
    return {
        "name": name,
        "description": description,
        "scheduleType": schedule_type,
        "time": str(data.get("time") or "09:00"),
        "weekdays": weekdays,
        "cronExpression": str(data.get("cronExpression") or ""),
        "timezone": str(data.get("timezone") or "Asia/Shanghai"),
    }

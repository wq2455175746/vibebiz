from __future__ import annotations

import uuid
from typing import Any

import httpx

from app.config import Settings
from app.observability import get_logger

logger = get_logger()

MODEL_LIST_PATH = "/v1/models"
MODEL_CHECK_TIMEOUT_SECONDS = 5.0


def _headers(settings: Settings) -> dict[str, str]:
    headers = {"anthropic-version": "2023-06-01"}
    token = settings.anthropic_auth_token or settings.anthropic_api_key
    if token:
        if settings.anthropic_auth_token:
            headers["Authorization"] = f"Bearer {token}"
        else:
            headers["x-api-key"] = token
    if "opencode.ai" in settings.anthropic_base_url:
        headers["x-opencode-session"] = f"vibebiz-sandbox-{uuid.uuid4().hex}"
    return headers


def _model_ids(payload: Any) -> list[str]:
    if not isinstance(payload, dict):
        return []
    data = payload.get("data")
    if not isinstance(data, list):
        return []
    ids: list[str] = []
    for item in data:
        if isinstance(item, dict):
            model_id = item.get("id")
            if isinstance(model_id, str) and model_id:
                ids.append(model_id)
    return ids


async def check_agent_model(
    settings: Settings,
    client: httpx.AsyncClient | None = None,
) -> bool | None:
    if not settings.agent_model_check:
        return None
    if settings.agent_mode != "claude":
        return None
    base_url = settings.anthropic_base_url.rstrip("/")
    if not base_url:
        return None
    if not (settings.anthropic_auth_token or settings.anthropic_api_key):
        logger.warning(
            "agent.model_check.skipped: missing ANTHROPIC_AUTH_TOKEN/ANTHROPIC_API_KEY",
            extra={"status": "skipped"},
        )
        return None

    owns_client = client is None
    if client is None:
        client = httpx.AsyncClient(timeout=MODEL_CHECK_TIMEOUT_SECONDS)
    try:
        response = await client.get(f"{base_url}{MODEL_LIST_PATH}", headers=_headers(settings))
        if response.status_code in (404, 405):
            logger.warning(
                "agent.model_check.unsupported: status=%s",
                response.status_code,
                extra={"status": "unsupported"},
            )
            return None
        response.raise_for_status()
        models = _model_ids(response.json())
        if settings.agent_model not in models:
            logger.error(
                "agent.model_unavailable: configured=%s available=%s",
                settings.agent_model,
                ", ".join(models[:20]) or "none",
                extra={"code": "agent.model_unavailable", "status": "unavailable"},
            )
            return False
        logger.info("agent.model_check.ok: model=%s", settings.agent_model)
        return True
    except (httpx.HTTPError, ValueError) as exc:
        logger.warning(
            "agent.model_check.failed: %s",
            exc,
            extra={"status": "unknown"},
        )
        return None
    finally:
        if owns_client:
            await client.aclose()

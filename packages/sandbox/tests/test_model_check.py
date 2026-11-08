import asyncio
from pathlib import Path

import httpx

from app.config import Settings
from app.services.model_check import check_agent_model


def settings(**overrides: object) -> Settings:
    values: dict[str, object] = {
        "workspaces_dir": Path("/tmp/vibebiz-sandbox-test"),
        "agent_mode": "claude",
        "anthropic_base_url": "https://gateway.example",
        "anthropic_auth_token": "token",
        "agent_model": "deepseek-flash",
    }
    values.update(overrides)
    return Settings(**values)  # type: ignore[arg-type]


def run(settings_obj: Settings, handler: httpx.MockTransport) -> bool | None:
    async def scenario() -> bool | None:
        async with httpx.AsyncClient(transport=handler) as client:
            return await check_agent_model(settings_obj, client)

    return asyncio.run(scenario())


def test_missing_model_returns_false() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["authorization"] == "Bearer token"
        return httpx.Response(200, json={"data": [{"id": "deepseek-v4.1-flash"}]})

    assert run(settings(), httpx.MockTransport(handler)) is False


def test_present_model_returns_true() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, json={"data": [{"id": "deepseek-flash"}]})

    assert run(settings(), httpx.MockTransport(handler)) is True


def test_unsupported_endpoint_returns_none() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(404)

    assert run(settings(), httpx.MockTransport(handler)) is None


def test_network_error_returns_none() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise httpx.ConnectError("boom", request=request)

    assert run(settings(), httpx.MockTransport(handler)) is None


def test_skips_when_disabled_or_not_claude() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        raise AssertionError("should not be called")

    assert run(settings(agent_model_check=False), httpx.MockTransport(handler)) is None
    assert run(settings(agent_mode="stub"), httpx.MockTransport(handler)) is None


def test_opencode_session_header_is_sent() -> None:
    def handler(request: httpx.Request) -> httpx.Response:
        assert request.headers["x-opencode-session"].startswith("vibebiz-sandbox-")
        return httpx.Response(200, json={"data": [{"id": "deepseek-flash"}]})

    settings_obj = settings(anthropic_base_url="https://opencode.ai/inference/go/anthropic")
    assert run(settings_obj, httpx.MockTransport(handler)) is True

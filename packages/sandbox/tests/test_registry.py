import asyncio
import types
from contextlib import suppress
from pathlib import Path

from app.config import Settings
from app.services import registry as registry_module
from app.services.registry import (
    ActiveRequestCounter,
    ActiveRequestsMiddleware,
    build_heartbeat,
    run_heartbeat,
)


def settings(**overrides: object) -> Settings:
    values = {
        "workspaces_dir": Path("/tmp/workspaces"),
        "sandbox_name": "sandbox-a",
        "advertise_scheme": "http",
        "advertise_host": "10.0.0.11",
        "advertise_port": 8000,
        "max_concurrency": 8,
        "cp_registry_url": "http://cp:8080/cp/api/v1/internal/sandboxes/heartbeat",
        "registry_token": "secret",
        "heartbeat_interval": 5.0,
    }
    values.update(overrides)
    return Settings(**values)


def test_build_heartbeat_payload():
    assert build_heartbeat(settings(), 3) == {
        "name": "sandbox-a",
        "scheme": "http",
        "host": "10.0.0.11",
        "port": 8000,
        "activeRequests": 3,
        "maxConcurrency": 8,
    }


def test_active_request_counter_thread_safe():
    counter = ActiveRequestCounter()
    counter.inc()
    counter.inc()
    counter.dec()
    assert counter.value == 1


def test_active_requests_middleware_counts_stream_until_final_body():
    counter = ActiveRequestCounter()
    app = types.SimpleNamespace(state=types.SimpleNamespace(active_requests=counter))
    observed = []
    sent = []

    async def downstream(scope, receive, send):
        await send({"type": "http.response.start", "status": 200, "headers": []})
        await send({"type": "http.response.body", "body": b"a", "more_body": True})
        observed.append(counter.value)
        await send({"type": "http.response.body", "body": b"b", "more_body": False})

    async def send(message):
        sent.append(message)

    middleware = ActiveRequestsMiddleware(downstream)
    asyncio.run(
        middleware(
            {"type": "http", "path": "/internal/projects/p1/chat", "app": app},
            lambda: None,
            send,
        )
    )
    assert observed == [1]
    assert len(sent) == 3
    assert counter.value == 0


def test_active_requests_middleware_ignores_health():
    counter = ActiveRequestCounter()
    app = types.SimpleNamespace(state=types.SimpleNamespace(active_requests=counter))
    observed = []

    async def downstream(scope, receive, send):
        observed.append(counter.value)
        await send({"type": "http.response.start", "status": 200, "headers": []})
        await send({"type": "http.response.body", "body": b"ok", "more_body": False})

    async def send(message):
        pass

    middleware = ActiveRequestsMiddleware(downstream)
    asyncio.run(
        middleware(
            {"type": "http", "path": "/health", "app": app},
            lambda: None,
            send,
        )
    )
    assert observed == [0]
    assert counter.value == 0


def test_heartbeat_disabled_without_url():
    asyncio.run(run_heartbeat(settings(cp_registry_url=""), ActiveRequestCounter()))


class FakeResponse:
    def raise_for_status(self) -> None:
        return None


class FakeClient:
    def __init__(self, calls: list[dict[str, object]]) -> None:
        self.calls = calls

    async def __aenter__(self) -> "FakeClient":
        return self

    async def __aexit__(self, *args: object) -> None:
        return None

    async def post(
        self, url: str, json: dict[str, object], headers: dict[str, str]
    ) -> FakeResponse:
        self.calls.append({"url": url, "json": json, "headers": headers})
        return FakeResponse()


def test_run_heartbeat_posts_payload(monkeypatch):
    calls: list[dict[str, object]] = []
    monkeypatch.setattr(registry_module.httpx, "AsyncClient", lambda timeout: FakeClient(calls))
    counter = ActiveRequestCounter()
    counter.inc()

    async def run_once():
        task = asyncio.create_task(run_heartbeat(settings(heartbeat_interval=3600), counter))
        await asyncio.sleep(0.01)
        task.cancel()
        with suppress(asyncio.CancelledError):
            await task

    asyncio.run(run_once())
    assert calls == [
        {
            "url": "http://cp:8080/cp/api/v1/internal/sandboxes/heartbeat",
            "json": build_heartbeat(settings(), 1),
            "headers": {"X-Sandbox-Token": "secret"},
        }
    ]

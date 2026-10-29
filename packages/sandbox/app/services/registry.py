import asyncio
import threading
from collections.abc import Awaitable, Callable
from typing import Any

import httpx

from app.config import Settings
from app.observability import get_logger

logger = get_logger()


class ActiveRequestCounter:
    def __init__(self) -> None:
        self._value = 0
        self._lock = threading.Lock()

    def inc(self) -> None:
        with self._lock:
            self._value += 1

    def dec(self) -> None:
        with self._lock:
            if self._value > 0:
                self._value -= 1

    @property
    def value(self) -> int:
        with self._lock:
            return self._value


class ActiveRequestsMiddleware:
    def __init__(self, app: Callable[..., Awaitable[None]]) -> None:
        self.app = app

    async def __call__(
        self,
        scope: dict[str, Any],
        receive: Callable[[], Awaitable[dict[str, Any]]],
        send: Callable[[dict[str, Any]], Awaitable[None]],
    ) -> None:
        if scope.get("type") != "http" or not scope.get("path", "").startswith("/internal/"):
            await self.app(scope, receive, send)
            return

        counter: ActiveRequestCounter = scope["app"].state.active_requests
        finished = False
        counter.inc()

        async def wrapped_send(message: dict[str, Any]) -> None:
            nonlocal finished
            await send(message)
            if (
                message["type"] == "http.response.body"
                and not message.get("more_body", False)
                and not finished
            ):
                finished = True
                counter.dec()

        try:
            await self.app(scope, receive, wrapped_send)
        finally:
            if not finished:
                counter.dec()


def build_heartbeat(settings: Settings, active_requests: int) -> dict[str, object]:
    return {
        "name": settings.sandbox_name,
        "scheme": settings.advertise_scheme,
        "host": settings.advertise_host,
        "port": settings.advertise_port,
        "activeRequests": active_requests,
        "maxConcurrency": settings.max_concurrency,
    }


async def run_heartbeat(settings: Settings, counter: ActiveRequestCounter) -> None:
    if not settings.cp_registry_url:
        return
    headers = {"X-Sandbox-Token": settings.registry_token}
    while True:
        try:
            async with httpx.AsyncClient(timeout=2.0) as client:
                response = await client.post(
                    settings.cp_registry_url,
                    json=build_heartbeat(settings, counter.value),
                    headers=headers,
                )
                response.raise_for_status()
        except Exception as exc:
            logger.warning(
                "sandbox.heartbeat.error",
                extra={"error": str(exc), "sandbox_name": settings.sandbox_name},
            )
        await asyncio.sleep(settings.heartbeat_interval)

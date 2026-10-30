from __future__ import annotations

import asyncio
import json
import logging
import os
import re
import sys
import time
import uuid
import zipfile
from collections.abc import Callable
from contextvars import ContextVar
from datetime import UTC, datetime
from pathlib import Path

from fastapi import FastAPI, Request
from fastapi.responses import Response
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.http.trace_exporter import OTLPSpanExporter
from opentelemetry.instrumentation.fastapi import FastAPIInstrumentor
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor, SpanExporter
from opentelemetry.sdk.trace.sampling import (
    ALWAYS_OFF,
    ALWAYS_ON,
    ParentBased,
    ParentBasedTraceIdRatio,
    Sampler,
    TraceIdRatioBased,
)
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

from app.services.project_logs import ProjectLogStore

SERVICE_NAME = os.getenv("OTEL_SERVICE_NAME", "").strip() or "vibebiz-sandbox"
PLANE = "sandbox"

HTTP_REQUESTS = Counter(
    "vibebiz_http_requests_total",
    "HTTP requests handled by the service",
    ["service", "plane", "route", "method", "status", "project_id"],
)
HTTP_DURATION = Histogram(
    "vibebiz_http_request_duration_seconds",
    "HTTP request duration in seconds",
    ["service", "plane", "route", "method"],
)
SANDBOX_REQUESTS = Counter(
    "vibebiz_sandbox_requests_total",
    "Sandbox operations",
    ["operation", "status", "project_id"],
)
AGENT_RUNS = Counter(
    "vibebiz_project_agent_runs_total",
    "Agent runs per project",
    ["project_id", "mode", "status"],
)
SSE_EVENTS = Counter(
    "vibebiz_sse_events_total",
    "SSE events emitted per project",
    ["project_id", "type"],
)

_PROJECT_PATH = re.compile(r"^/internal/projects/([^/]+)")
_request_id: ContextVar[str] = ContextVar("request_id", default="")
_project_id: ContextVar[str] = ContextVar("project_id", default="")
_logger = logging.getLogger("vibebiz.sandbox")
_tracer_provider: TracerProvider | None = None
_project_root: Path | None = None


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        span_context = trace.get_current_span().get_span_context()
        trace_id = ""
        span_id = ""
        if span_context.is_valid:
            trace_id = format(span_context.trace_id, "032x")
            span_id = format(span_context.span_id, "016x")

        payload = {
            "time": datetime.fromtimestamp(record.created, UTC)
            .isoformat(timespec="milliseconds")
            .replace("+00:00", "Z"),
            "level": record.levelname.lower(),
            "service": SERVICE_NAME,
            "plane": PLANE,
            "request_id": _request_id.get(),
            "trace_id": trace_id,
            "span_id": span_id,
            "project_id": _project_id.get() or getattr(record, "project_id", ""),
            "route": getattr(record, "route", ""),
            "method": getattr(record, "method", ""),
            "status": getattr(record, "status", None),
            "duration_ms": getattr(record, "duration_ms", None),
            "msg": record.getMessage(),
        }
        if getattr(record, "mode", None):
            payload["mode"] = record.mode
        if getattr(record, "code", None) is not None:
            payload["code"] = record.code
        if getattr(record, "error_message", None):
            payload["message"] = record.error_message
        if record.exc_info:
            payload["error"] = self.formatException(record.exc_info)
        return json.dumps(payload, ensure_ascii=False)


class _StdoutHandler(logging.StreamHandler):
    def emit(self, record: logging.LogRecord) -> None:
        self.stream = sys.stdout
        super().emit(record)


def _coroutine_id() -> str:
    try:
        task = asyncio.current_task()
        return str(id(task.get_coro())) if task else "-"
    except RuntimeError:
        return "-"


class PlainFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        timestamp = datetime.fromtimestamp(record.created).strftime("%Y-%m-%d %H:%M:%S")
        line = (
            f"{timestamp}.{int(record.msecs):03d} | {record.levelname} | "
            f"{record.threadName} | {record.thread} | "
            f"{_request_id.get() or '-'} | {_coroutine_id()} | {record.getMessage()}"
        )
        if record.exc_info:
            line += "\n" + self.formatException(record.exc_info)
        return line


def _project_source(record: logging.LogRecord) -> str:
    source = str(getattr(record, "source", "") or "")
    if source:
        return source
    message = record.getMessage()
    if message.startswith(("agent.", "chat.")):
        return "agent"
    if message.startswith("workspace."):
        return "sandbox"
    return "system"


class ProjectLogHandler(logging.Handler):
    def __init__(self, root: Path, retention_days: int = 30) -> None:
        super().__init__()
        self.store = ProjectLogStore(root, retention_days)

    def emit(self, record: logging.LogRecord) -> None:
        try:
            message = record.getMessage()
            if message in {"request.complete", "sandbox.call"} or message.startswith("workspace."):
                return
            project_id = str(getattr(record, "project_id", "") or _project_id.get())
            if not project_id:
                return
            level = record.levelname.lower()
            if level == "warning":
                level = "warn"
            elif level == "critical":
                level = "error"
            entry = {
                "ts": datetime.fromtimestamp(record.created, UTC)
                .isoformat(timespec="milliseconds")
                .replace("+00:00", "Z"),
                "level": level,
                "source": _project_source(record),
                "message": message,
            }
            self.store.append(project_id, entry, record.created)
        except Exception:
            self.handleError(record)


class DailyZipFileHandler(logging.Handler):
    def __init__(
        self,
        log_dir: str,
        prefix: str = "sandbox",
        retention_days: int = 180,
        now: Callable[[], datetime] | None = None,
    ) -> None:
        super().__init__()
        self.log_dir = log_dir
        self.prefix = prefix
        self.retention_days = retention_days
        self._now = now or datetime.now
        self._day = ""
        self._stream = None
        os.makedirs(log_dir, exist_ok=True)
        self._open(self._day_of(self._now()))

    def _day_of(self, value: datetime) -> str:
        return value.strftime("%Y-%m-%d")

    def _path(self, day: str) -> str:
        return os.path.join(self.log_dir, f"{self.prefix}_{day}.log")

    def _open(self, day: str) -> None:
        self._stream = open(self._path(day), "a", encoding="utf-8")  # noqa: SIM115
        self._day = day

    def emit(self, record: logging.LogRecord) -> None:
        try:
            day = self._day_of(self._now())
            if day != self._day:
                self._rotate(day)
            if self._stream is None:
                self._open(day)
            self._stream.write(self.format(record) + "\n")
            self._stream.flush()
        except Exception:
            self.handleError(record)

    def _rotate(self, day: str) -> None:
        if self._stream is not None:
            self._stream.close()
            self._stream = None
        if self._day:
            path = self._path(self._day)
            if os.path.exists(path):
                with zipfile.ZipFile(path + ".zip", "w", zipfile.ZIP_DEFLATED) as archive:
                    archive.write(path, arcname=os.path.basename(path))
                os.remove(path)
        self._cleanup()
        self._open(day)

    def _cleanup(self) -> None:
        cutoff = self._now().timestamp() - self.retention_days * 86400
        for name in os.listdir(self.log_dir):
            if not name.endswith(".zip"):
                continue
            path = os.path.join(self.log_dir, name)
            try:
                if os.path.getmtime(path) < cutoff:
                    os.remove(path)
            except OSError:
                continue

    def close(self) -> None:
        if self._stream is not None:
            self._stream.close()
            self._stream = None
        super().close()


def configure_logging(workspace_root: Path | None = None) -> None:
    global _project_root
    level_name = os.getenv("LOG_LEVEL", "info").upper()
    _logger.setLevel(getattr(logging, level_name, logging.INFO))
    _logger.propagate = False
    if not any(getattr(handler, "_vibebiz_json", False) for handler in _logger.handlers):
        handler = _StdoutHandler()
        handler._vibebiz_json = True
        handler.setFormatter(JsonFormatter())
        _logger.addHandler(handler)

    log_dir = os.getenv("LOG_DIR", "").strip()
    if log_dir and not any(
        getattr(handler, "_vibebiz_file", False) for handler in _logger.handlers
    ):
        try:
            file_handler = DailyZipFileHandler(log_dir)
            file_handler._vibebiz_file = True
            file_handler.setFormatter(PlainFormatter())
            _logger.addHandler(file_handler)
        except OSError as exc:
            _logger.warning("file logging disabled: %s", exc)

    if workspace_root is not None and workspace_root != _project_root:
        for handler in list(_logger.handlers):
            if getattr(handler, "_vibebiz_project", False):
                _logger.removeHandler(handler)
        project_handler = ProjectLogHandler(workspace_root)
        project_handler._vibebiz_project = True
        _logger.addHandler(project_handler)
        _project_root = workspace_root


def get_logger() -> logging.Logger:
    return _logger


def get_tracer() -> trace.Tracer:
    if _tracer_provider is not None:
        return _tracer_provider.get_tracer(SERVICE_NAME)
    return trace.get_tracer(SERVICE_NAME)


def build_sampler(
    sampler: str | None = None,
    sampler_arg: str | None = None,
) -> Sampler:
    name = (
        (
            sampler
            if sampler is not None
            else os.getenv("OTEL_TRACES_SAMPLER", "parentbased_always_on")
        )
        .strip()
        .lower()
    )
    raw_arg = sampler_arg if sampler_arg is not None else os.getenv("OTEL_TRACES_SAMPLER_ARG", "")

    ratio = 1.0
    if name in ("traceidratio", "parentbased_traceidratio"):
        try:
            ratio = float(raw_arg)
        except (TypeError, ValueError):
            ratio = 1.0

    if name == "always_on":
        return ALWAYS_ON
    if name == "always_off":
        return ALWAYS_OFF
    if name == "traceidratio":
        return TraceIdRatioBased(ratio)
    if name == "parentbased_always_off":
        return ParentBased(root=ALWAYS_OFF)
    if name == "parentbased_traceidratio":
        return ParentBasedTraceIdRatio(ratio)
    if name != "parentbased_always_on":
        _logger.warning("unknown OTEL_TRACES_SAMPLER=%s, using parentbased_always_on", name)
    return ParentBased(root=ALWAYS_ON)


def _build_resource() -> Resource:
    attributes = {"service.name": SERVICE_NAME}
    if not os.getenv("OTEL_RESOURCE_ATTRIBUTES", "").strip():
        attributes["deployment.environment"] = "local"
    return Resource.create(attributes)


def _build_exporter() -> SpanExporter:
    protocol = (
        os.getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
        or os.getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
        or "http/protobuf"
    ).lower()
    if protocol.startswith("grpc"):
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
            OTLPSpanExporter as GrpcOTLPSpanExporter,
        )

        return GrpcOTLPSpanExporter()
    return OTLPSpanExporter()


def _build_provider() -> TracerProvider:
    provider = TracerProvider(resource=_build_resource(), sampler=build_sampler())
    if os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "").strip():
        provider.add_span_processor(BatchSpanProcessor(_build_exporter()))
    return provider


def _extract_project_id(path: str) -> str:
    match = _PROJECT_PATH.match(path)
    return match.group(1) if match else ""


async def _http_metrics_middleware(request: Request, call_next):
    started = time.perf_counter()
    request_id = request.headers.get("x-request-id") or uuid.uuid4().hex
    project_id = _extract_project_id(request.url.path)
    request_token = _request_id.set(request_id)
    project_token = _project_id.set(project_id)

    span = trace.get_current_span()
    if project_id and span.is_recording():
        span.set_attribute("project.id", project_id)
    if span.is_recording():
        span.set_attribute("request.id", request_id)

    status_code = 500
    try:
        response = await call_next(request)
        status_code = response.status_code
        response.headers["X-Request-ID"] = request_id
        return response
    finally:
        duration_ms = (time.perf_counter() - started) * 1000
        route = getattr(request.scope.get("route"), "path", None) or "unmatched"
        method = request.method

        HTTP_REQUESTS.labels(
            service=SERVICE_NAME,
            plane=PLANE,
            route=route,
            method=method,
            status=str(status_code),
            project_id=project_id,
        ).inc()
        HTTP_DURATION.labels(
            service=SERVICE_NAME,
            plane=PLANE,
            route=route,
            method=method,
        ).observe(duration_ms / 1000)
        _logger.info(
            "request.complete",
            extra={
                "route": f"{method} {route}",
                "method": method,
                "status": status_code,
                "duration_ms": round(duration_ms, 2),
            },
        )

        _project_id.reset(project_token)
        _request_id.reset(request_token)


def configure_observability(
    app: FastAPI,
    tracer_provider: TracerProvider | None = None,
    workspace_root: Path | None = None,
) -> None:
    global _tracer_provider

    configure_logging(workspace_root)
    _tracer_provider = tracer_provider or _build_provider()

    app.middleware("http")(_http_metrics_middleware)
    FastAPIInstrumentor.instrument_app(app, tracer_provider=_tracer_provider)

    @app.get("/metrics", include_in_schema=False)
    def metrics() -> Response:
        return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)

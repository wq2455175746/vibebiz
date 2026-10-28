import json
from pathlib import Path

import pytest
from fastapi.testclient import TestClient
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import InMemorySpanExporter
from opentelemetry.sdk.trace.sampling import ALWAYS_OFF, ParentBased
from opentelemetry.trace import SpanKind

from app.config import Settings
from app.main import create_app
from app.observability import _build_provider, build_sampler


def make_client(tmp_path: Path, tracer_provider: TracerProvider | None = None) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings, tracer_provider=tracer_provider))


def init_project(client: TestClient, project_id: str = "p1") -> None:
    response = client.post(f"/internal/projects/{project_id}/init")
    assert response.status_code == 200, response.text


def run_chat(client: TestClient, project_id: str = "p1") -> str:
    with client.stream(
        "POST",
        f"/internal/projects/{project_id}/chat",
        json={"message": "observability"},
    ) as response:
        assert response.status_code == 200
        return "".join(response.iter_text())


def test_metrics_endpoint_exposes_vibebiz_metrics(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    current = client.get("/internal/projects/p1/file", params={"path": "index.html"}).json()[
        "result"
    ]
    client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "<h1>v2</h1>", "baseVersion": current["version"]},
    )
    run_chat(client)

    response = client.get("/metrics")

    assert response.status_code == 200
    assert "text/plain" in response.headers["content-type"]
    for metric in (
        "vibebiz_http_requests_total",
        "vibebiz_http_request_duration_seconds",
        "vibebiz_sandbox_requests_total",
        "vibebiz_project_agent_runs_total",
        "vibebiz_sse_events_total",
    ):
        assert metric in response.text


def test_request_log_is_json_with_required_fields(tmp_path, capsys):
    client = make_client(tmp_path)
    client.get("/internal/projects/p1/file", params={"path": "index.html"})

    records = [
        json.loads(line)
        for line in capsys.readouterr().out.splitlines()
        if line.strip().startswith("{")
    ]
    record = next(item for item in records if item.get("msg") == "request.complete")

    assert record["service"] == "vibebiz-sandbox"
    assert record["plane"] == "sandbox"
    assert record["project_id"] == "p1"
    assert record["route"] == "GET /internal/projects/{project_id}/file"
    assert record["method"] == "GET"
    assert record["status"] == 404
    for field in ("time", "level", "request_id", "trace_id", "span_id", "duration_ms"):
        assert field in record
    assert record["trace_id"]
    assert record["span_id"]


def test_project_scoped_spans(tmp_path):
    exporter = InMemorySpanExporter()
    provider = TracerProvider(resource=Resource.create({"service.name": "vibebiz-sandbox"}))
    provider.add_span_processor(SimpleSpanProcessor(exporter))
    client = make_client(tmp_path, tracer_provider=provider)

    init_project(client)
    current = client.get("/internal/projects/p1/file", params={"path": "index.html"}).json()[
        "result"
    ]
    client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "<h1>v2</h1>", "baseVersion": current["version"]},
    )
    body = run_chat(client)

    assert "data: [DONE]" in body
    finished = exporter.get_finished_spans()
    spans = {span.name: span for span in finished}

    server_span = next(
        span
        for span in finished
        if span.kind is SpanKind.SERVER
        and span.attributes.get("http.route") == "/internal/projects/{project_id}/file"
    )
    assert server_span.attributes["project.id"] == "p1"

    read_span = spans["workspace.read"]
    assert read_span.attributes["project.id"] == "p1"
    assert read_span.attributes["file.path"] == "index.html"
    assert read_span.attributes["file.version"]

    write_span = spans["workspace.write"]
    assert write_span.attributes["project.id"] == "p1"
    assert write_span.attributes["file.path"] == "index.html"

    chat_span = spans["chat.stream"]
    assert chat_span.attributes["project.id"] == "p1"
    assert chat_span.attributes["sse.done"] is True

    agent_span = spans["agent.run"]
    assert agent_span.attributes["project.id"] == "p1"
    assert agent_span.attributes["agent.mode"] == "stub"
    assert agent_span.attributes["agent.status"] == "success"


@pytest.mark.parametrize(
    ("name", "description"),
    [
        ("always_on", "AlwaysOnSampler"),
        ("always_off", "AlwaysOffSampler"),
    ],
)
def test_build_sampler_static_selection(name, description):
    assert build_sampler(name).get_description() == description


def test_build_sampler_defaults_to_parentbased_always_on(monkeypatch):
    monkeypatch.delenv("OTEL_TRACES_SAMPLER", raising=False)
    monkeypatch.delenv("OTEL_TRACES_SAMPLER_ARG", raising=False)

    sampler = build_sampler()

    assert isinstance(sampler, ParentBased)
    assert "root:AlwaysOnSampler" in sampler.get_description()


def test_build_sampler_parentbased_always_off():
    sampler = build_sampler("parentbased_always_off")

    assert isinstance(sampler, ParentBased)
    assert "root:AlwaysOffSampler" in sampler.get_description()


def test_build_sampler_traceidratio_uses_arg():
    sampler = build_sampler("traceidratio", "0.25")

    assert sampler.get_description() == "TraceIdRatioBased{0.25}"


def test_build_sampler_parentbased_traceidratio_uses_env(monkeypatch):
    monkeypatch.setenv("OTEL_TRACES_SAMPLER", "parentbased_traceidratio")
    monkeypatch.setenv("OTEL_TRACES_SAMPLER_ARG", "0.5")

    sampler = build_sampler()

    assert isinstance(sampler, ParentBased)
    assert "root:TraceIdRatioBased{0.5}" in sampler.get_description()


def test_build_sampler_invalid_ratio_defaults_to_one():
    sampler = build_sampler("traceidratio", "not-a-number")

    assert sampler.get_description() == "TraceIdRatioBased{1.0}"


def test_build_provider_passes_sampler(monkeypatch):
    monkeypatch.setenv("OTEL_TRACES_SAMPLER", "always_off")
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)

    provider = _build_provider()

    assert provider.sampler is ALWAYS_OFF


def test_agent_run_logs_project_mode_status_and_trace(tmp_path, capsys):
    client = make_client(tmp_path)
    init_project(client)
    run_chat(client)

    records = [
        json.loads(line)
        for line in capsys.readouterr().out.splitlines()
        if line.strip().startswith("{")
    ]
    start = next(item for item in records if item.get("msg") == "agent.run.start")
    end = next(item for item in records if item.get("msg") == "agent.run.end")

    assert start["project_id"] == "p1"
    assert start["mode"] == "stub"
    assert start["status"] == "running"
    assert end["project_id"] == "p1"
    assert end["mode"] == "stub"
    assert end["status"] == "success"
    for record in (start, end):
        assert record["plane"] == "sandbox"
        assert record["request_id"]
        assert record["trace_id"]
        assert record["span_id"]


def test_agent_failure_logs_error_context(tmp_path, capsys):
    client = make_client(tmp_path)
    init_project(client)

    async def fail(*_args, **_kwargs):
        raise RuntimeError("agent failed")
        yield {}

    client.app.state.agent._run = fail

    body = run_chat(client)
    assert '"type":"error"' in body

    records = [
        json.loads(line)
        for line in capsys.readouterr().out.splitlines()
        if line.strip().startswith("{")
    ]
    agent_error = next(item for item in records if item.get("msg") == "agent.run.error")
    chat_error = next(item for item in records if item.get("msg") == "chat.stream.error")
    end = next(item for item in records if item.get("msg") == "agent.run.end")

    for record in (agent_error, chat_error):
        assert record["project_id"] == "p1"
        assert record["plane"] == "sandbox"
        assert record["code"] == 50000
        assert record["message"] == "agent failed"
        assert record["request_id"]
        assert record["trace_id"]
        assert record["span_id"]
    assert agent_error["mode"] == "stub"
    assert end["status"] == "error"

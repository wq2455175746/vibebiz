import json
from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def init_project(client: TestClient, project_id: str = "p1") -> None:
    response = client.post(f"/internal/projects/{project_id}/init")
    assert response.status_code == 200, response.text


def run_chat(client: TestClient, project_id: str = "p1") -> None:
    with client.stream(
        "POST",
        f"/internal/projects/{project_id}/chat",
        json={"message": "记录项目日志"},
    ) as response:
        assert response.status_code == 200
        assert "data: [DONE]" in "".join(response.iter_text())


def test_project_logs_capture_agent_tools_and_results(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    run_chat(client)

    response = client.get("/internal/projects/p1/logs", params={"limit": 100})

    assert response.status_code == 200, response.text
    logs = response.json()["result"]["logs"]
    assert 0 < len(logs) <= 100
    assert any(item["message"] == "agent.run.start" for item in logs)
    tool = next(item for item in logs if item["source"] == "tool")
    assert tool["message"].startswith("Read")
    assert next(item for item in logs if item["message"] == "agent.run.end")["level"] == "info"
    for item in logs:
        assert set(item) == {"ts", "level", "source", "message"}


def test_project_logs_are_isolated_from_workspace_files(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    log_dir = tmp_path / "workspaces" / "p1" / ".logs"
    log_dir.mkdir()
    (log_dir / "agent_2026-10-30.jsonl").write_text(
        json.dumps(
            {
                "ts": "2026-10-30T00:00:00.000Z",
                "level": "info",
                "source": "agent",
                "message": "secret",
            }
        )
        + "\n",
        encoding="utf-8",
    )

    listed = client.get("/internal/projects/p1/files")
    paths = {item["path"] for item in listed.json()["result"]["files"]}
    assert not any(path.startswith(".logs") for path in paths)

    read = client.get(
        "/internal/projects/p1/file",
        params={"path": ".logs/agent_2026-10-30.jsonl"},
    )
    assert read.status_code == 400


def test_project_logs_returns_latest_100_across_dates(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    log_dir = tmp_path / "workspaces" / "p1" / ".logs"
    log_dir.mkdir()

    for day, prefix in (("2026-10-29", "old"), ("2026-10-30", "new")):
        lines = []
        for index in range(60):
            lines.append(
                json.dumps(
                    {
                        "ts": f"{day}T00:00:{index:02d}.000Z",
                        "level": "info",
                        "source": "agent",
                        "message": f"{prefix}-{index}",
                    }
                )
            )
        (log_dir / f"agent_{day}.jsonl").write_text("\n".join(lines) + "\n", encoding="utf-8")

    response = client.get("/internal/projects/p1/logs", params={"limit": 100})

    assert response.status_code == 200, response.text
    logs = response.json()["result"]["logs"]
    assert len(logs) == 100
    assert logs[0]["message"] == "old-20"
    assert logs[-1]["message"] == "new-59"

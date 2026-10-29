import json
from contextlib import asynccontextmanager
from pathlib import Path
from types import SimpleNamespace

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app
from app.services.chat_turns import TurnLocked


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def init_project(client: TestClient, project_id: str = "p1") -> None:
    response = client.post(f"/internal/projects/{project_id}/init")
    assert response.status_code == 200, response.text


def test_health(tmp_path):
    response = make_client(tmp_path).get("/health")
    assert response.status_code == 200
    assert response.json()["code"] == 0


def test_browser_cors_is_not_enabled(tmp_path):
    client = make_client(tmp_path)
    response = client.get("/health", headers={"Origin": "https://example.com"})

    assert response.status_code == 200
    assert "access-control-allow-origin" not in response.headers


def test_init_and_list_files(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    response = client.get("/internal/projects/p1/files")
    assert response.status_code == 200
    paths = {item["path"] for item in response.json()["result"]["files"]}
    assert {"index.html", "styles.css", "app.js"} <= paths


def test_write_conflict(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    current = client.get("/internal/projects/p1/file", params={"path": "index.html"}).json()[
        "result"
    ]
    first = client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "<h1>v2</h1>", "baseVersion": current["version"]},
    )
    assert first.status_code == 200

    stale = client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "<h1>stale</h1>", "baseVersion": current["version"]},
    )
    assert stale.status_code == 409


def test_preview_serves_static_files(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    page = client.get("/internal/projects/p1/preview/")
    assert page.status_code == 200
    assert "text/html" in page.headers["content-type"]

    css = client.get("/internal/projects/p1/preview/styles.css")
    assert css.status_code == 200
    assert "--blue" in css.text


def test_chat_status_is_idle(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    response = client.get("/internal/projects/p1/chat/status")

    assert response.status_code == 200
    assert response.json()["result"] == {"active": False, "turnId": None}


def test_chat_status_reports_active_turn(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    client.app.state.chat_turns._active["p1"] = SimpleNamespace(id="turn-1")

    response = client.get("/internal/projects/p1/chat/status")

    assert response.status_code == 200
    assert response.json()["result"] == {"active": True, "turnId": "turn-1"}


def test_file_write_is_locked_while_agent_runs(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    @asynccontextmanager
    async def locked(_project_id):
        raise TurnLocked("agent is running")
        yield

    client.app.state.chat_turns.write_guard = locked

    response = client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "<h1>blocked</h1>"},
    )

    assert response.status_code == 423
    assert response.json()["code"] == 42300


def test_invalid_project_id(tmp_path):
    client = make_client(tmp_path)
    response = client.post("/internal/projects/bad id/init")
    assert response.status_code == 400


def test_project_stats_envelope(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    response = client.get("/internal/projects/p1/stats")

    assert response.status_code == 200
    body = response.json()
    assert body["code"] == 0
    assert set(body["result"]) == {"fileSizeBytes", "fileCount", "dirCount", "lastModifiedAt"}
    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert body["result"]["fileCount"] == len(files)
    assert body["result"]["fileCount"] >= 4
    assert body["result"]["dirCount"] == 0
    assert body["result"]["fileSizeBytes"] > 0
    assert body["result"]["lastModifiedAt"] > 0


def test_project_stats_missing_project(tmp_path):
    client = make_client(tmp_path)
    response = client.get("/internal/projects/missing/stats")

    assert response.status_code == 404
    assert response.json()["code"] != 0


def test_project_stats_invalid_project_id(tmp_path):
    client = make_client(tmp_path)
    response = client.get("/internal/projects/bad id/stats")

    assert response.status_code == 400
    assert response.json()["code"] != 0


def test_chat_stream_updates_workspace(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    with client.stream(
        "POST",
        "/internal/projects/p1/chat",
        json={"message": "做一个贪吃蛇小游戏"},
    ) as response:
        assert response.status_code == 200
        body = "".join(response.iter_text())

    assert "data: [DONE]" in body
    assert '"tool_use"' in body

    updated = client.get("/internal/projects/p1/file", params={"path": "index.html"}).json()[
        "result"
    ]
    assert "做一个贪吃蛇小游戏" in updated["content"]


def test_chat_history_is_recorded_and_hidden(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    with client.stream(
        "POST",
        "/internal/projects/p1/chat",
        json={"message": "记录一下这次对话"},
    ) as response:
        assert response.status_code == 200
        body = "".join(response.iter_text())
    assert "data: [DONE]" in body

    history = client.get("/internal/projects/p1/chat/history").json()["result"]
    entries = [json.loads(line) for line in history["content"].splitlines() if line.strip()]
    messages = [entry for entry in entries if entry["type"] == "message"]
    assert [item["message"]["role"] for item in messages] == ["user", "assistant"]
    assert messages[0]["message"]["content"] == "记录一下这次对话"
    assert messages[1]["parentId"] == messages[0]["id"]

    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(not item["path"].startswith(".chathistory") for item in files)
    assert client.get("/internal/projects/p1/preview/.chathistory/session.jsonl").status_code != 200


def test_chat_with_resource_reference_enriches_prompt_and_history(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    headers = {
        "X-Resource-Id": "res-1",
        "X-Resource-Name": "report.pdf",
        "X-Resource-Display-Name": "report.pdf",
        "X-Resource-Storage-Name": "res-1.pdf",
        "X-Resource-Content-Type": "application/pdf",
        "X-Resource-S3-Key": "resources/p1/res-1/res-1.pdf",
        "X-Resource-Kind": "document",
    }
    assert (
        client.put(
            "/internal/projects/p1/resources/res-1", content=b"%PDF", headers=headers
        ).status_code
        == 200
    )

    with client.stream(
        "POST",
        "/internal/projects/p1/chat",
        json={"message": "参考资料改页面", "references": [{"kind": "resource", "id": "res-1"}]},
    ) as response:
        assert response.status_code == 200
        body = "".join(response.iter_text())
    assert "data: [DONE]" in body

    updated = client.get("/internal/projects/p1/file", params={"path": "index.html"}).json()[
        "result"
    ]
    assert ".resources/res-1.pdf" in updated["content"]
    assert "document-skills" in updated["content"]

    history = client.get("/internal/projects/p1/chat/history").json()["result"]
    entries = [json.loads(line) for line in history["content"].splitlines() if line.strip()]
    user = next(entry for entry in entries if entry.get("message", {}).get("role") == "user")
    assert user["message"]["references"] == [
        {
            "kind": "resource",
            "id": "res-1",
            "name": "report.pdf",
            "path": ".resources/res-1.pdf",
            "contentType": "application/pdf",
            "resourceKind": "document",
        }
    ]


def test_chat_rejects_missing_reference(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    response = client.post(
        "/internal/projects/p1/chat",
        json={"message": "看看这个", "references": [{"kind": "resource", "id": "missing"}]},
    )

    assert response.status_code == 400

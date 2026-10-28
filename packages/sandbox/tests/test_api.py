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


def test_invalid_project_id(tmp_path):
    client = make_client(tmp_path)
    response = client.post("/internal/projects/bad id/init")
    assert response.status_code == 400


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

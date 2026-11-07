import io
import json
import tarfile
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


def write_lease(tmp_path: Path, project_id: str, epoch: int, sandbox: str = "sandbox-a") -> None:
    lease = tmp_path / "workspaces" / project_id / ".vibebiz" / "lease"
    lease.parent.mkdir(parents=True, exist_ok=True)
    lease.write_text(json.dumps({"epoch": epoch, "sandbox": sandbox}), encoding="utf-8")


class RecordingAgent:
    mode = "recording"

    def __init__(self) -> None:
        self.calls: list[dict] = []

    async def run(
        self,
        project_id,
        message,
        workspace,
        session_id=None,
        agent_roles=None,
        team_mode=False,
        system_prompt="",
        context="",
        user_id=0,
    ):
        self.calls.append(
            {
                "project_id": project_id,
                "message": message,
                "session_id": session_id,
                "agent_roles": agent_roles,
                "team_mode": team_mode,
                "user_id": user_id,
            }
        )
        yield {"type": "agent", "payload": {"type": "result", "subtype": "success"}}


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


def test_file_write_rejects_stale_epoch(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    response = client.put(
        "/internal/projects/p1/file",
        headers={"X-Project-Epoch": "4"},
        json={"path": "index.html", "content": "new"},
    )

    assert response.status_code == 409
    assert response.json()["code"] == 40902


def test_file_write_accepts_newer_epoch_and_updates_lease(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 4)

    response = client.put(
        "/internal/projects/p1/file",
        headers={"X-Project-Epoch": "5"},
        json={"path": "index.html", "content": "new"},
    )

    assert response.status_code == 200
    lease = json.loads((tmp_path / "workspaces" / "p1" / ".vibebiz" / "lease").read_text())
    assert lease["epoch"] == 5


def test_file_write_without_epoch_still_works(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    response = client.put(
        "/internal/projects/p1/file",
        json={"path": "index.html", "content": "new"},
    )

    assert response.status_code == 200


def test_file_write_returns_busy_when_project_lock_is_held(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    client.app.state.project_lock.timeout = 0.1
    project_dir = client.app.state.workspace.project_dir("p1")

    with client.app.state.project_lock.acquire_sync(project_dir):
        response = client.put(
            "/internal/projects/p1/file",
            json={"path": "index.html", "content": "new"},
        )

    assert response.status_code == 423
    assert response.json()["code"] == 42300


def test_project_init_rejects_stale_epoch(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    response = client.post(
        "/internal/projects/p1/init",
        headers={"X-Project-Epoch": "4"},
    )

    assert response.status_code == 409
    assert response.json()["code"] == 40902


def test_archive_import_rejects_stale_epoch(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    response = client.post(
        "/internal/projects/p1/archive/import",
        content=b"not-a-real-archive",
        headers={"X-Project-Epoch": "4"},
    )

    assert response.status_code == 409
    assert response.json()["code"] == 40902


def test_invalid_epoch_header_is_rejected(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    response = client.put(
        "/internal/projects/p1/file",
        headers={"X-Project-Epoch": "abc"},
        json={"path": "index.html", "content": "new"},
    )

    assert response.status_code == 400


def test_vibebiz_is_hidden_from_files_and_archive(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    project_dir = tmp_path / "workspaces" / "p1"
    (project_dir / ".vibebiz").mkdir(exist_ok=True)
    (project_dir / ".vibebiz" / "lease").write_text("{}", encoding="utf-8")

    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(".vibebiz" not in item["path"] for item in files)
    archive = client.get("/internal/projects/p1/archive")
    assert b".vibebiz" not in archive.content


def test_vibebiz_is_hidden_from_files_stats_archive_and_preview(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(not item["path"].startswith(".vibebiz") for item in files)

    stats = client.get("/internal/projects/p1/stats").json()["result"]
    assert stats["fileCount"] == len([item for item in files if item["type"] == "file"])

    archive = client.get("/internal/projects/p1/archive")
    with tarfile.open(fileobj=io.BytesIO(archive.content), mode="r:gz") as bundle:
        assert all(not name.startswith(".vibebiz") for name in bundle.getnames())

    assert client.get("/internal/projects/p1/preview/.vibebiz/lease").status_code != 200


def test_init_gitignore_excludes_vibebiz(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    ignore = (tmp_path / "workspaces" / "p1" / ".gitignore").read_text(encoding="utf-8")
    assert ".vibebiz/" in ignore


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


def test_chat_rejects_stale_epoch(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    write_lease(tmp_path, "p1", 5)

    response = client.post(
        "/internal/projects/p1/chat",
        headers={"X-Project-Epoch": "4"},
        json={"message": "hello"},
    )

    assert response.status_code == 409
    assert response.json()["code"] == 40902


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
    assert '"type": "usage"' in body or '"type":"usage"' in body
    assert body.index("usage") < body.index("[DONE]")

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


def test_chat_passes_agent_roles_and_team_mode_through_turn_to_agent(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    recorder = RecordingAgent()
    manager = client.app.state.chat_turns
    manager.agent = recorder
    original_start = manager.start
    turns = []

    async def recording_start(*args, **kwargs):
        turn = await original_start(*args, **kwargs)
        turns.append(turn)
        return turn

    manager.start = recording_start
    agent_roles = [
        {
            "id": 1,
            "roleCode": "team",
            "roleNameEn": "Team",
            "roleNameZh": "团队",
            "description": "lead",
            "systemPrompt": "你是团队协调者",
        },
        {
            "id": 2,
            "roleCode": "alex",
            "roleNameEn": "Alex",
            "roleNameZh": "全栈工程师",
            "description": "code",
            "systemPrompt": "你是 Alex",
        },
    ]

    with client.stream(
        "POST",
        "/internal/projects/p1/chat",
        json={"message": "组队实现页面", "agentRoles": agent_roles, "teamMode": True, "userId": 7},
    ) as response:
        assert response.status_code == 200
        body = "".join(response.iter_text())
    assert "data: [DONE]" in body

    assert len(turns) == 1
    assert turns[0].agent_roles == agent_roles
    assert turns[0].team_mode is True

    assert len(recorder.calls) == 1
    assert recorder.calls[0]["user_id"] == 7
    assert recorder.calls[0]["agent_roles"] == agent_roles
    assert recorder.calls[0]["team_mode"] is True


def test_preview_injects_console_bridge_only_into_html(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    page = client.get("/internal/projects/p1/preview/")

    assert page.status_code == 200
    assert "vibebiz-preview-log" in page.text
    assert "preserveDrawingBuffer" in page.text
    assert page.text.index("preserveDrawingBuffer") < page.text.index("<body")

    css = client.get("/internal/projects/p1/preview/styles.css")
    assert css.status_code == 200
    assert "vibebiz-preview-log" not in css.text

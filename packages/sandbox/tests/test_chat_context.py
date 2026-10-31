from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    return TestClient(create_app(Settings(workspaces_dir=tmp_path, agent_mode="stub")))


def test_context_and_system_prompt_are_not_persisted(tmp_path: Path):
    client = make_client(tmp_path)
    assert client.post("/internal/projects/p1/init").status_code == 200
    with client.stream(
        "POST",
        "/internal/projects/p1/chat",
        json={
            "message": "真正任务",
            "systemPrompt": "系统要求",
            "context": "用户：旧消息\n助手：旧回复",
        },
    ) as response:
        list(response.iter_lines())
    history = client.get("/internal/projects/p1/chat/history").json()["result"]["content"]
    assert "真正任务" in history
    assert "系统要求" not in history
    assert "旧消息" not in history

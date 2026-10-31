from pathlib import Path

import httpx
from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(
        workspaces_dir=tmp_path,
        llm_api_key="key",
        llm_base_url="https://llm.test",
        llm_model="model",
    )
    return TestClient(create_app(settings))


def test_parse_task_returns_draft(tmp_path: Path, monkeypatch):
    client = make_client(tmp_path)

    async def fake_post(self, url, headers=None, json=None):
        assert url == "https://llm.test/chat/completions"
        request = httpx.Request("POST", url)
        return httpx.Response(
            200,
            request=request,
            json={
                "choices": [
                    {
                        "message": {
                            "content": '{"name":"检查依赖","description":"检查更新","scheduleType":"daily","time":"09:00","weekdays":[],"cronExpression":"0 9 * * *","timezone":"Asia/Shanghai"}'
                        }
                    }
                ]
            },
        )

    monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
    response = client.post("/internal/tasks/parse", json={"prompt": "每天 9 点检查依赖"})
    assert response.status_code == 200
    body = response.json()
    assert body["code"] == 0
    assert body["result"]["scheduleType"] == "daily"
    assert body["result"]["cronExpression"] == "0 9 * * *"


def test_parse_task_requires_llm_key(tmp_path: Path):
    settings = Settings(workspaces_dir=tmp_path, llm_api_key="")
    client = TestClient(create_app(settings))
    response = client.post("/internal/tasks/parse", json={"prompt": "每天 9 点检查依赖"})
    assert response.status_code == 503

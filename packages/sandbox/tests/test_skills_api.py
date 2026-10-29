from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path, monkeypatch) -> TestClient:
    home = tmp_path / "home"
    monkeypatch.setenv("HOME", str(home))
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def write_skill(root: Path, name: str, description: str) -> None:
    target = root / name
    target.mkdir(parents=True, exist_ok=True)
    (target / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n",
        encoding="utf-8",
    )


def test_lists_project_skills(tmp_path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)
    project = tmp_path / "workspaces" / "p1"
    project.mkdir(parents=True)
    write_skill(project / ".claude" / "skills", "demo", "demo skill")

    response = client.get("/internal/projects/p1/skills")

    assert response.status_code == 200
    assert response.json()["result"]["skills"] == [
        {"name": "demo", "description": "demo skill", "source": "project"}
    ]


def test_missing_project_returns_404(tmp_path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)

    response = client.get("/internal/projects/missing/skills")

    assert response.status_code == 404

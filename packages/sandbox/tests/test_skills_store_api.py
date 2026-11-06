import io
import zipfile
from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def skill_zip(name: str = "demo", description: str = "demo skill") -> bytes:
    buf = io.BytesIO()
    with zipfile.ZipFile(buf, "w") as zf:
        zf.writestr(f"{name}/SKILL.md", f"---\nname: {name}\ndescription: {description}\n---\n")
    return buf.getvalue()


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(
        workspaces_dir=tmp_path / "workspaces",
        skills_root=tmp_path / "skills",
        agent_mode="stub",
    )
    return TestClient(create_app(settings))


def test_upload_list_delete_public(tmp_path: Path):
    client = make_client(tmp_path)
    response = client.post(
        "/internal/skills/upload",
        data={"scope": "public", "overwrite": "false"},
        files={"file": ("demo.zip", skill_zip(), "application/zip")},
    )
    assert response.status_code == 200, response.text
    assert response.json()["result"]["name"] == "demo"

    response = client.get("/internal/skills", params={"scope": "public"})
    assert response.status_code == 200
    assert [item["name"] for item in response.json()["result"]["skills"]] == ["demo"]

    response = client.delete("/internal/skills/public/demo")
    assert response.status_code == 200
    assert (
        client.get("/internal/skills", params={"scope": "public"}).json()["result"]["skills"] == []
    )


def test_user_scope_requires_user_id(tmp_path: Path):
    client = make_client(tmp_path)
    response = client.get("/internal/skills", params={"scope": "user"})
    assert response.status_code == 400

    response = client.post(
        "/internal/skills/upload",
        data={"scope": "user"},
        files={"file": ("demo.zip", skill_zip(), "application/zip")},
    )
    assert response.status_code == 400


def test_user_upload_and_list(tmp_path: Path):
    client = make_client(tmp_path)
    response = client.post(
        "/internal/skills/upload",
        data={"scope": "user", "userId": "7"},
        files={"file": ("mine.zip", skill_zip("mine"), "application/zip")},
    )
    assert response.status_code == 200, response.text

    response = client.get("/internal/skills", params={"scope": "all", "userId": 7})
    assert [item["name"] for item in response.json()["result"]["skills"]] == ["mine"]


def test_project_skills_include_user_and_public(tmp_path: Path):
    client = make_client(tmp_path)
    project = tmp_path / "workspaces" / "p1"
    (project / ".claude" / "skills" / "project-one").mkdir(parents=True)
    (project / ".claude" / "skills" / "project-one" / "SKILL.md").write_text(
        "---\nname: project-one\ndescription: project\n---\n", encoding="utf-8"
    )
    client.post(
        "/internal/skills/upload",
        data={"scope": "public"},
        files={"file": ("pub.zip", skill_zip("pub-one"), "application/zip")},
    )
    client.post(
        "/internal/skills/upload",
        data={"scope": "user", "userId": "7"},
        files={"file": ("mine.zip", skill_zip("mine"), "application/zip")},
    )

    response = client.get("/internal/projects/p1/skills", params={"userId": 7})
    assert response.status_code == 200
    names = {item["name"] for item in response.json()["result"]["skills"]}
    assert names == {"project-one", "pub-one", "mine"}

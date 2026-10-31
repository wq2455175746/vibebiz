import io
import zipfile
from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def zip_bytes(files: dict[str, str]) -> bytes:
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w") as archive:
        for name, content in files.items():
            archive.writestr(name, content)
    return buffer.getvalue()


def test_clone_sync_and_tests_api(tmp_path):
    client = make_client(tmp_path)
    assert client.post("/internal/projects/p1/init").status_code == 200
    payload = zip_bytes({"test_ok.py": "def test_ok():\n    assert True\n"})
    response = client.put(
        "/internal/projects/p1/resources/r1",
        content=payload,
        headers={
            "Content-Type": "application/zip",
            "X-Resource-Name": "tests.zip",
            "X-Resource-Display-Name": "tests.zip",
            "X-Resource-Storage-Name": "r1.zip",
            "X-Resource-Content-Type": "application/zip",
            "X-Resource-Kind": "document",
        },
    )
    assert response.status_code == 200, response.text

    setup = {
        "name": "Acme",
        "description": "clone acme",
        "sourceUrl": "https://acme.example.com",
        "assets": [
            {
                "role": "unit_test",
                "name": "tests.zip",
                "storageName": "r1.zip",
                "contentType": "application/zip",
                "kind": "document",
                "size": len(payload),
            }
        ],
    }
    response = client.put("/internal/projects/p1/clone", json=setup)
    assert response.status_code == 200, response.text
    assert response.json()["result"]["name"] == "Acme"

    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(not item["path"].startswith(".clone") for item in files)

    response = client.post("/internal/projects/p1/clone/tests/run")
    assert response.status_code == 200, response.text
    result = response.json()["result"]
    assert result["status"] == "passed"
    report = client.get("/internal/projects/p1/file", params={"path": "docs/clone/test-report.md"})
    assert report.status_code == 200
    assert "通过" in report.json()["result"]["content"]


def test_clone_sync_rejects_invalid_url(tmp_path):
    client = make_client(tmp_path)
    client.post("/internal/projects/p1/init")
    response = client.put(
        "/internal/projects/p1/clone",
        json={"name": "Acme", "description": "x", "sourceUrl": "ftp://x", "assets": []},
    )
    assert response.status_code == 400

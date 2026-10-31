from pathlib import Path
from urllib.parse import quote_plus

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path / "workspaces", agent_mode="stub")
    return TestClient(create_app(settings))


def init_project(client: TestClient, project_id: str = "p1") -> None:
    response = client.post(f"/internal/projects/{project_id}/init")
    assert response.status_code == 200, response.text


def resource_headers(
    resource_id: str = "res-1", display_name: str = "需求文档.pdf"
) -> dict[str, str]:
    return {
        "X-Resource-Id": resource_id,
        "X-Resource-Name": quote_plus("需求文档.pdf"),
        "X-Resource-Display-Name": quote_plus(display_name),
        "X-Resource-Storage-Name": f"{resource_id}.pdf",
        "X-Resource-Content-Type": "application/pdf",
        "X-Resource-S3-Key": f"resources/p1/{resource_id}/{resource_id}.pdf",
        "X-Resource-Kind": "document",
    }


def test_write_list_delete_resource_and_hide_it_from_files(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    written = client.put(
        "/internal/projects/p1/resources/res-1",
        content=b"%PDF-1.7",
        headers=resource_headers(),
    )
    assert written.status_code == 200, written.text
    assert written.json()["result"]["displayName"] == "需求文档.pdf"

    listing = client.get("/internal/projects/p1/resources").json()["result"]
    assert listing["totalBytes"] == len(b"%PDF-1.7")
    assert listing["resources"][0]["path"] == ".resources/res-1.pdf"

    files = client.get("/internal/projects/p1/files").json()["result"]["files"]
    assert all(not item["path"].startswith(".resources") for item in files)
    assert client.get("/internal/projects/p1/preview/.resources/res-1.pdf").status_code != 200

    deleted = client.delete("/internal/projects/p1/resources/res-1")
    assert deleted.status_code == 200
    assert deleted.json()["result"]["deleted"] is True
    assert client.get("/internal/projects/p1/resources").json()["result"]["resources"] == []


def test_duplicate_display_name_is_rejected(tmp_path):
    client = make_client(tmp_path)
    init_project(client)

    first = client.put(
        "/internal/projects/p1/resources/res-1",
        content=b"one",
        headers=resource_headers("res-1", "same.txt"),
    )
    assert first.status_code == 200

    second = client.put(
        "/internal/projects/p1/resources/res-2",
        content=b"two",
        headers=resource_headers("res-2", "same.txt"),
    )
    assert second.status_code == 409


def test_storage_name_cannot_escape_resources(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    headers = resource_headers()
    headers["X-Resource-Storage-Name"] = "../escape.pdf"

    response = client.put(
        "/internal/projects/p1/resources/res-1",
        content=b"bad",
        headers=headers,
    )

    assert response.status_code == 400


def test_clone_docs_allows_large_file(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    headers = resource_headers("res-clone", "spec.custom")
    headers.update(
        {
            "X-Resource-Storage-Name": "res-clone.custom",
            "X-Resource-Content-Type": "application/octet-stream",
            "X-Resource-Purpose": "clone_docs",
        }
    )
    body = b"x" * (25 * 1024 * 1024)
    response = client.put(
        "/internal/projects/p1/resources/res-clone", content=body, headers=headers
    )
    assert response.status_code == 200, response.text
    assert response.json()["result"]["purpose"] == "clone_docs"


def test_clone_image_rejects_over_ten_megabytes(tmp_path):
    client = make_client(tmp_path)
    init_project(client)
    headers = resource_headers("res-image", "cover.png")
    headers.update(
        {
            "X-Resource-Storage-Name": "res-image.png",
            "X-Resource-Content-Type": "image/png",
            "X-Resource-Kind": "image",
            "X-Resource-Purpose": "clone_image",
        }
    )
    response = client.put(
        "/internal/projects/p1/resources/res-image",
        content=b"x" * (10 * 1024 * 1024 + 1),
        headers=headers,
    )
    assert response.status_code == 413

import asyncio
import ipaddress
from pathlib import Path

import httpx
import pytest

from app.config import Settings
from app.services import web_tools

PUBLIC_IP = ipaddress.ip_address("93.184.216.34")


def make_settings(tmp_path: Path, **kwargs) -> Settings:
    return Settings(workspaces_dir=tmp_path / "workspaces", **kwargs)


def test_host_allowed_supports_exact_and_subdomains():
    domains = ["cdn.example.com", "*.assets.example.com"]
    assert web_tools.host_allowed("cdn.example.com", domains)
    assert web_tools.host_allowed("a.assets.example.com", domains)
    assert not web_tools.host_allowed("evilcdn.example.com", domains)
    assert not web_tools.host_allowed("assets.example.com.evil.test", domains)


def test_validate_fetch_url_accepts_allowlisted_public_host(tmp_path, monkeypatch):
    async def fake_resolve(host: str, port: int):
        return [PUBLIC_IP]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    url = asyncio.run(
        web_tools.validate_fetch_url("https://cdn.example.com/app.css", ["cdn.example.com"])
    )
    assert url == "https://cdn.example.com/app.css"


def test_validate_fetch_url_rejects_unlisted_host(tmp_path):
    with pytest.raises(web_tools.WebToolError):
        asyncio.run(web_tools.validate_fetch_url("https://evil.test/app.css", ["cdn.example.com"]))


def test_validate_fetch_url_rejects_private_address(monkeypatch):
    async def fake_resolve(host: str, port: int):
        return [ipaddress.ip_address("127.0.0.1")]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    with pytest.raises(web_tools.WebToolError):
        asyncio.run(web_tools.validate_fetch_url("http://127.0.0.1/", ["127.0.0.1"]))


def test_validate_fetch_url_rejects_userinfo():
    with pytest.raises(web_tools.WebToolError):
        asyncio.run(
            web_tools.validate_fetch_url(
                "https://user:pass@cdn.example.com/app.css", ["cdn.example.com"]
            )
        )


def test_fetch_url_returns_text(tmp_path, monkeypatch):
    async def fake_resolve(host: str, port: int):
        return [PUBLIC_IP]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    settings = make_settings(tmp_path, web_fetch_allowed_domains="cdn.example.com")

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(
            200, headers={"content-type": "text/css; charset=utf-8"}, text="body{color:red}"
        )

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.fetch_url(
                settings, "https://cdn.example.com/app.css", client=client
            )
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    assert not result.get("is_error")
    text = result["content"][0]["text"]
    assert "body{color:red}" in text
    assert "Content-Type: text/css" in text


def test_fetch_url_truncates_large_bodies(tmp_path, monkeypatch):
    async def fake_resolve(host: str, port: int):
        return [PUBLIC_IP]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    settings = make_settings(
        tmp_path,
        web_fetch_allowed_domains="cdn.example.com",
        web_fetch_max_bytes=5,
    )

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, headers={"content-type": "text/plain"}, text="abcdefgh")

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.fetch_url(
                settings, "https://cdn.example.com/app.css", client=client
            )
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    text = result["content"][0]["text"]
    assert "Bytes: 5 (truncated)" in text
    assert text.endswith("abcde")


def test_fetch_url_rejects_redirect_to_private_address(tmp_path, monkeypatch):
    async def fake_resolve(host: str, port: int):
        if host in {"127.0.0.1", "localhost"}:
            return [ipaddress.ip_address("127.0.0.1")]
        return [PUBLIC_IP]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    settings = make_settings(tmp_path, web_fetch_allowed_domains="cdn.example.com,127.0.0.1")

    def handler(request: httpx.Request) -> httpx.Response:
        if request.url.host == "cdn.example.com":
            return httpx.Response(302, headers={"location": "http://127.0.0.1/"})
        return httpx.Response(200, text="should not be reached")

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.fetch_url(
                settings, "https://cdn.example.com/app.css", client=client
            )
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    assert result.get("is_error")
    assert "private or reserved" in result["content"][0]["text"]


def test_fetch_url_rejects_binary_content(tmp_path, monkeypatch):
    async def fake_resolve(host: str, port: int):
        return [PUBLIC_IP]

    monkeypatch.setattr(web_tools, "resolve_host", fake_resolve)
    settings = make_settings(tmp_path, web_fetch_allowed_domains="cdn.example.com")

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(200, headers={"content-type": "image/png"}, content=b"png")

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.fetch_url(
                settings, "https://cdn.example.com/logo.png", client=client
            )
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    assert result.get("is_error")
    assert "unsupported content type" in result["content"][0]["text"]


def test_search_web_parses_results(tmp_path):
    settings = make_settings(tmp_path, web_search_base_url="http://searxng:8080")

    def handler(request: httpx.Request) -> httpx.Response:
        assert request.url.path == "/search"
        assert request.url.params["q"] == "css grid"
        assert request.url.params["format"] == "json"
        return httpx.Response(
            200,
            json={
                "results": [
                    {
                        "title": "CSS Grid",
                        "url": "https://example.com/grid",
                        "content": "A guide",
                    }
                ]
            },
        )

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.search_web(settings, "css grid", 3, client=client)
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    assert not result.get("is_error")
    text = result["content"][0]["text"]
    assert "CSS Grid" in text
    assert "https://example.com/grid" in text


def test_search_web_reports_http_errors(tmp_path):
    settings = make_settings(tmp_path, web_search_base_url="http://searxng:8080")

    def handler(request: httpx.Request) -> httpx.Response:
        return httpx.Response(503, text="unavailable")

    async def scenario():
        client = httpx.AsyncClient(transport=httpx.MockTransport(handler))
        try:
            return await web_tools.search_web(settings, "css", client=client)
        finally:
            await client.aclose()

    result = asyncio.run(scenario())
    assert result.get("is_error")
    assert "search request failed" in result["content"][0]["text"]


def test_build_web_mcp_server_skips_unconfigured_tools(tmp_path):
    server, names = web_tools.build_web_mcp_server(make_settings(tmp_path))
    assert server is None
    assert names == []


def test_build_web_mcp_server_registers_configured_tools(tmp_path):
    settings = make_settings(
        tmp_path,
        web_search_base_url="http://searxng:8080",
        web_fetch_allowed_domains="cdn.example.com",
    )
    server, names = web_tools.build_web_mcp_server(settings)
    assert server is not None
    assert names == ["mcp__web__web_search", "mcp__web__web_fetch"]

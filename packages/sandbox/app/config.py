import os
import socket
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Settings:
    workspaces_dir: Path
    agent_mode: str = "stub"
    llm_api_key: str = ""
    llm_base_url: str = "https://api.deepseek.com"
    llm_model: str = "deepseek-flash"
    anthropic_base_url: str = ""
    anthropic_auth_token: str = ""
    anthropic_api_key: str = ""
    agent_model: str = "deepseek-flash"
    agent_skills: str = "all"
    agent_plugins: str = ""
    agent_max_turns: int = 40
    chat_turn_timeout_seconds: int = 600
    chat_history_max_bytes: int = 10 * 1024 * 1024
    sandbox_name: str = ""
    advertise_scheme: str = "http"
    advertise_host: str = "127.0.0.1"
    advertise_port: int = 8000
    cp_registry_url: str = ""
    registry_token: str = ""
    heartbeat_interval: float = 5.0
    max_concurrency: int = 8


def _seconds(name: str, default: float) -> float:
    raw = os.getenv(name, "").strip()
    if raw.endswith("s"):
        raw = raw[:-1]
    return float(raw) if raw else default


def load_settings() -> Settings:
    root = Path(os.getenv("WORKSPACES_DIR", "./data/workspaces"))
    advertise_port = int(
        os.getenv("SANDBOX_ADVERTISE_PORT", "") or os.getenv("SANDBOX_PORT", "8000")
    )
    sandbox_name = (
        os.getenv("SANDBOX_NAME", "").strip() or f"{socket.gethostname()}:{advertise_port}"
    )
    return Settings(
        workspaces_dir=root.resolve(),
        agent_mode=os.getenv("AGENT_MODE", "stub").strip().lower(),
        llm_api_key=os.getenv("LLM_API_KEY", ""),
        llm_base_url=os.getenv("LLM_BASE_URL", "https://api.deepseek.com"),
        llm_model=os.getenv("LLM_MODEL_NAME", "deepseek-chat"),
        anthropic_base_url=os.getenv("ANTHROPIC_BASE_URL", "").strip(),
        anthropic_auth_token=os.getenv("ANTHROPIC_AUTH_TOKEN", "").strip(),
        anthropic_api_key=os.getenv("ANTHROPIC_API_KEY", "").strip(),
        agent_model=(
            os.getenv("AGENT_MODEL", "").strip()
            or os.getenv("LLM_MODEL_NAME", "deepseek-flash").strip()
            or "deepseek-flash"
        ),
        agent_skills=os.getenv("AGENT_SKILLS", "all").strip(),
        agent_plugins=os.getenv("AGENT_PLUGINS", "").strip(),
        agent_max_turns=int(os.getenv("AGENT_MAX_TURNS", "40")),
        chat_turn_timeout_seconds=int(os.getenv("CHAT_TURN_TIMEOUT_SECONDS", "600")),
        chat_history_max_bytes=int(os.getenv("CHAT_HISTORY_MAX_BYTES", str(10 * 1024 * 1024))),
        sandbox_name=sandbox_name,
        advertise_scheme=os.getenv("SANDBOX_ADVERTISE_SCHEME", "http").strip() or "http",
        advertise_host=os.getenv("SANDBOX_ADVERTISE_HOST", "127.0.0.1").strip() or "127.0.0.1",
        advertise_port=advertise_port,
        cp_registry_url=os.getenv("CP_REGISTRY_URL", "").strip(),
        registry_token=os.getenv("SANDBOX_REGISTRY_TOKEN", "").strip(),
        heartbeat_interval=max(1.0, _seconds("SANDBOX_HEARTBEAT_INTERVAL", 5.0)),
        max_concurrency=max(1, int(os.getenv("SANDBOX_MAX_CONCURRENCY", "8"))),
    )

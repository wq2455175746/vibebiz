import os
from dataclasses import dataclass
from pathlib import Path


@dataclass(frozen=True)
class Settings:
    workspaces_dir: Path
    agent_mode: str = "stub"
    llm_api_key: str = ""
    llm_base_url: str = "https://api.deepseek.com"
    llm_model: str = "deepseek-chat"
    anthropic_base_url: str = "http://127.0.0.1:4000"
    anthropic_auth_token: str = ""
    agent_model: str = "deepseek-chat"
    agent_skills: str = "all"
    agent_plugins: str = ""
    agent_max_turns: int = 40


def load_settings() -> Settings:
    root = Path(os.getenv("WORKSPACES_DIR", "./data/workspaces"))
    return Settings(
        workspaces_dir=root.resolve(),
        agent_mode=os.getenv("AGENT_MODE", "stub").strip().lower(),
        llm_api_key=os.getenv("LLM_API_KEY", ""),
        llm_base_url=os.getenv("LLM_BASE_URL", "https://api.deepseek.com"),
        llm_model=os.getenv("LLM_MODEL_NAME", "deepseek-chat"),
        anthropic_base_url=os.getenv("ANTHROPIC_BASE_URL", "http://127.0.0.1:4000").strip(),
        anthropic_auth_token=os.getenv("ANTHROPIC_AUTH_TOKEN", "").strip(),
        agent_model=(
            os.getenv("AGENT_MODEL", "").strip()
            or os.getenv("LLM_MODEL_NAME", "deepseek-chat").strip()
            or "deepseek-chat"
        ),
        agent_skills=os.getenv("AGENT_SKILLS", "all").strip(),
        agent_plugins=os.getenv("AGENT_PLUGINS", "").strip(),
        agent_max_turns=int(os.getenv("AGENT_MAX_TURNS", "40")),
    )

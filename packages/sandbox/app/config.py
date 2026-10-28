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


def load_settings() -> Settings:
    root = Path(os.getenv("WORKSPACES_DIR", "./data/workspaces"))
    return Settings(
        workspaces_dir=root.resolve(),
        agent_mode=os.getenv("AGENT_MODE", "stub").strip().lower(),
        llm_api_key=os.getenv("LLM_API_KEY", ""),
        llm_base_url=os.getenv("LLM_BASE_URL", "https://api.deepseek.com"),
        llm_model=os.getenv("LLM_MODEL_NAME", "deepseek-chat"),
    )

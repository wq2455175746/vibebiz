from pathlib import Path

from app.config import Settings
from app.services.skill_catalog import list_skills


def write_skill(root: Path, folder: str, name: str, description: str) -> None:
    target = root / folder
    target.mkdir(parents=True, exist_ok=True)
    (target / "SKILL.md").write_text(
        f"---\nname: {name}\ndescription: {description}\n---\n\n# Body\n",
        encoding="utf-8",
    )


def make_settings(tmp_path: Path, skills: str = "all", plugins: str = "") -> Settings:
    return Settings(
        workspaces_dir=tmp_path / "workspaces",
        agent_skills=skills,
        agent_plugins=plugins,
    )


def test_reads_user_project_and_plugin_skills(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    plugin = tmp_path / "plugin"
    write_skill(home / ".claude" / "skills", "user-one", "user-one", "from user")
    write_skill(project / ".claude" / "skills", "project-one", "project-one", "from project")
    write_skill(plugin / "skills", "plugin-one", "plugin-one", "from plugin")

    items = list_skills(make_settings(tmp_path, plugins=str(plugin)), project, home=home)

    assert [(item["name"], item["source"]) for item in items] == [
        ("plugin-one", "plugin"),
        ("project-one", "project"),
        ("user-one", "user"),
    ]


def test_project_skill_wins_over_user_and_plugin(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    plugin = tmp_path / "plugin"
    write_skill(home / ".claude" / "skills", "same", "same", "user version")
    write_skill(project / ".claude" / "skills", "same", "same", "project version")
    write_skill(plugin / "skills", "same", "same", "plugin version")

    items = list_skills(make_settings(tmp_path, plugins=str(plugin)), project, home=home)

    assert len(items) == 1
    assert items[0]["description"] == "project version"
    assert items[0]["source"] == "project"


def test_filters_by_agent_skills(tmp_path):
    home = tmp_path / "home"
    project = tmp_path / "project"
    write_skill(home / ".claude" / "skills", "one", "one", "first")
    write_skill(home / ".claude" / "skills", "two", "two", "second")

    items = list_skills(make_settings(tmp_path, skills="two"), project, home=home)

    assert [item["name"] for item in items] == ["two"]


def test_none_returns_empty(tmp_path):
    home = tmp_path / "home"
    write_skill(home / ".claude" / "skills", "one", "one", "first")

    assert list_skills(make_settings(tmp_path, skills="none"), tmp_path / "p", home=home) == []


def test_parses_quotes_and_block_description(tmp_path):
    home = tmp_path / "home"
    skill = home / ".claude" / "skills" / "block"
    skill.mkdir(parents=True)
    (skill / "SKILL.md").write_text(
        '---\nname: "block-skill"\ndescription: >\n  line one\n  line two\n---\n',
        encoding="utf-8",
    )

    items = list_skills(make_settings(tmp_path), tmp_path / "p", home=home)

    assert items == [{"name": "block-skill", "description": "line one line two", "source": "user"}]


def test_skips_symlink_escaping_root(tmp_path):
    home = tmp_path / "home"
    outside = tmp_path / "outside"
    write_skill(outside, "escape", "escape", "outside")
    skills = home / ".claude" / "skills"
    skills.mkdir(parents=True)
    (skills / "escape").symlink_to(outside / "escape", target_is_directory=True)

    assert list_skills(make_settings(tmp_path), tmp_path / "p", home=home) == []

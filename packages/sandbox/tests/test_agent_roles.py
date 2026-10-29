from app.services.agent_roles import build_agent_definitions, build_role_prompt


def role(code, name, prompt):
    return {
        "roleCode": code,
        "roleNameEn": name,
        "roleNameZh": name,
        "description": f"{name} description",
        "systemPrompt": prompt,
    }


def test_single_role_prompt_includes_identity_and_description():
    text = build_role_prompt([role("alex", "Alex", "你是 Alex")], False)
    assert "全栈工程师" in text or "Alex" in text
    assert "Alex description" in text
    assert "你是 Alex" in text


def test_team_prompt_includes_team_and_members():
    text = build_role_prompt(
        [role("team", "Team", "你是团队协调者"), role("alex", "Alex", "你是 Alex")],
        True,
    )
    assert "团队协调者" in text
    assert "Alex" in text


def test_multi_role_definitions_include_each_normal_role():
    agents = build_agent_definitions(
        [role("alex", "Alex", "你是 Alex"), role("emma", "Emma", "你是 Emma")],
        False,
    )
    assert set(agents) == {"alex", "emma"}
    assert agents["alex"].prompt == "你是 Alex"
    assert agents["alex"].model == "inherit"
    assert "Agent" not in (agents["alex"].tools or [])


def test_team_definitions_exclude_team_itself():
    agents = build_agent_definitions(
        [role("team", "Team", "lead"), role("alex", "Alex", "你是 Alex")],
        True,
    )
    assert set(agents) == {"alex"}

from typing import Any

from claude_agent_sdk import AgentDefinition

ALLOWED_TOOLS = ["Read", "Write", "Edit", "Bash", "Glob", "Grep"]


def _name(role: dict[str, Any]) -> str:
    zh = str(role.get("roleNameZh") or "").strip()
    en = str(role.get("roleNameEn") or role.get("roleCode") or "").strip()
    return f"{zh}（{en}）" if zh and en else zh or en


def _normal_roles(roles: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [role for role in roles if role.get("roleCode") != "team"]


def _team_role(roles: list[dict[str, Any]]) -> dict[str, Any] | None:
    return next((role for role in roles if role.get("roleCode") == "team"), None)


def build_role_prompt(roles: list[dict[str, Any]], team_mode: bool) -> str:
    if not roles:
        return ""
    team = _team_role(roles)
    members = _normal_roles(roles)
    if team_mode and team is not None:
        lines = [
            "你正在以团队协调者身份工作。",
            str(team.get("systemPrompt") or ""),
            "",
            "团队成员：",
        ]
        lines.extend(f"- {_name(role)}：{role.get('description', '')}" for role in members)
        lines.extend(
            [
                "",
                "先拆解任务和依赖，再调用合适的子 Agent；子 Agent 共享当前项目工作区。",
                "主 Agent 负责检查冲突、整合结果并验证最终交付。",
            ]
        )
        return "\n".join(line for line in lines if line is not None)
    if len(roles) == 1:
        role = roles[0]
        return "\n".join(
            [
                f"当前项目绑定开发者角色：{_name(role)}",
                f"角色描述：{role.get('description', '')}",
                f"角色要求：{role.get('systemPrompt', '')}",
            ]
        )
    lines = [
        "你负责协调以下开发者角色共同完成任务：",
    ]
    for role in members:
        lines.append(f"- {_name(role)}：{role.get('description', '')}")
    lines.extend(
        [
            "",
            "先拆分任务、分析依赖，再按角色调用子 Agent；子 Agent 共享当前项目工作区。",
            "主 Agent 负责检查冲突、整合结果并验证最终交付。",
        ]
    )
    return "\n".join(lines)


def build_agent_definitions(
    roles: list[dict[str, Any]], team_mode: bool
) -> dict[str, AgentDefinition]:
    if not roles or (len(roles) == 1 and not team_mode):
        return {}
    agents: dict[str, AgentDefinition] = {}
    for role in _normal_roles(roles):
        code = str(role.get("roleCode") or "").strip()
        if not code:
            continue
        agents[code] = AgentDefinition(
            description=str(role.get("description") or _name(role)),
            prompt=str(role.get("systemPrompt") or ""),
            tools=list(ALLOWED_TOOLS),
            model="inherit",
        )
    return agents

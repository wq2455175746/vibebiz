# 多角色选择与多 Agent 协作设计

**日期：** 2026-10-29  
**状态：** 待评审  
**范围：** MySQL / CP / DP / sandbox / web  
**不包含：** 用户自定义角色、角色管理后台、跨项目协作、独立 Agent 运行时

## 1. 目标

- 首页创建项目时可用 `#` 选择角色，支持多选。
- 编辑页聊天框可用 `#` 重新设置项目角色，立即持久化，下一条消息生效。
- 新增默认角色 `团队`：选中后由全部启用角色协作完成复杂任务。
- 真实多 Agent 编排：主 Agent 拆分任务，按角色启动子 Agent，汇总结果并交付代码。
- 未选择角色时保持现有单 Agent 行为。
- 角色和协作方式在刷新、重启后仍然有效。

## 2. 非目标

- 不做用户自定义角色或角色管理后台；角色仍由内置 seed 提供。
- 不做跨项目 Agent 协作。
- 不做独立 Agent 进程、远程 Agent 或 MCP Agent 市场。
- 不在第一版展示完整子 Agent 嵌套对话；只展示任务级工具状态。
- 不保证多个子 Agent 同时修改同一文件时的自动冲突合并。
- 不改变现有聊天记录 JSONL 的事实源。

## 3. 当前状态

- `agent_role` 已有 5 个内置角色：`alex`、`bob`、`david`、`emma`、`mike`。
- `project.default_agent_role_id` 已存在，但当前没有读写逻辑。
- 项目创建走 CP，聊天走 DP 直连 sandbox。
- sandbox 当前只把用户消息和资源引用拼成 prompt，没有角色上下文。
- Claude Agent SDK 已支持 `ClaudeAgentOptions.agents` 和 `AgentDefinition`，可通过 Agent 工具启动子 Agent。

## 4. 数据模型

### 4.1 `project_agent_role`

新增项目与角色的多对多关联表：

```sql
CREATE TABLE `project_agent_role` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT,
  `project_id` bigint UNSIGNED NOT NULL COMMENT 'project 内部 ID',
  `agent_role_id` bigint UNSIGNED NOT NULL COMMENT 'agent_role ID',
  `sort_order` int NOT NULL DEFAULT 0,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_agent_role` (`project_id`, `agent_role_id`),
  KEY `idx_project_agent_role_project` (`project_id`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

- 一个项目 0..N 个角色。
- 普通角色最多 5 个；`team` 独占。
- `team` 只保存 `team` 这一行；运行时展开全部启用角色。
- `project.default_agent_role_id` 继续保留并同步为：
  - 普通单角色/多角色：第一个角色 ID。
  - 团队：`team` 角色 ID。
  - 空选择：`NULL`。
- 历史项目没有关联行时，DP 回退读取 `default_agent_role_id`。

### 4.2 `team` seed

新增迁移插入默认数据：

| 字段 | 值 |
|---|---|
| `role_code` | `team` |
| `role_name_en` | `Team` |
| `role_name_zh` | `团队` |
| `description` | 由全部内置角色协作完成复杂任务。 |
| `system_prompt` | 团队协调者提示词，要求拆分任务、分派角色、检查依赖、汇总结果。 |
| `is_builtin` | `1` |
| `sort_order` | `5` |

团队提示词不重复硬编码全部角色提示词；角色成员在运行时从 `agent_role` 启用角色中解析。这样以后新增角色无需改团队 seed。

### 4.3 迁移

- `00012_project_agent_role.sql`：新增关联表。
- `00013_seed_team_role.sql`：插入/更新 `team` 角色。
- 不修改 `00004_seed_topics_roles.sql`，避免已部署数据库无法重放。
- 两个迁移都提供 `-- +goose Up` 和 `-- +goose Down`。

## 5. 角色目录 API

### `GET /cp/api/v1/agent-roles`

登录用户可访问。

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "roles": [
      {
        "id": 6,
        "roleCode": "team",
        "roleNameEn": "Team",
        "roleNameZh": "团队",
        "description": "由全部内置角色协作完成复杂任务。",
        "avatarUrl": ""
      }
    ]
  }
}
```

- 只返回 `status=1` 的角色。
- 按 `sort_order ASC, id ASC` 排序。
- 不返回 `systemPrompt`。
- 团队角色排在普通角色之前。

## 6. 项目角色 API

### `POST /cp/api/v1/projects`

请求增加可选字段：

```json
{
  "prompt": "做一个带任务看板的项目",
  "agentRoleIds": [1, 3]
}
```

- `agentRoleIds` 为空或省略：不绑定角色。
- 普通角色去重后最多 5 个。
- 包含 `team` 时，后端规范化为只保存 `team`。
- 任一角色不存在或未启用：返回 `400`，不能部分保存。

### `PUT /cp/api/v1/projects/:id/agent-roles`

```json
{
  "agentRoleIds": [1, 3]
}
```

- 校验项目属于当前工作区。
- 校验角色存在且启用。
- 事务内删除旧关联、写入新关联、同步 `default_agent_role_id`。
- 空数组表示清除角色，项目回到无角色单 Agent 行为。
- 成功返回规范化后的 `agentRoleIds`。
- 正在运行的 Agent 不受影响；DP 在下一次 `/chat` 重新读取角色。

### `GET /cp/api/v1/projects/:id`

响应增加：

```json
{
  "id": "uuid",
  "defaultAgentRoleId": 1,
  "agentRoleIds": [1, 3]
}
```

## 7. 前端交互

### 7.1 共享状态

- 新增 `useAgentRolesStore`：
  - 缓存启用角色列表。
  - 提供按 ID 查找角色。
  - 首页和编辑页共用，避免重复请求。
- 新增共享 `AgentRoleMenu` 组件：
  - 多选勾选状态。
  - 团队选项独占。
  - 显示角色中文名、英文名、描述。
  - 支持键盘上下、Enter、Esc。
  - 点击外部关闭。

### 7.2 `#` 触发

- 首页和编辑页输入框都监听 `#`。
- 检测到光标前存在未带空格的 `#` token 时打开角色菜单。
- 打开菜单时移除该 `#` token，不把 `#` 留在最终 prompt 中。
- 菜单保持打开，允许连续勾选多个角色。
- 点击角色切换选中状态；团队选中时清空普通角色，普通角色选中时取消团队。
- 菜单关闭后保留角色 chip。
- 角色 chip 可单独移除；移除后触发保存。

### 7.3 首页

- 创建项目时把 `agentRoleIds` 传给 `createProject`。
- 选择角色只生成 chip，不写入 prompt 文本。
- 追加到已有项目时隐藏角色选择，沿用目标项目角色。

### 7.4 编辑页

- 进入项目后显示当前角色 chip。
- 点击 chip 或输入 `#` 可重新选择。
- 每次选择变化调用 `PUT /projects/:id/agent-roles`。
- 保存失败时回滚本地选择并提示错误。
- Agent 运行中仍可修改角色；当前请求不变，下一条消息生效。
- 不提供角色切换的聊天消息，只更新项目设置。

## 8. 聊天角色解析

DP 新增项目角色解析器：

1. 根据项目 public ID 查内部项目 ID。
2. 读取 `project_agent_role`。
3. 如果为空且 `default_agent_role_id` 非空，回退读取该角色。
4. 过滤 `status=1` 的角色。
5. 如果包含 `team`：
   - 展开全部启用的非团队角色。
   - 主角色使用 `team`，成员为全部普通角色。
6. 否则成员为选中的普通角色。

DP 在转发 `/chat` 前把角色写入 payload：

```json
{
  "agentRoles": [
    {
      "id": 1,
      "roleCode": "alex",
      "roleNameEn": "Alex",
      "roleNameZh": "全栈工程师",
      "description": "负责实现页面、组件和交互，直接产出可运行代码。",
      "systemPrompt": "你是 Alex，全栈工程师。..."
    }
  ],
  "teamMode": false
}
```

- DP 覆盖客户端传入的 `agentRoles`，防止伪造角色提示词。
- 角色查询失败记录 warning，并在无角色模式下继续聊天。
- 角色列表为空时保持现有行为。

## 9. Sandbox 编排

### 9.1 单角色

- 把角色描述和 `systemPrompt` 追加到 Claude Code 的 system prompt。
- 不注册子 Agent。
- 行为与现有单 Agent 一致，只是增加角色约束。

### 9.2 多角色

- 主 Agent 使用默认 Claude Code system prompt，并追加协作说明：
  - 当前可用角色和职责。
  - 先拆分任务和依赖，再调用角色子 Agent。
  - 子 Agent 输出需要被主 Agent 验证和整合。
  - 避免重复修改同一文件；有冲突时由主 Agent 决定。
- 通过 `ClaudeAgentOptions.agents` 注册每个普通角色：
  - key：`roleCode`
  - `description`：角色描述
  - `prompt`：角色 `system_prompt`
  - `tools`：现有 `ALLOWED_TOOLS`
  - `model`：继承主模型
- 主 Agent 允许使用 Claude Agent SDK 的 Agent 工具；子 Agent 不再允许 Agent 工具，避免递归委派。
- 子 Agent 共享当前项目工作区，可以直接读文件和写文件。

### 9.3 团队

- `teamMode=true`。
- 主 Agent 使用 `team.system_prompt` 作为协作指令。
- 注册全部启用的普通角色为子 Agent。
- 团队角色本身不作为子 Agent。
- 主 Agent 负责拆解、分派、检查冲突、汇总和最终验证。

### 9.4 流式与历史

- 子 Agent 的 Agent 工具调用通过现有 SSE 工具事件展示。
- 第一版不展示完整子 Agent 文本流，避免聊天内容重复和上下文膨胀。
- 最终聊天记录只保存用户消息和主 Agent 最终回复。
- 子 Agent 的工具结果不写入 `.chathistory`。

## 10. 限制与失败处理

- 普通角色最多 5 个；超过返回 `400`。
- `team` 与普通角色同时提交时，后端规范化为 `team`。
- 主 Agent 最大轮次沿用现有 `AGENT_MAX_TURNS`。
- 整个 turn 沿用现有 600 秒超时；超时后停止主 Agent 和子 Agent。
- 单个子 Agent 失败时，主 Agent 可以重试或改派；不立即中断整体。
- 所有角色解析失败时按无角色模式运行。
- 角色配置存在但全部不可用时，本轮不注入角色并记录 warning。
- Agent 运行中切换角色不改变当前 turn。

## 11. 可观测性

- 日志增加：
  - `project_id`
  - `agent_role_ids`
  - `team_mode`
  - `subagent_role_code`
  - `subagent_status`
- Trace span 增加：
  - `agent.roles.count`
  - `agent.team_mode`
- 不记录角色 `system_prompt` 全文，避免日志膨胀。

## 12. 测试

### 后端

- 迁移：`project_agent_role` 表存在，`team` seed 存在，Up/Down 可执行。
- CP：
  - 角色列表只返回启用角色且不含 `systemPrompt`。
  - 创建项目写入多个角色。
  - 编辑项目替换角色。
  - `team` 规范化。
  - 空数组清除角色。
  - 非当前工作区项目不能修改。
  - 非法/停用角色返回 `400`。
  - `default_agent_role_id` 同步正确。
- DP：
  - 多角色项目聊天 payload 注入 `agentRoles`。
  - 团队展开全部普通角色。
  - 无关联时回退 `default_agent_role_id`。
  - 角色查询失败时聊天继续且无角色字段。
  - 客户端伪造 `agentRoles` 被覆盖。

### Sandbox

- 单角色 system prompt 拼接。
- 多角色生成正确的 `AgentDefinition` 和主 Agent 协作提示。
- 团队模式排除 team 自身，注册全部普通角色。
- 无角色时不设置 `agents`，system prompt 不变。
- 子 Agent 不允许 Agent 工具。
- 子 Agent 失败回退逻辑。

### Web

- 角色菜单多选、团队互斥、chip 移除。
- `#` token 被消费且不进入 prompt。
- 首页创建传递 `agentRoleIds`。
- 编辑页切换调用更新接口，失败回滚。
- 角色列表加载失败显示提示，不影响普通聊天。

## 13. 兼容与上线

- 现有项目没有 `project_agent_role` 行时继续按 `default_agent_role_id` 或无角色运行。
- 现有聊天、资源引用、文件编辑和预览契约不变。
- 先迁移数据库，再部署 CP/DP/sandbox/web。
- 回滚代码后，新表可保留；回滚迁移会删除角色关联和团队 seed。
- 不要求重启已有项目工作区；角色在下次聊天请求时生效。

## 14. 风险

- Claude Agent SDK 的子 Agent 行为依赖当前 SDK 和模型能力，需要用真实模型做一次冒烟验证。
- 多个子 Agent 修改同一工作区可能产生冲突，第一版由主 Agent 协调，不提供自动合并。
- 多 Agent 会增加 token 成本和执行时间，需要轮次和超时限制。
- 团队模式会注册全部角色，后续角色数量增长时需要重新评估上限。

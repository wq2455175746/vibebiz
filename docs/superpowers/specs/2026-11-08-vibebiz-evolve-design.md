# 集思进化计划（Vibebiz Evolve）设计

- 日期：2026-11-08
- 状态：Phase 1 设计已确认，进入前端原型实现
- 关联规则：`.ai/rules/architecture.md`（大任务先设计）、`AGENTS.md`

## 1. 目标

让真实用户以「痛点事件」驱动 Vibebiz 平台自身演进：

1. 用户从首页右下角进入一个保留的特殊项目，看到当前平台版本的交互副本。
2. 用户可以提出 bug、视觉/交互优化、功能建议；Agent 必须先评估、给出理由，不能无脑接受。
3. 每个用户拥有独立版本与聊天记录，互不覆盖；官方版本只读。
4. 运营后台能查看所有用户版本、预览、回复，并把选定版本（文件 + 对话摘要）投递到公司内部自动编码任务队列，经 CI/CD 后随下一版本发布。

## 2. 范围

### Phase 1（本次实现）

- 仅前端 UI 原型：`packages/web` + `packages/ops-web`。
- 假 Agent（规则引擎）、localStorage 持久化、静态 HTML 版 `origin_version`。
- 不接 CP / DP / sandbox，不接真实 LLM，不接 CI/CD。

### Phase 2（后续）

- sandbox 工作区落盘、CP/DP 接口、SSE、真实 Agent、路径安全、审计。
- CI 把真实 web 构建产物写入 `origin_version/`。
- ops-web 编码任务队列对接内部 superagent。

## 3. 关键决策

| 决策 | 选择 |
|---|---|
| 方案 | 独立模块 + 特殊保留项目，不改造主编辑器 |
| 特殊项目 ID | `vibebiz-evolve`（保留，普通项目禁止占用） |
| 用户目录 | `users/<userId>/{current,versions,chat.jsonl,state.json}` |
| origin_version | CI 产物只读；Phase 1 用手写静态 HTML mock |
| web 路由 | `/evolve` |
| ops-web 路由 | `/evolution` |
| 预览 | iframe 加载静态 mock，用 `scenario` 区分版本差异 |
| 分享/发布 | 无 |

## 4. 项目目录结构

```text
data/workspaces/vibebiz-evolve/
  project.json                  # 保留项目元数据，mode=evolve
  origin_version/               # CI 产物，只读
    manifest.json               # version / commit / builtAt
    index.html
    assets/
  users/
    <userId>/
      current/                  # 最新可预览版本（指针或软链）
      versions/<versionId>/     # 不可变快照
      chat.jsonl                # 用户 / Agent / Admin 对话，追加式
      state.json                # currentVersionId / 摘要 / 状态
  taskqueue.jsonl               # 提议队列，追加式
  audit.jsonl                   # 评估、提交、发布审计
```

约束：

- `origin_version` 对用户只读，任何 Agent 不得写入。
- 所有路径先 `resolve()` 再校验 `is_relative_to`。
- `userId` 只取自登录 Session，不信任请求体。
- `taskqueue.jsonl`、`chat.jsonl`、`audit.jsonl` 只追加，不改写历史行。

## 5. 数据契约

### 5.1 提议（taskqueue.jsonl）

```json
{
  "id": "p-0001",
  "createdAt": "2026-11-08T10:00:00.000Z",
  "userId": "1001",
  "userName": "林岚",
  "kind": "bug",
  "content": "首页项目卡片的删除按钮点击没有反应",
  "status": "queued",
  "agentReply": "已定位到该问题，将直接在当前用户版本修复并渲染。",
  "versionId": "v-1001-0001"
}
```

`kind`：`bug` / `visual` / `feature`。
`status`：`queued` / `reviewing` / `accepted` / `rejected` / `released`。

### 5.2 用户版本

```json
{
  "id": "v-1001-0001",
  "userId": "1001",
  "userName": "林岚",
  "label": "v1001.2",
  "createdAt": "2026-11-08T10:00:05.000Z",
  "updatedAt": "2026-11-08T10:00:05.000Z",
  "status": "draft",
  "scenario": "compact-sidebar",
  "summary": "收紧首页项目卡片密度，修复删除按钮无响应。",
  "proposalIds": ["p-0001"]
}
```

`status`：`draft` / `preview` / `accepted` / `submitted` / `released`。
官方基线版本使用 `userId = "official"`、`scenario = "baseline"`、`status = "released"`。

### 5.3 聊天消息（chat.jsonl）

```json
{
  "id": "c-0001",
  "createdAt": "2026-11-08T10:00:00.000Z",
  "role": "user",
  "userName": "林岚",
  "content": "首页卡片太挤了，能不能更紧凑一点？"
}
```

`role`：`user` / `agent` / `admin`。

### 5.4 编码任务（ops-web）

```json
{
  "id": "t-0001",
  "versionId": "v-1001-0001",
  "userId": "1001",
  "userName": "林岚",
  "versionLabel": "v1001.2",
  "originVersion": "v1.2026.11",
  "chatSummary": "用户希望首页卡片更紧凑，已确认预览版本。",
  "status": "queued",
  "createdAt": "2026-11-08T10:10:00.000Z"
}
```

`status`：`queued` / `coding` / `ci` / `released` / `failed`。

## 6. Agent 决策策略

1. **bug**：直接在当前用户版本快照修复，生成新版本并立即渲染。
2. **visual**（视觉 / 交互优化）：先给出判断与理由，生成候选版本渲染，等待用户确认。
3. **feature**（新功能 / 架构改动）：Phase 2B 本地原型中先按页面效果生成候选版本并预览；真实架构与功能实现风险留到后续评估。
4. 用户确认后固定回复：「已收录你的意见版本，评估通过后随下一版本发布。」
5. Agent 不得修改 `origin_version`，不得合并、发布、删除他人版本。
6. 每条回复记录到该用户 `chat.jsonl` 与 `audit.jsonl`。

Phase 1 用关键词规则实现 `bug` / `visual` / `feature` 分类与固定话术；Phase 2 替换为真实 Agent。

## 7. web 页面

路由：`/evolve`，特殊项目 ID：`vibebiz-evolve`。

```text
┌──────────────────────────────────────────────────────────────┐
│ Header：返回 / 集思进化计划 / origin version / 当前用户        │
├───────────────┬───────────────────────────┬──────────────────┤
│ 左：对话       │ 中：预览                   │ 右：版本列表      │
│ 对话/历史/提议 │ iframe + 版本元信息        │ 当前用户置顶      │
│ 输入框         │ 无分享、无发布             │ 其余用户按时间倒序 │
└───────────────┴───────────────────────────┴──────────────────┘
```

交互：

- 首页右下角固定按钮「集思进化计划」进入 `/evolve`。
- 左侧输入框支持「对话」与「提议」两种模式：
  - 对话：追加 `chat.jsonl`，Agent 回复。
  - 提议：追加 `taskqueue.jsonl`，Agent 按策略回复；bug/visual 生成候选版本。
- 右侧点击任意用户版本：中间预览与左侧聊天切换到该用户（非本人只读）。
- 当前用户的最新版本固定置顶。
- 视觉：黑 / 白 / 灰 / 蓝，无分享、无发布。

## 8. ops-web 页面

路由：`/evolution`，导航项「集思进化」。

```text
┌──────────────────────────────────────────────────────────────┐
│ 版本列表（按用户聚合，时间倒序）│ 预览 iframe │ Admin 对话      │
├──────────────────────────────────────────────────────────────┤
│ 自动编码任务队列：queued → coding → ci → released / failed     │
└──────────────────────────────────────────────────────────────┘
```

交互：

- 查看所有用户版本并预览。
- Admin 回复追加到该用户 `chat.jsonl`。
- 选中版本后「提交自动编码任务」，任务进入队列，携带版本、对话摘要、提议 ID。
- 任务只排队，不自动合并、不自动发布。

## 9. 未来接口（Phase 2）

```text
web/CP
  GET  /cp/api/v1/evolve
  GET  /cp/api/v1/evolve/versions
  POST /cp/api/v1/evolve/proposals
  POST /cp/api/v1/evolve/chat

web/DP
  GET  /dp/api/v1/evolve/versions/:id/preview/*
  POST /dp/api/v1/evolve/chat (SSE)

ops-web/CP admin
  GET  /cp/api/v1/admin/evolve/versions
  POST /cp/api/v1/admin/evolve/chat
  POST /cp/api/v1/admin/evolve/tasks
  GET  /cp/api/v1/admin/evolve/tasks
```

Phase 1 前端通过 `api/evolution.ts` 门面隔离，Phase 2 只替换门面实现。

## 10. 测试

- web：提议分类、版本排序、Agent 决策、预览 URL、store 提交提议行为。
- ops-web：版本排序、任务创建与去重、预览 URL、导航与路由表。
- 质量门禁：`npm run test`、`npm run lint`、`npm run build`。

## 11. 非目标

- 不做真实多租户鉴权、SSO、RBAC。
- 不做真实 CI/CD、镜像构建、发布。
- 不做跨用户实时代理与并发写入协调。
- 不把 Phase 1 的 localStorage mock 当作真实数据源。

## 12. Phase 2A 实施范围（2026-11-08 确认）

在 Phase 1 前端原型基础上打通真实后端，但暂不接 CI/CD。

### 存储

- 使用 MySQL 新增 4 张元数据表：`evolution_version`、`evolution_proposal`、`evolution_message`、`evolution_task`。
- 迁移中写入 4 个演示用户及其版本、提议、对话、任务，便于直接查看效果；真实用户数据在其后追加。
- 版本、提议、聊天、任务的 ID 使用字符串公开 ID，前端继续使用 `id` 字段。
- 真实用户版本文件快照、`taskqueue.jsonl` 与 CI 产物写入留到 CI/CD 阶段。

### CP 接口

```text
用户
  GET  /cp/api/v1/evolve
  GET  /cp/api/v1/evolve/conversations/:userId
  POST /cp/api/v1/evolve/proposals
  POST /cp/api/v1/evolve/chat
  POST /cp/api/v1/evolve/versions/:id/confirm

运营
  GET   /cp/api/v1/admin/evolve
  GET   /cp/api/v1/admin/evolve/conversations/:userId
  POST  /cp/api/v1/admin/evolve/versions/:id/reply
  POST  /cp/api/v1/admin/evolve/tasks
  PATCH /cp/api/v1/admin/evolve/tasks/:id
```

- 用户与运营都通过 CP 访问；web 不直连 sandbox，ops-web 不直连 DP。
- 预览继续复用 `origin_version` 静态交互副本，通过版本 `scenario` 参数渲染。
- 编码任务状态由运营后台手动推进：`queued → coding → ci → released / failed`。

### ops-web 交互

- 默认展示「提交变更」表格：用户、版本、变更摘要、提议数、状态、更新时间、操作。
- 最右侧操作列提供：
  - `预览`：打开弹窗，在弹窗内加载 iframe。
  - `添加到编码计划`：创建编码任务，已提交版本禁用。
  - `对话`：打开弹窗查看该用户对话与提议，并以运营身份追加回复。
- 不在页面中默认内嵌预览。
- 底部为编码任务队列表格，展示状态并支持手动推进。

### 暂不做

- CI/CD 自动编码、任务下发与回传。
- 真实版本文件快照、版本 diff、回滚。
- 真实 LLM Agent 与 SSE（继续使用确定性规则引擎）。

## 13. Phase 2B 实施范围（2026-11-08 确认）

- 每个真实用户在 sandbox 使用一个隐藏项目 `evolve-<userId>`，首次进入 `/evolve` 时创建，基于 `origin_version` 静态交互页初始化，不出现在普通项目列表。
- 用户提交 `bug` / `visual` 提议：CP 记录提议和版本；web 通过现有 DP SSE 调用 sandbox Agent 修改 `index.html`，完成后刷新预览。
- 所有提议（`bug` / `visual` / `feature`）都按页面效果生成候选版本，并通过 sandbox Agent 修改预览；真实架构评估留到后续。
- web 预览使用 `/dp/api/v1/projects/evolve-<userId>/preview/`；ops-web 预览使用 `/cp/api/v1/admin/projects/evolve-<userId>/live/`。
- 删除 1001-1004 的 seed 演示版本、提议、对话、任务，只保留 official 基线；由 `00032_remove_evolution_seed.sql` 清理。
- 不做版本文件快照，预览始终对应用户当前 sandbox 状态。
- ops-web 移除「添加到编码计划」按钮和编码计划表格；后端任务接口保留但暂不调用。

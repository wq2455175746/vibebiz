# 超级团队 · 设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-09 |
| 状态 | 待评审 |
| 范围 | 超级团队模块：web 首页、三个演示详情页、ops-web 只读管理；后端/Redis/sandbox/MySQL 仅设计不实现 |
| 关联文档 | `docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`、`docs/superpowers/specs/2026-10-30-enterprise-engineering-design.md`、`.ai/rules/architecture.md` |
| 本轮交付 | web 与 ops-web 前端；三步复用平台项目 / 企业项目 / 定时任务的现有后端逻辑，仅事件源与统计使用 mock / localStorage，不新增 CP API |

> 修订 2026-11-09：需求挖掘复用平台项目，系统开发复用企业级项目，观测运维复用定时任务；删除三个独立演示详情页，改为打开真实项目并在页面内提供「超级团队」上下文条。

---

## 1. 背景与目标

现有平台已有三类能力：标准编辑器、企业工程开发（七阶段）、定时任务。超级团队要把这三类能力串成一条从事件到价值的链路：

```text
飞书 / Webhook / Email / 消息队列
        │
        ▼
   事件入口（用户级配置）
        │
        ▼
   需求挖掘 → 系统开发 → 观测运维
   （严格顺序，人工推进）
```

目标：

1. web 首页新增「超级团队」菜单，内容居中：上方四个事件入口卡片，下方三个大步骤。
2. 事件入口卡片可点击配置，配置属于用户，存 localStorage（本轮 mock）。
3. 三个步骤严格门禁，分别复用编辑器、企业工程开发、定时任务的项目流程效果。
4. ops-web 新增「超级团队」只读页，三 tab：事件源、三步项目、Agent 行为审计。
5. 把事件驱动全链路的后端设计记录在本规格，作为下一轮实现依据。

## 2. 非目标（本轮）

- 不实现 Redis 队列、事件消费者、噪音过滤 / 语义评估、sandbox AI 接口。
- 不新增 CP / DP / sandbox 路由，不改 HTTP / SSE 契约。
- 不建表、不写迁移、不落库；审计数据全部 mock。
- 不接真实飞书、SMTP/IMAP、消息队列 SDK。
- 不引入新 UI 框架、图表库或状态库。
- 不做多租户权限、SSO/RBAC。
- 不在 web / ops-web 引入观测 SDK。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 本轮只做前端，事件源与统计 mock/localStorage | 用户明确先看效果；事件队列与审计后端单独一轮 |
| D2 | 三步分别复用三类真实项目：需求挖掘=平台项目、系统开发=企业项目、观测运维=定时任务；严格门禁、人工推进 | 用户确认；不新增项目类型，直接复用现有创建、阶段、定时任务逻辑 |
| D3 | 事件源属于用户，web 可配置，ops-web 只读 | 用户确认；与现有通知渠道「用户级配置」一致 |
| D4 | 消息队列本轮只占位，禁用 | 用户明确「暂时只占位」 |
| D5 | 超级团队首页走 HomeView 条件渲染；三步打开现有 `editor` / `enterprise-editor` / `scheduled-tasks` 页面 | 复用成熟后端与页面，避免重复实现编辑器 |
| D6 | 步骤通过现有 CP API 创建/打开真实项目与定时任务；事件流与统计仍为前端 mock | 满足「复用后端逻辑」，同时不新增接口 |
| D7 | 不新增图表库；观测图表由定时任务绑定的项目预览/产物提供 | web 当前无图表依赖；保持黑白灰蓝 |
| D8 | ops-web 单导航 + 三 tab | 用户确认；信息集中，避免主导航膨胀 |
| D9 | 超级团队状态、步骤与项目/任务绑定集中 Pinia + localStorage | 便于后续替换为 `superteam_pipeline` 后端存储 |
| D10 | 门禁与绑定校验做成纯函数并测试 | 防止 UI 与状态耦合，后续接后端可复用 |

## 4. 本轮信息架构与路由

### 4.1 web

| 路由 | name | 组件 | 说明 |
|---|---|---|---|
| `/super-team` | `super-team` | HomeView → `SuperTeamPanel` | 保留左侧栏，内容居中；步骤创建/打开真实项目 |
| `/editor/:id?from=super-team&step=requirements` | `editor` | `EditorView` | 平台项目，复用聊天 / 预览 / 代码 / 终端 |
| `/enterprise/editor/:id?from=super-team&step=development` | `enterprise-editor` | `EnterpriseEditorView` | 企业级项目，复用七阶段流程 |
| `/scheduled-tasks?from=super-team&step=operations&taskId=…` | `scheduled-tasks` | `ScheduledTasksPanel` | 定时任务，默认每小时执行绑定项目 |

- 侧栏 `HomeSidebar` 在「企业工程开发」后新增 `超级团队`，图标 `pi pi-users`。
- 不再新增三个演示详情页；`from=super-team` 时在复用页面顶部显示「超级团队上下文条」，提供「返回超级团队」与「完成本阶段」。
- `router.afterEach` 只需新增 `Vibebiz · 超级团队`；复用页面保留原有标题。

### 4.2 ops-web

| 路由 | name | 组件 | 说明 |
|---|---|---|---|
| `/super-team` | `super-team` | `SuperTeamView` | 单页三 tab，只读 |

- `navItems` 在「集思进化」后新增 `超级团队`，图标 `pi pi-users`。
- 现有 `navigation.test.ts` 与 `routes` 同步更新。

## 5. web 数据模型

新增 `packages/web/src/types/superTeam.ts`：

```ts
export type EventSourceKind = 'feishu' | 'webhook' | 'email' | 'queue'
export type EventSourceStatus = 'connected' | 'configured' | 'unconfigured' | 'coming_soon'

export interface EventSourceConfig {
  kind: EventSourceKind
  enabled: boolean
  status: EventSourceStatus
  lastEventAt: string | null
  eventCount: number
  fields: Record<string, string>
}

export type SuperStepKey = 'requirements' | 'development' | 'operations'
export type SuperStepStatus = 'locked' | 'ready' | 'active' | 'done'

export interface SuperStepState {
  key: SuperStepKey
  status: SuperStepStatus
  progress: number
  updatedAt: string | null
  summary: string
}

export type SuperEventStage = 'received' | 'filtered' | 'evaluated' | 'routed' | 'ignored'

export interface SuperEvent {
  id: string
  source: EventSourceKind
  title: string
  receivedAt: string
  noiseScore: number
  semanticScore: number
  stage: SuperEventStage
  routedStep: SuperStepKey | null
}

export type AgentActionKind = 'noise_filter' | 'semantic_eval' | 'artifact' | 'step_advance'

export interface AgentActionAudit {
  id: string
  eventId: string
  step: SuperStepKey
  agent: string
  action: AgentActionKind
  status: 'success' | 'failed' | 'running'
  durationMs: number
  createdAt: string
}

export interface SuperTeamBindings {
  requirementsProjectId: string | null
  developmentProjectId: string | null
  operationsTaskId: number | null
  operationsProjectId: string | null
}

export interface SuperTeamState {
  sources: EventSourceConfig[]
  steps: SuperStepState[]
  events: SuperEvent[]
  audit: AgentActionAudit[]
  bindings: SuperTeamBindings
}
```

绑定约定：

| 步骤 | 绑定字段 | 真实来源 |
|---|---|---|
| 需求挖掘 | `requirementsProjectId` | `POST /cp/api/v1/projects`（默认 `standard`） |
| 系统开发 | `developmentProjectId` | `POST /cp/api/v1/projects` + `mode=enterprise`，并记录企业项目标记 |
| 观测运维 | `operationsTaskId` / `operationsProjectId` | `POST /cp/api/v1/scheduled-tasks`（返回 `id` 与 `projectId`） |

## 6. web 状态与持久化

新增 `packages/web/src/stores/superTeam.ts`（Pinia）：

- state：`sources`、`steps`、`events`、`audit`、`bindings`。
- localStorage key：`vibebiz:super-team`。
- 初始化：读取 localStorage，与默认值合并；字段缺失回退默认。
- actions：
  - `updateSource(kind, patch)`：更新配置，重算 status。
  - `toggleSource(kind)`：启用/停用（`queue` 禁止）。
  - `setBinding(field, value)`：记录真实项目 / 定时任务 ID。
  - `reconcile(projects, tasks)`：校验绑定仍存在；缺失时清空并回退步骤状态。
  - `advance(key)`：门禁校验后完成当前步骤并解锁下一步。
  - `reset()`：恢复默认 demo 数据。
- 本轮不写事件入队逻辑，`events` / `audit` 为种子 mock。

新增 `packages/web/src/utils/superTeam.ts`：

- `defaultSources()`、`defaultSteps()`、`defaultEvents()`、`defaultAudit()`、`defaultBindings()`。
- `initialState()`。
- `loadState(raw)` / `serializeState(state)`：容错合并。
- `isStepUnlocked(steps, key)`。
- `advanceStep(steps, key)`：纯函数，已锁定时返回原数组。
- `reconcileSteps(steps, bindings, projectIds, taskIds)`：删除失效绑定并回退状态。
- `sourceStatusLabel(kind, config)`。
- `stepStatusLabel(status)`。
- `noiseBand(score)`：`low | medium | high`。
- `semanticBand(score)`：`low | medium | high`。

门禁规则：

| 步骤 | 初始状态 | 开始条件 | 完成后 |
|---|---|---|---|
| requirements | `ready`（绑定存在则 `active`） | 无前置 | `development` 变 `ready` |
| development | `locked` | requirements `done` | `operations` 变 `ready` |
| operations | `locked` | development `done` | 全流程完成 |

步骤开始时会调用真实 API 创建项目 / 定时任务；「完成本阶段」只更新超级团队门禁，不改变项目本身状态。

## 7. web 页面设计

### 7.1 超级团队首页 `SuperTeamPanel`

布局：

```text
┌──────────────────────────────────────────────┐
│ 超级团队                                      │
│ 由事件驱动的多 Agent 团队，覆盖需求到运维全流程。 │
│ [事件流概览：今日事件 128 · 有效 36 · 已路由 24] │
├──────────────────────────────────────────────┤
│ [飞书] [Webhook] [Email] [消息队列]            │  4 列卡片
├──────────────────────────────────────────────┤
│ ① 需求挖掘  →  ② 系统开发  →  ③ 观测运维        │
│ [card]          [card]         [card]          │
└──────────────────────────────────────────────┘
```

- 外层 `max-width: 1080px; margin: 0 auto`，上下留白 32px。
- 事件卡片：图标、名称、状态标签、最近事件、事件数、`配置` 按钮；整卡可点击打开配置。
- 消息队列卡片置灰，标注「即将支持」，不可配置。
- 步骤卡片：序号、标题、说明、状态标签、进度条、绑定摘要、主按钮。
  - `locked`：按钮禁用，提示「完成上一步后解锁」。
  - `ready`：按钮「开始」，调用真实 API 创建项目 / 定时任务后跳转。
  - `active`：按钮「继续」，打开已绑定的真实项目 / 定时任务。
  - `done`：按钮「查看」，打开已绑定项目；显示完成时间。
- 步骤与真实对象映射：

| 步骤 | 创建动作 | 打开目标 |
|---|---|---|
| 需求挖掘 | `createProject(prompt, [], undefined)` | `/editor/:id?from=super-team&step=requirements` |
| 系统开发 | `createProject(prompt, [teamRoleId], 'enterprise')` + `markEnterpriseProject` | `/enterprise/editor/:id?from=super-team&step=development` |
| 观测运维 | `createScheduledTask({ cronExpression: '0 * * * *', … })` | `/scheduled-tasks?from=super-team&step=operations&taskId=…` |

- 企业项目创建前先从 `agentRoles.load()` 取 `roleCode=team` 的角色 ID；取不到时提示错误，不降级创建。
- 步骤之间用 CSS 箭头/连接线，移动端纵向排列。

### 7.2 事件源配置抽屉 `EventSourceDialog`

- 使用 PrimeVue `Dialog` 或右侧抽屉样式，宽度 560px。
- 按类型渲染字段：

| 类型 | 字段 |
|---|---|
| 飞书 | App ID、App Secret、Encrypt Key、Verification Token、启用开关 |
| Webhook | 入站地址（只读 mock，形如 `/cp/api/v1/superteam/events/webhook/{token}`）、签名密钥、启用开关 |
| Email | IMAP 服务器、端口、邮箱、授权码、轮询间隔（分钟）、启用开关 |
| 消息队列 | 只读占位说明，禁用保存 |

- 保存后写入 store 并持久化；关闭抽屉后卡片状态即时更新。
- 密码类字段仅本地 mock，不写日志、不打印。
- 校验：启用时必填关键字段；错误用行内文案，不弹全局错误。

### 7.3 需求挖掘：复用平台项目

不新建独立详情页。开始需求挖掘时创建标准项目（`mode=standard`），跳转现有 `EditorView`：

- 左侧聊天、右侧预览 / 代码 / 终端全部走既有 DP 与 sandbox 链路。
- 思维导图、知识图谱、经济可行性文档由 Agent 在项目工作区内生成（例如 markdown / HTML），通过既有预览与代码视图查看；前端不再内置 mock 思维导图与知识图谱。
- 顶部显示 `SuperTeamContextBar`：`超级团队 · 需求挖掘`、`返回超级团队`、`完成本阶段`。

### 7.4 系统开发：复用企业级项目

开始系统开发时创建 `mode=enterprise` 项目，跳转现有 `EnterpriseEditorView`：

- 七个阶段、阶段确认、文档 / 开发 / 制品 / 交付工作区全部复用现有逻辑。
- 事件注入信息由需求挖掘产物与事件源提供；前端不在超级团队页重复实现阶段条。
- 顶部显示 `SuperTeamContextBar`：`超级团队 · 系统开发`、`返回超级团队`、`完成本阶段`。

### 7.5 观测运维：复用定时任务

开始观测运维时创建定时任务（默认 `0 * * * *`，每小时执行），并跳转现有 `ScheduledTasksPanel`：

- 任务列表、暂停 / 恢复 / 停止、打开绑定项目全部复用现有逻辑。
- 从 `taskId` 查询参数选中刚创建的任务。
- 定时生成的指标 / 图表由绑定项目的 Agent 产物提供，通过「打开项目」查看预览。
- 顶部显示 `SuperTeamContextBar`：`超级团队 · 观测运维`、`返回超级团队`、`完成本阶段`。

## 8. ops-web 设计

新增 `packages/ops-web/src/views/SuperTeamView.vue`，单页三 tab：

### 8.1 事件源（只读）

列：归属用户、类型、状态、启用、最近事件、事件数、更新时间。只读，无编辑/删除按钮。

### 8.2 三步项目

列：用户、工作区、需求挖掘、系统开发、观测运维、当前步骤、更新时间。状态用 `tag` 展示。

### 8.3 Agent 行为审计

列：事件 ID、来源、步骤、Agent、动作、噪音分、语义分、状态、耗时、时间。

- 复用 `DataTable`、`TableSearch`、`TablePagination`。
- 数据来自 `packages/ops-web/src/mocks/superTeam.ts`，不访问 CP。
- 新增 `packages/ops-web/src/types/superTeam.ts` 与 `utils/superTeam.ts`：
  - `agentActionLabel(action)`。
  - `superStepLabel(key)`。
  - `sourceKindLabel(kind)`。
  - `formatScore(score, band)`。
- 顶部说明：「只读视图；事件源由用户在 web 端配置。」

## 9. 组件与文件清单

### 9.1 web 新增

```text
packages/web/src/types/superTeam.ts
packages/web/src/utils/superTeam.ts
packages/web/src/utils/superTeam.test.ts
packages/web/src/stores/superTeam.ts
packages/web/src/stores/superTeam.test.ts
packages/web/src/components/superteam/SuperTeamPanel.vue
packages/web/src/components/superteam/EventSourceCard.vue
packages/web/src/components/superteam/EventSourceDialog.vue
packages/web/src/components/superteam/StepCard.vue
packages/web/src/components/superteam/SuperTeamContextBar.vue
packages/web/src/styles/superteam.css
```

### 9.2 web 修改

```text
packages/web/src/components/home/HomeSidebar.vue       # 新增菜单
packages/web/src/views/HomeView.vue                    # 新增 isSuperTeamPage 分支
packages/web/src/router/index.ts                       # 新增 /super-team 路由与标题
packages/web/src/views/EditorView.vue                  # from=super-team 时挂载上下文条
packages/web/src/views/EnterpriseEditorView.vue        # from=super-team 时挂载上下文条
packages/web/src/components/scheduled/ScheduledTasksPanel.vue  # 上下文条 + taskId 选中
```

### 9.3 ops-web 新增

```text
packages/ops-web/src/views/SuperTeamView.vue
packages/ops-web/src/mocks/superTeam.ts
packages/ops-web/src/types/superTeam.ts
packages/ops-web/src/utils/superTeam.ts
packages/ops-web/src/utils/superTeam.test.ts
```

### 9.4 ops-web 修改

```text
packages/ops-web/src/router/navigation.ts
packages/ops-web/src/router/routes.ts
packages/ops-web/src/router/navigation.test.ts
```

## 10. 样式规范

- 仅使用 `--ink`、`--text`、`--muted`、`--line`、`--line-strong`、`--surface`、`--canvas`、`--soft`、`--blue`、`--blue-soft`。
- 卡片圆角 16px，边框 1px `var(--line)`，背景 `var(--surface)`。
- 步骤状态：
  - `locked`：灰底、灰字、禁用。
  - `ready` 主按钮：`--blue`。
  - `active`：蓝边 + 进度条。
  - `done`：灰底 + 勾选图标。
- 响应式：`max-width: 900px` 时事件卡片 2 列、步骤纵向；`max-width: 640px` 时单列。
- 不新增颜色，不引入第三方图标库，沿用 PrimeIcons。

## 11. 测试策略

| 层 | 测试 | 覆盖 |
|---|---|---|
| web utils | vitest | `initialState`、门禁链、锁定步骤不可推进、`reconcileSteps` 失效绑定回退、`noiseBand` / `semanticBand`、`loadState` 容错 |
| web store | vitest | 更新事件源持久化、绑定读写、`advanceStep` 解锁、`queue` 不可启用 |
| web 视图 | 可选 smoke | 路由可渲染、tab 切换（纯函数优先，避免脆测试） |
| ops-web utils | vitest | 标签映射、分数格式化 |
| ops-web 导航 | vitest | `navItems` 含 `super-team`，`childRoutes` 含 `/super-team` |
| 质量门禁 | `make quality` / `make validate` | ESLint、Prettier、调色板、依赖方向 |

## 12. 验收标准

1. 首页侧栏出现「超级团队」，点击进入 `/super-team`，内容居中，含 4 个事件卡片与 3 个步骤卡片。
2. 点击飞书 / Webhook / Email 卡片可配置并保存；刷新后配置保留；消息队列卡片禁用并标注「即将支持」。
3. 三步骤严格门禁：初始仅需求挖掘可用；完成需求挖掘后系统开发解锁；完成后观测运维解锁；锁定步骤无法进入。
4. 需求挖掘创建平台项目并进入 `editor`；系统开发创建企业级项目并进入 `enterprise-editor`；观测运维创建每小时定时任务并在 `scheduled-tasks` 中选中；三个页面顶部均有「超级团队」上下文条，可返回并完成阶段。
5. ops-web 侧栏出现「超级团队」，三 tab 均为只读，展示事件源、三步项目、Agent 行为审计 mock 数据。
6. 所有新增颜色仅黑白灰蓝；不新增前端依赖。
7. `make test`、`make quality`、`make validate` 通过。

## 13. 后续后端设计（本轮不实现）

### 13.1 事件统一信封

```json
{
  "id": "evt_...",
  "source": "feishu|webhook|email|queue",
  "sourceAccountId": "user-event-source-id",
  "userId": 1001,
  "workspaceId": 2001,
  "receivedAt": "2026-11-09T10:00:00Z",
  "title": "客户反馈支付失败",
  "content": "……",
  "raw": {}
}
```

### 13.2 事件源接入

| 类型 | 接入方式 | 参考 |
|---|---|---|
| 飞书 | 官方 SDK WebSocket 长连接，订阅 `im.message.receive_v1`，文本消息转信封 | `guineapig-all-in-one/packages/guineapig-backend/internal/bot/feishu.go` |
| Webhook | CP 暴露 `POST /cp/api/v1/superteam/events/webhook/:token`，直接接收 JSON，签名校验 | 新增 |
| Email | 后台 IMAP 轮询（默认 5 分钟），只解析纯文本与 HTML 正文 | 新增 |
| 消息队列 | 预留 Redis Stream / Kafka 适配器，本轮不实现 | 占位 |

所有入站事件统一收敛到 Redis：

- Stream：`superteam:events`，消费者组 `superteam-worker`。
- 去重：`superteam:event:<id>`（TTL 7 天）。
- 背压：单用户队列长度上限与丢弃策略写入配置。

### 13.3 backend `internal/superteam`

```text
internal/superteam/
  types.go          # EventEnvelope / Evaluation / Pipeline
  ingestor.go       # 飞书 / Webhook / Email 适配
  queue.go          # Redis Stream 生产与消费
  worker.go         # 批量拉取、去重、调用 sandbox、路由
  evaluator.go      # sandbox 批量评估客户端
  router.go         # 语义路由到 requirements/development/operations
  service.go        # 项目创建、阶段推进、审计写入
  repository.go     # GORM 实现
  handler_user.go   # 用户事件源 CRUD、事件列表
  handler_admin.go  # ops-web 只读查询
```

处理流程：

```text
入站事件 → 规范化 → Redis Stream
  → worker 批量拉取（≤20 条）
  → sandbox POST /internal/ai/superteam/evaluate
  → 噪音过滤（noiseScore ≥ 阈值丢弃）
  → 语义评估（semanticScore、suggestedStep、tags）
  → 路由到三步之一：创建/推进项目、触发 Agent
  → 写 superteam_event / superteam_agent_action
```

### 13.4 sandbox AI 批量接口

新增 `packages/sandbox/app/routers/superteam.py`：

```http
POST /internal/ai/superteam/evaluate
```

请求：

```json
{ "events": [{ "id": "evt_1", "source": "feishu", "title": "...", "content": "..." }] }
```

响应：

```json
{
  "results": [
    {
      "id": "evt_1",
      "noiseScore": 12,
      "semanticScore": 88,
      "summary": "支付失败相关问题反馈",
      "suggestedStep": "requirements",
      "tags": ["支付", "可用性"]
    }
  ]
}
```

- 复用现有 LLM 客户端与模型配置，批量处理、超时与重试由 worker 控制。
- 不修改现有 Agent SSE 契约。

### 13.5 数据库表

| 表 | 用途 |
|---|---|
| `superteam_event_source` | 用户级事件源配置（类型、加密凭证、启用、状态） |
| `superteam_event` | 事件信封、噪音分、语义分、阶段、路由步骤 |
| `superteam_pipeline` | 用户/工作区级三步流程实例 |
| `superteam_step` | 每步状态、进度、项目 ID、时间 |
| `superteam_agent_action` | Agent 行为审计：动作、状态、耗时、输入输出摘要 |

- 归属字段：`user_id`、`workspace_id`、`project_id`。
- 凭证加密存储，禁止明文入日志。
- 迁移编号承接现有 goose 版本。

### 13.6 admin API（ops-web）

只读：

```http
GET /cp/api/v1/admin/superteam/event-sources
GET /cp/api/v1/admin/superteam/pipelines
GET /cp/api/v1/admin/superteam/agent-actions
```

用户端（web）：

```http
GET/PUT /cp/api/v1/me/superteam/event-sources
GET     /cp/api/v1/me/superteam/pipeline
POST    /cp/api/v1/me/superteam/steps/:key/advance
```

### 13.7 可观测性

- 日志、Trace、Metrics 带 `user_id`、`workspace_id`、`event_id`、`project_id`。
- 新增指标：`vibebiz_superteam_events_total`、`vibebiz_superteam_noise_filtered_total`、`vibebiz_superteam_agent_actions_total`。
- 不记录事件正文、凭证、聊天内容。

## 14. 风险

| 风险 | 影响 | 处置 |
|---|---|---|
| 本轮纯 mock，后端契约未验证 | 前端状态可能与真实模型偏差 | 类型与纯函数先固定，后续替换 store action 实现 |
| localStorage 多标签不同步 | 配置显示不一致 | 本轮接受；后续接 CP API |
| 事件量过大导致评估成本高 | LLM 费用与延迟 | 后端设计批量、阈值、去重与限流 |
| 飞书长连接稳定性 | 事件丢失 | 复用官方 SDK 自动重连；补心跳与告警 |
| 凭证安全 | 泄露 | 服务端加密存储；前端仅展示脱敏值 |
| 步骤门禁被绕过 | 流程乱序 | 前端纯函数 + 后端二次校验（后续） |

## 15. 需求映射

| 用户要求 | 落地 |
|---|---|
| 主页新增「超级团队」菜单 | `HomeSidebar` 新增导航项 |
| 右侧内容居中，多事件入口卡片 | `SuperTeamPanel` 居中布局 + 4 张事件卡片 |
| 点击卡片可配置 | `EventSourceDialog` + Pinia/localStorage |
| 三个大步骤严格顺序 | `StepCard` + `advanceStep` 门禁纯函数 |
| 需求挖掘复用平台项目 | 创建标准项目并打开 `EditorView`，产物由 Agent 写入工作区 |
| 系统开发复用企业级项目 | 创建 `mode=enterprise` 项目并打开 `EnterpriseEditorView` 七阶段 |
| 观测运维复用定时任务，默认一小时生成可预览图表 | 创建 `0 * * * *` 定时任务，打开 `ScheduledTasksPanel`，图表由绑定项目产物提供 |
| 复用三类项目后端逻辑 | 仅调用现有 `/projects`、`/projects/:id/stages`、`/scheduled-tasks` API |
| 页面业务场景调整 | `SuperTeamContextBar` + `taskId` 选中，不复制编辑器 |
| ops-web 审计 Agent 行为 | `SuperTeamView` 三 tab，含 Agent 行为审计 |
| 事件源属于用户，ops-web 只读 | web 可配置；ops-web 只读 |
| 后端/Redis/sandbox/DB | 第 13 节设计，下一轮实现 |

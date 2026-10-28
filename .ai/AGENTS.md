# Vibebiz AI Harness

## MANDATORY FIRST STEPS

每次任务开始前，按顺序执行：

```bash
cat .ai/CURRENT_FOCUS
cat .ai/rules/architecture.md
cat .ai/rules/sandbox-architecture.md
tail -20 .ai/traces/failures.log
make validate
```

任务结束后：

```bash
make checkpoint
make memory-add     # 有可复用经验时
```

不要跳过 `.ai`；它是跨会话的唯一上下文入口。

## 变更分级（必须）

开始实现前先判断任务大小，并把判断说清楚；不确定时按大任务处理。

### 大任务：必须先设计文档，再开发

满足任一条件即按大任务：

- 新子系统、新页面组、新跨端功能。
- 鉴权、权限、数据隔离、安全边界变化。
- 数据库 schema、DAO、迁移、种子数据变化。
- 接入外部服务（S3、Redis、邮件、支付等）。
- 修改跨 package 的 HTTP / SSE / 事件契约。
- 同时改动 backend / web / ops-web / sandbox 中两个及以上 package。
- 预计改动超过 3 个文件，或需要多 agent 并行。

流程：

1. 读代码与原型，确认边界和验收标准。
2. 写 `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`。
3. 设计确认后写 `docs/superpowers/plans/YYYY-MM-DD-<topic>.md`。
4. 按计划实现；多 agent 时先固定文件所有权与接口契约。
5. 设计文档不能省略；时间紧可以写短，但不能跳过。

### 小修改：直接开发，不写设计文档

满足全部条件才算小修改：

- 单点 bug、文案、样式、配置值、日志、测试修复。
- 不改变 HTTP / SSE 契约、数据库 schema、权限模型、依赖方向。
- 不新增子系统、页面组或外部服务。
- 改动集中在 1-2 个文件。

小修改直接按 TDD / 验证流程处理，不写 spec，不写 plan。

### 边界

- 设计文档是决策记录，不是形式；大任务不写文档视为未完成。
- 设计文档与实现冲突时，先更新文档或明确记录偏离原因。
- 多 agent 并行前必须有设计文档与文件所有权划分，避免互相覆盖。

## Module Routing Map

| 任务类型 | 目标 Package | 技术栈 |
|---|---|---|
| 首页 / 编辑器 / 聊天 / 预览 / 代码视图 | `packages/web` | Vue 3 + Vite + TypeScript + PrimeVue + Pinia + Monaco |
| 运营后台：概览 / 用户 / 项目 / 沙箱 / 配额 / 审计 | `packages/ops-web` | Vue 3 + Vite + TypeScript + PrimeVue + Pinia |
| CP：项目元数据 / admin / 沙箱初始化编排 | `packages/backend/cmd/cp` | Go 1.24 + Echo v4 |
| DP：文件 / SSE / 预览反代 | `packages/backend/cmd/dp` | Go 1.24 + Echo v4 |
| 工作区 / 文件 API / Agent 循环 / 静态预览 | `packages/sandbox` | Python 3.13 + FastAPI + pytest |

依赖方向固定为 `web → CP / DP`，`ops-web → CP`，`CP / DP → sandbox`。禁止 ops-web 访问 DP，禁止两个前端互相访问。

## Backend 内部模块路由

| 路径 | 职责 |
|---|---|
| `cmd/cp/main.go` | CP 进程启动、依赖装配、优雅退出 |
| `cmd/dp/main.go` | DP 进程启动、依赖装配、优雅退出 |
| `internal/config/` | 环境变量配置；按入口设置默认端口 |
| `internal/server/` | Echo 实例、CORS、`/health`、`/metrics` |
| `internal/router/control.go` | CP 路由：`/cp/api/v1/*` 与 `/admin` |
| `internal/router/data.go` | DP 路由：`/dp/api/v1/*`，含 `chat` / `preview` |
| `internal/admin/` | admin `Repository` 接口 + 内存 seed |
| `internal/handler/project.go` | 创建 / 列表 / 查询项目 |
| `internal/handler/admin.go` | 概览 / 用户 / 项目 / 沙箱 / 配额 / 审计 |
| `internal/account/` | 用户资料、工作区、成员、头像预签名编排 |
| `internal/s3/` | S3 presigned PUT/GET + HeadObject |
| `internal/handler/settings.go` | 设置页 API |
| `internal/handler/storage.go` | 当前工作区磁盘统计 |
| `internal/handler/file.go` | 文件树、读、写、409 透传 |
| `internal/handler/chat.go` | SSE 字节流透传，逐行 flush |
| `internal/handler/preview.go` | 预览反代（含 Upgrade 透传） |
| `internal/client/sandbox.go` | 唯一 sandbox HTTP 客户端 |
| `internal/observability/` | JSON 日志、OTel Trace、Prometheus Metrics |
| `internal/store/` | `ProjectStore` 接口 + 内存实现 |
| `internal/model/` | M0 项目投影模型 |
| `internal/dao/` | GORM MySQL 连接 + 泛型 Repository + 20 表 Repositories |
| `internal/dao/model/` | 20 张业务表 model；统一 Base / extra_p / 软删除 |
| `internal/response/` | `{code,message,result}` 信封 |

## Sandbox 内部模块路由

| 路径 | 职责 |
|---|---|
| `app/main.py` | `create_app(settings)` 工厂、异常处理、路由装配 |
| `app/config.py` | 环境配置 |
| `app/routers/projects.py` | 工作区初始化 |
| `app/routers/files.py` | 文件树 / 读 / 写 |
| `app/routers/chat.py` | SSE Agent 流 + 轮次提交 |
| `app/routers/preview.py` | 静态预览 + SPA fallback |
| `app/services/workspace.py` | 路径安全、模板、版本号、原子写入 |
| `app/services/agent.py` | `StubAgent`；后续替换为真实 LLM 循环 |
| `app/services/versioning.py` | 每轮 git commit |
| `app/services/events.py` | SSE 编码与 `[DONE]` |

## Web 内部模块路由

| 路径 | 职责 |
|---|---|
| `src/views/HomeView.vue` | 首页装配：创建项目、最近项目 |
| `src/components/home/HomeSidebar.vue` | 首页左侧栏区域 |
| `src/components/home/HomeHero.vue` | 首页问候与创作输入区域 |
| `src/components/home/HomeDiscovery.vue` | 首页发现 / 模板区域 |
| `src/views/EditorView.vue` | 编辑器骨架：预览 / 代码切换 |
| `src/views/settings/` | 个人信息 / 磁盘空间 / 工作区设置 / 成员 |
| `src/layouts/SettingsLayout.vue` | 设置页左侧菜单布局 |
| `src/components/editor/ChatPanel.vue` | 对话面板占位（M1） |
| `src/components/editor/PreviewPanel.vue` | iframe 预览 |
| `src/components/editor/CodePanel.vue` | 文件列表 + 只读代码 |
| `src/api/client.ts` | axios 实例、信封解包、`ApiError` |
| `src/api/{projects,files,chat,settings,storage,auth}.ts` | 领域 API |
| `src/stores/{project,files,chat}.ts` | Pinia 领域 store |
| `src/utils/sse.ts` | 纯函数 SSE 分帧 |
| `src/monaco/setup.ts` | Monaco worker 装配（M2 启用） |

## ops-web 内部模块路由

| 路径 | 职责 |
|---|---|
| `src/api/client.ts` | axios 实例（`VITE_OPS_API_BASE`）、信封解包、`ApiError` |
| `src/api/admin.ts` | 六个 admin 领域端点 |
| `src/router/routes.ts` | 概览 / 用户 / 项目 / 沙箱 / 配额 / 审计 / 设置 |
| `src/router/navigation.ts` | 侧栏导航配置（纯数据，可测试） |
| `src/layouts/OpsLayout.vue` | 左侧栏 + 顶部 `LOCAL` 标识 |
| `src/views/` | 六个运营页面 + 只读系统设置 |
| `src/components/DataTable.vue` | 加载 / 错误 / 空态表格骨架 |
| `src/stores/ops.ts` | 运营概览状态 |

ops-web 只允许访问 `/cp/api/v1/admin/*`；禁止出现 `/dp/`、sandbox 地址或 `/internal/projects`。

## Architecture Constraints

1. **CP / DP 是哑管道**：不解析 `agent.payload`，只转发字节与 HTTP 状态。CP 不承载文件字节 / SSE / 预览，DP 不承载 admin / 项目元数据。
2. **单一真相源**：代码只在 sandbox 工作区；前端不持有权威副本。
3. **路径安全**：sandbox 所有路径先 `resolve()`，再 `is_relative_to(project_dir)`；禁止字符串前缀判断。
4. **版本并发**：文件 `version = sha256(content)[:12]`；`baseVersion` 不一致返回 HTTP 409。
5. **SSE 契约**：`data: <json>\n\n`，结束 `data: [DONE]\n\n`；网关逐行 flush，禁止缓冲。
6. **预览**：本地 harness 同 origin 仅限开发；生产必须独立子域 + iframe sandbox 白名单。
7. **密钥**：LLM key 只在 sandbox/服务端环境变量；MySQL 凭据只在 CP 服务端；禁止进前端或用户工程。
8. **UI 色彩**：仅黑 / 白 / 灰 / 蓝；新增颜色需产品确认。
9. **当前边界**：不做登录、K8s、CI/CD、OT/CRDT；CP 项目元数据与 admin seed 在内存，重启即丢；目标数据库为 MySQL 8，本轮不建表。
10. **可观测性**：CP / DP / sandbox 输出 stdout JSON、OTLP Trace、`/metrics`；web / ops-web 不接入观测 SDK；项目作用域必须带 `project_id`。
11. **TDD**：先写失败测试再实现；各端测试命令见下。

## Code Templates

`templates/` 提供三端代码与文档骨架，使用 `{{ key }}` 占位符：

```bash
python3 scripts/generate_from_template.py \
  templates/go-handler.go.j2 \
  packages/backend/internal/handler/comment.go \
  --var name=Comment --var method=ListComments --var description="list comments"
```

生成后必须 `gofmt` / `ruff format` / `prettier`，并先补测试。模板清单见 `templates/README.md`。

## Common Task Templates

### 添加 Backend API
1. `internal/model/` 定义模型
2. `internal/store/` 或 `internal/client/` 增加依赖接口
3. `internal/handler/` 实现 handler，用 `response.OK/Fail`
4. 控制类注册到 `internal/router/control.go`（`/cp/api/v1`）；数据类注册到 `internal/router/data.go`（`/dp/api/v1`）
5. `internal/handler/*_test.go` 用 `httptest` + fake 覆盖成功 / 400 / 409 / 502
6. 涉及项目时确保日志、span、指标带 `project_id`

### 添加 Sandbox 能力
1. `app/services/` 实现纯逻辑并注入 `Workspace`
2. `app/routers/` 增加端点，返回统一信封
3. 需要流式时用 `AsyncIterator[dict]` + `StreamingResponse`
4. `tests/` 覆盖路径穿越、版本冲突、SSE 结束标记
5. 若改变 Agent 事件结构，先更新 `.ai/rules/architecture.md`

### 添加前端页面 / 组件
1. `src/views/` 或 `src/components/` 建组件
2. API 统一走对应前端的 `src/api/client.ts`（web 用 `cpHttp` / `dpHttp`，ops-web 用 admin 客户端）
3. 状态按领域进 `src/stores/`
4. 纯函数放 `src/utils/` 并补 vitest
5. 遵守黑白灰蓝；web 不得访问 admin，ops-web 不得访问 DP 或 sandbox

### 接入真实 Agent（M1）
1. 在 `packages/sandbox/app/services/agent.py` 实现与 `StubAgent` 相同的 `run()` 签名
2. 工具调用严格限制在项目工作区内
3. 保持 SSE 事件形状：`assistant` / `tool_use` / `tool_result` / `result`
4. 每轮结束调用 `Versioning.commit`
5. 不修改 backend 的消息解析逻辑

### Docker / 部署
1. 五个镜像：`sandbox` / `cp` / `dp` / `web` / `ops-web`
2. `cp` / `dp` 共用 `packages/backend` 构建上下文，通过 `build.args.SERVICE=cp|dp` 选择入口
3. 所有服务暴露 `/health`；CP / DP / sandbox 暴露 `/metrics`
4. `web` 依赖 `cp` + `dp`，`ops-web` 依赖 `cp`
5. SSE 反代必须关闭 buffering；不交付观测后端容器

## Memory & Rules

| 文件 | 用途 |
|---|---|
| `.ai/CURRENT_FOCUS` | 当前任务焦点与里程碑 |
| `.ai/TODO.md` | 跨里程碑待办 |
| `.ai/memory/lessons.json` | 结构化经验教训 |
| `.ai/memory/anti-patterns.md` | 已知错误模式 |
| `.ai/memory/successful-patterns.md` | 可复用成功模式 |
| `.ai/rules/architecture.md` | 架构规则与契约 |
| `.ai/rules/sandbox-architecture.md` | 多类型 sandbox、项目租约、RWX 工作区与 SSE 扩容目标态 |
| `.ai/rules/go.md` | Go 代码规范（gofmt / vet / 错误 / 测试） |
| `.ai/rules/python.md` | Python 规范（ruff / 路径 / async / 脚本） |
| `.ai/rules/typescript.md` | TS / Vue 规范（strict / Pinia / SSE / 调色板） |
| `.ai/traces/failures.log` | 失败 / 修复记录 |
| `.ai/checkpoints/` | 任务检查点 |

## Harness Auto-Updater

`scripts/harness_autoupdate.py`：

- 从 `.ai/CURRENT_FOCUS` 保存 checkpoint
- 向 `.ai/traces/failures.log` 追加会话事件
- 刷新 `updated_at`
- 每个 `task_id` 只保留最近 3 个 checkpoint

```bash
make harness-update
python3 scripts/harness_autoupdate.py --reason=manual
```

## Commands

```bash
make init
make install
make dev
make test
make quality
make validate
make checkpoint
make memory-add
make docker-build
```

单服务：`make dev-sandbox` / `make dev-cp` / `make dev-dp` / `make dev-web` / `make dev-ops-web`。

每个 package 自带 `Makefile` 与 `.env.example`；优先使用 `make -C packages/<name> <target>`。`make init` 生成各 package 的 `.env`，根目录不再保留统一 `.env`。

## Key Docs

| 文件 | 用途 |
|---|---|
| `README.md` | 快速开始与 M0 边界 |
| `docs/AI-Coding-Platform-技术设计文档.md` | 总体架构与设计决策 |
| `docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md` | CP/DP/ops-web/可观测性规格 |
| `docs/superpowers/specs/2026-10-28-business-database-design.md` | 业务数据库 ER、DDL、索引、种子与迁移顺序 |
| `.ai/rules/architecture.md` | 分层、契约、流程 |
| `.ai/rules/sandbox-architecture.md` | 多类型 sandbox、项目租约、RWX 工作区与 SSE 扩容目标态 |
| `AGENTS.md` | 仓库根行为约束 |

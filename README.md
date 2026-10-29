# vibebiz · AI Coding Platform Harness

对话即开发平台的主流程复刻：用户输入一句话，Agent 在隔离工作区里生成 HTML 小应用 / 小游戏 / 网页，浏览器实时预览并可查看源码。

当前仓库是 **harness（可运行骨架）**，不是最终功能全集。目标是把 CP / DP / sandbox / 两个前端的边界、通信契约、测试与部署骨架先固定下来，再按里程碑逐步填充业务。

## 架构

```
                  ┌──────────────────────────────┐
                  │ packages/web · Vue3 :5173    │
                  │ 控制请求 /cp   数据请求 /dp   │
                  └───────┬──────────────┬───────┘
                          │              │
           /cp/api/v1     │              │  /dp/api/v1
                          ▼              ▼
           ┌──────────────────┐   ┌──────────────────┐
           │ CP · Go :8080    │   │ DP · Go :8081    │
           │ 项目元数据        │   │ 文件树 / 读写     │
           │ admin / 配额占位  │   │ SSE / 预览反代    │
           │ 沙箱初始化编排    │   │ 无项目元数据      │
           └────────┬─────────┘   └────────┬─────────┘
                    │ init / orchestrate   │ files / chat / preview
                    └──────────┬───────────┘
                               ▼
                  ┌──────────────────────────────┐
                  │ packages/sandbox · Python    │
                  │ 工作区 / Agent / 静态预览     │
                  └──────────────────────────────┘

                  ┌──────────────────────────────┐
                  │ packages/ops-web · Vue3 :5174│
                  │ 只访问 /cp/api/v1/admin/*    │
                  └──────────────┬───────────────┘
                                 ▼
                            CP · :8080
```

- **控制面 CP**：项目创建 / 列表 / 详情、运营后台 API、创建项目时调用 sandbox 初始化。
- **数据面 DP**：文件树、文件读写、SSE 对话、预览反代；不持有项目元数据，也不承载 admin。
- **单一真相源**：项目代码只存在于沙箱工作区；聊天、代码、预览都是它的投影。
- **CP / DP 保持哑管道**：只做校验、转发、反代，不解析 Agent 消息。
- **ops-web 只走 CP admin**：不访问 DP 或 sandbox；使用独立 admin 登录，生产必须接入 SSO + RBAC。

## 目录

```
packages/
  web/        Vue3 + Vite + TypeScript + PrimeVue + Pinia + Monaco
  ops-web/    运营后台：Vue3 + Vite + TypeScript + PrimeVue + Pinia
  backend/    Go + Echo：cmd/cp 控制面、cmd/dp 数据面、共享 internal/
  sandbox/    Python + FastAPI：workspace、文件 API、agent、预览
.ai/          AI harness：焦点、规则、记忆、失败记录、检查点
docs/         规格、设计文档与实施计划
templates/    Go / Python / TS / Vue / 文档模板
scripts/      本地开发、校验、质量门禁、harness 自动维护脚本
```

依赖方向：`web → CP / DP`，`ops-web → CP`，`CP / DP → sandbox`。禁止 web / ops-web 直接访问 sandbox，禁止 ops-web 访问 DP。

## 端口

| 服务 | dev | 容器 | 健康检查 | 业务前缀 |
|---|---|---|---|---|
| sandbox | 8000 | 8000 | `/health` | `/internal/*`（仅内网） |
| cp | 8080 | 8080 | `/health` | `/cp/api/v1` |
| dp | 8081 | 8081 | `/health` | `/dp/api/v1` |
| web | 5173 | 80 | `/` | 代理 `/cp` `/dp` |
| ops-web | 5174 | 80 | `/` | 代理 `/cp` |

`/health` 与 `/metrics` 不带业务前缀；业务 API 不使用无前缀的 `/api/v1`。

## AI Harness (.ai)

每次 AI 会话先读：

```bash
cat .ai/AGENTS.md
cat .ai/CURRENT_FOCUS
cat .ai/rules/architecture.md
tail -20 .ai/traces/failures.log
make validate
```

任务结束：

```bash
make checkpoint      # 保存当前焦点快照
make memory-add      # 记录可复用经验
make harness-update  # 刷新焦点、归档检查点（保留最近 3 个）
```

| 文件 | 用途 |
|---|---|
| `.ai/CURRENT_FOCUS` | 当前任务与下一批任务 |
| `.ai/TODO.md` | 待办 |
| `.ai/rules/architecture.md` | 分层、契约、安全、可观测性、测试规则 |
| `.ai/rules/{go,python,typescript}.md` | 三端语言级编码规范 |
| `.ai/memory/` | lessons / anti-patterns / successful-patterns |
| `.ai/traces/failures.log` | 失败与修复记录 |
| `.ai/checkpoints/` | 任务检查点 |

## 代码模板

`templates/` 提供三端骨架与文档模板，统一 `{{ key }}` 占位符：

```bash
python3 scripts/generate_from_template.py \
  templates/go-handler.go.j2 \
  packages/backend/internal/handler/comment.go \
  --var name=Comment --var method=ListComments --var description="list comments"
```

生成后按 `.ai/rules/{go,python,typescript}.md` 格式化并补测试；模板清单见 `templates/README.md`。

## 快速开始

要求：Go 1.24+、Node 20+、Python 3.13+、uv。

本地开发使用固定域名，先配置 hosts（仅首次）：

```text
127.0.0.1 vibebiz-dev.com vibebiz-dev-ops.com
```

登录与会话依赖 MySQL 8 + Redis：先执行 `make db-create && make db-migrate-up`，并确保 Redis 可连接。

```bash
make init
make install
make dev
```

- Web: http://vibebiz-dev.com:5173
- Ops Web: http://vibebiz-dev-ops.com:5174
- CP health: http://vibebiz-dev.com:8080/health
- DP health: http://vibebiz-dev.com:8081/health
- Sandbox health: http://localhost:8000/health

每个 package 自带 `Makefile`、`.env.example` 和 `.dockerignore`；`make init` 生成各自的 `.env`，配置不集中在根目录，`.env` 不会进入镜像。

```bash
make -C packages/backend dev-cp
make -C packages/sandbox dev
make -C packages/web dev
make -C packages/ops-web dev
```

根目录仍保留单服务别名：`make dev-sandbox` / `make dev-cp` / `make dev-dp` / `make dev-web` / `make dev-ops-web`。

`make dev` 通过 `scripts/dev.sh` 分别加载各 package 的 `.env`，启动五个进程；每个进程使用自己的 `OTEL_SERVICE_NAME`（`vibebiz-cp` / `vibebiz-dp` / `vibebiz-sandbox`）与 CORS 来源（`http://vibebiz-dev.com:5173` / `http://vibebiz-dev-ops.com:5174`）。

## 登录与会话

- web 与 ops-web 都需要登录；登录分别使用 `/cp/api/v1/auth/login` 与 `/cp/api/v1/admin/auth/login`。
- 身份保存在 HttpOnly Cookie，服务端 Session 存在 Redis；用户 Session TTL 7 天，ops Session 12 小时。前端不能读取 Session ID。
- Cookie 不设置 Domain：web Cookie 属于 `vibebiz-dev.com`，ops Cookie 属于 `vibebiz-dev-ops.com`。
- 登录与会话依赖 MySQL 8 + Redis；Redis 不可用时 fail closed 返回 503，不做内存降级。
- 默认账号由 `packages/backend/migrations/00005_default_users.sql` seed；默认密码仅用于本地开发，不写入文档，生产必须重置或删除默认账号。
- web 的项目按当前登录用户的工作区过滤；ops-web 的 admin 视角可见全部项目。

## 设置页

- 账户弹窗进入 `/settings/profile`，左侧包含个人信息、磁盘空间、工作区设置、成员。
- 用户头像和工作区头像优先使用 S3 预签名 PUT；存储不支持浏览器 CORS 时自动回退到 CP 代理上传。读取使用预签名 GET。
- 默认存储额度 10GB；磁盘统计来自 sandbox 项目目录扫描。
- 首页左侧工作区下拉切换当前工作区；邀请成员后对方重新登录/刷新可看到共享工作区。
- 首页左侧“文件夹”支持在当前工作区创建一级文件夹；文件夹是工作区下的逻辑目录节点，后续用于归档项目。
- 编辑页采用两栏工作台：左侧任务对话，右侧“应用查看器 / 代码”与可展开执行日志；聊天走真实 DP SSE，日志暂由 SSE 派生，项目/文件/预览走真实 CP/DP API。

## Sandbox Agent SDK

- `AGENT_MODE=stub` 仍使用确定性 StubAgent；`AGENT_MODE=claude` 使用官方 `claude-agent-sdk`。
- DeepSeek 直连：`ANTHROPIC_BASE_URL=https://api.deepseek.com/anthropic` + `ANTHROPIC_AUTH_TOKEN`，模型用 `deepseek-flash`。
- Anthropic 官方：`ANTHROPIC_BASE_URL` 留空 + `ANTHROPIC_API_KEY`。
- LiteLLM 仅用于没有 Anthropic 兼容端点的 OpenAI-only 模型；DeepSeek 不需要。
- `ANTHROPIC_API_KEY` / `ANTHROPIC_AUTH_TOKEN` 只放 sandbox/网关配置，不进入前端或项目目录。
- `AGENT_SKILLS=all` 或逗号分隔技能名；`AGENT_PLUGINS` 为逗号分隔本地插件目录。
- 切换真实模型前先启动 LiteLLM 网关并配置 DeepSeek key；未配置时保持 `AGENT_MODE=stub`。
- `packages/sandbox/litellm.yaml` 仅作为 OpenAI-only 模型的备用适配配置，不是 DeepSeek 必需项。
- 本地 sandbox 冒烟测试步骤见 `docs/sandbox-smoke-test.md`。

## 数据库迁移

MySQL schema 使用版本化 SQL + `goose` 管理；`AutoMigrate` 默认关闭。迁移 DSN 从 `packages/backend/.env` 的 `MYSQL_*` 读取。

```bash
make db-create           # 创建 MYSQL_DATABASE（如不存在）
make db-migrate-up       # 执行所有待执行迁移
make db-migrate-status   # 查看版本状态
make db-migrate-down     # 回滚一个迁移
make db-migrate-version  # 当前版本
make db-migrate-redo     # 回滚并重新执行一个迁移
```

迁移文件在 `packages/backend/migrations/`，执行器为 `packages/backend/cmd/migrate`。

## Docker

```bash
make docker-build
make docker-up     # web :5173 / ops-web :5174 / cp :8080 / dp :8081 / sandbox :8000
make docker-down
```

- `cp` / `dp` 共用 `packages/backend` 构建上下文，通过 `build.args.SERVICE=cp|dp` 选择入口。
- `web` 依赖 `cp` + `dp`，`ops-web` 依赖 `cp`。
- 不在本仓库交付 Prometheus / Jaeger / Grafana 等观测后端。

## 测试

```bash
make test          # backend / sandbox / web / ops-web 全跑
make test-backend  # go test ./...
make test-sandbox  # pytest
make test-web      # vitest
make test-ops-web  # vitest
make quality       # Go / Python / TypeScript 质量门禁（含两个前端）
make validate      # 结构 / 契约 / 分层依赖 / 质量 静态校验
```

逐包执行：

```bash
make -C packages/backend test quality
make -C packages/sandbox test quality
make -C packages/web test quality
make -C packages/ops-web test quality
```

## 当前已打通的最小链路

1. 首页输入 prompt → `POST /cp/api/v1/projects` → CP 创建项目并调用 sandbox 初始化工作区。
2. 跳转编辑器 → 通过 DP 加载文件树与预览 iframe。
3. 发送消息 → `POST /dp/api/v1/projects/{id}/chat` → DP SSE 透传 → sandbox stub agent 修改 `index.html` → 前端刷新预览。
4. 代码视图通过 DP 读取 / 保存文件，带版本号校验（409 冲突）。
5. ops-web 从 `/cp/api/v1/admin/*` 读取概览、用户、项目、沙箱、配额、审计；项目来自 MySQL，其余 admin 数据仍为内存 seed。

Stub agent 只做确定性的演示改写，**不具备真实生成能力**；真实 LLM agent 是下一个里程碑。

## API 契约

统一响应信封：

```json
{ "code": 0, "message": "ok", "result": {} }
```

### CP · `/cp/api/v1`

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/projects` | 项目列表 |
| POST | `/projects` | 创建项目 `{ prompt, name? }` |
| GET | `/projects/:id` | 项目详情 |
| GET | `/workspaces/current/folders` | 当前工作区文件夹列表 |
| POST | `/workspaces/current/folders` | 创建文件夹 `{ name, parentId? }` |
| GET | `/admin/overview` | 运营概览 |
| GET | `/admin/users` | 用户列表 |
| GET | `/admin/projects` | 管理视角项目列表 |
| GET | `/admin/sandboxes` | 沙箱节点列表 |
| GET | `/admin/quotas` | 配额列表 |
| GET | `/admin/audit` | 审计事件列表 |

### DP · `/dp/api/v1`

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/projects/:id/files` | 文件树 |
| GET | `/projects/:id/file?path=` | 读取文件 |
| PUT | `/projects/:id/file` | 写入文件 `{ path, content, baseVersion? }` |
| POST | `/projects/:id/chat` | SSE 对话流 |
| GET | `/projects/:id/preview/*` | 预览反代 |

SSE 事件：

```json
{ "type": "agent", "payload": { "type": "assistant", "message": { "content": [] } } }
{ "type": "sandbox-status", "state": "running" }
data: [DONE]
```

文件读取返回 `content`、`version`（内容 sha256 前 12 位）、`size`、`mtime`、`isBinary`；写入带 `baseVersion` 时版本不一致返回 HTTP 409。

## 可观测性

观测只覆盖 CP / DP / sandbox：

| 服务 | 日志 | Trace | Metrics |
|---|---|---|---|
| CP | stdout JSON | OTLP | `GET /metrics` |
| DP | stdout JSON | OTLP | `GET /metrics` |
| sandbox | stdout JSON | OTLP | `GET /metrics` |
| web / ops-web | 不接入 | 不接入 | 不接入 |

- 项目作用域的日志、span、指标标签必须带 `project_id`。
- 日志禁止记录文件正文、聊天正文、密钥、Authorization。
- `OTEL_EXPORTER_OTLP_ENDPOINT` 为空时关闭 trace 导出且不报错。
- Prometheus 只从三个服务的 `/metrics` 拉取；生产环境不要把该端点暴露到公网。
- 配置项见各 package 的 `.env.example`：`packages/{backend,sandbox,web,ops-web}/.env.example`。

## 数据与存储

- CP 的项目目录已持久化到 MySQL `project` 表；admin 用户/沙箱/配额/审计仍为内存 seed。
- sandbox 工作区在磁盘持久化，是项目代码的唯一真相源。
- 目标数据库为 **MySQL 8**；GORM DAO 与 goose 版本化迁移已加入 `packages/backend`，但尚未接入 `ProjectStore` / `admin.Repository`。
- 数据库凭据只存在于 CP 服务端环境变量，禁止进入前端、日志或仓库。

## 里程碑

- **M0 Harness（当前）**：CP / DP 拆分、ops-web 骨架、可观测性接线、端到端最小链路、Docker、测试脚手架。
- **M1 真实生成**：OpenAI 兼容模型 + 工具调用（读/写/列目录），每轮自动 git commit。
- **M2 编辑器体验**：Monaco 完整接入、文件树 CRUD、409 冲突合并、预览自动刷新。
- **M3 产品化**：登录、多项目、项目 MySQL 持久化、版本列表/diff/回滚、Remix。
- **M4 运行隔离**：每项目容器、卷持久化、预览独立 origin、资源限额。
- **M5 交付**：Compose/K8s、CI/CD、观测后端接入。

## 设计约束

- 页面色彩仅使用黑、白、灰、蓝；新增颜色需产品确认。
- TDD：先写失败测试，再实现。
- CP / DP 不解析 Agent 消息；Agent 能力演进只改 sandbox。
- ops-web 与 admin API 仅用于本地骨架；生产必须接 SSO + RBAC，admin 路由不得裸露公网。
- 预览与主站同 origin 仅限本地 harness；生产必须独立子域。

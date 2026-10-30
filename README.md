# Vibebiz · AI Coding Platform

用一句话描述需求，AI Agent 在隔离工作区中生成网页、小游戏或小应用；企业项目则按需求、设计、测试、开发、制品、交付 7 个阶段推进。编辑器实时展示代码、预览与执行过程。

Vibebiz 是一个可运行的全栈 AI Coding Platform 原型，重点验证：

- 对话驱动的代码生成与实时预览
- 企业工程开发的阶段化协作与人工确认推进
- 控制面 / 数据面 / 执行面的清晰拆分
- 多用户、工作区、项目、会话与运营后台
- MySQL 持久化、Redis 会话与多实例通知
- sandbox 注册、健康检查与按压力调度
- 本地可复现的测试、质量门禁与可观测性

> 当前是可运行的产品原型，不是生产级 SaaS。生产部署前请阅读文末的「安全说明」与「当前限制」。

## 功能特性

- **AI 对话生成**：SSE 流式输出，支持生成中状态、停止生成、工具调用过程与聊天记录回看。
- **代码与预览**：Monaco 代码视图、文件树、文件读写版本校验、iframe 实时预览。
- **企业工程开发**：独立入口、团队角色、7 阶段进度、阶段产物校验、Harness 工程模板、人工确认推进。
- **通知渠道**：个人设置中管理 Webhook、飞书、企微、钉钉机器人和邮箱，支持测试发送。
- **项目与工作区**：多工作区切换、项目归属隔离、文件夹、成员邀请、个人资料与头像、项目封面。
- **登录与会话**：web / ops-web 独立登录，HttpOnly Cookie + Redis Session。
- **运营后台**：概览、用户、项目、沙箱、配额、审计、通知管理。
- **数据持久化**：项目模式、项目阶段、通知渠道、用户、配额、审计、通知、sandbox 实例均落 MySQL。
- **sandbox 注册中心**：心跳上报、asynq 健康复核、最低压力调度、项目粘性绑定与离线改绑。
- **可观测性**：JSON 日志、OTLP Trace、Prometheus `/metrics`，`project_id` 贯穿调用链。
- **质量门禁**：Go / Python / TypeScript 测试、lint、格式检查、分层依赖校验。
- **Kubernetes**：`deployment/helm` 提供 backend / sandbox / web / ops-web 四个 chart 与 umbrella chart。

## 架构

```text
                       ┌─────────────────────────────────────┐
                       │ web :5173        ops-web :5174       │
                       │ 标准/企业编辑器   运营后台             │
                       └──────────┬───────────────┬──────────┘
                                  │               │
                         /cp/api/v1│               │/cp/api/v1/admin
                                  ▼               ▼
                       ┌─────────────────────────────────────┐
                       │ CP · Go :8080                       │
                       │ 项目 / 企业阶段 / Harness / 鉴权     │
                       │ admin / sandbox 注册                 │
                       └──────┬───────────────┬──────────────┘
                              │               │
                    /dp/api/v1│               │MySQL / Redis
                              ▼               ▼
                       ┌──────────────┐  ┌──────────────────┐
                       │ DP · Go :8081│  │ MySQL 8          │
                       │ 文件 / SSE   │  │ Redis            │
                       │ 日志 / 预览  │  └──────────────────┘
                       └──────┬───────┘
                              │ internal HTTP / SSE
                              ▼
              ┌──────────────────────────────────────────┐
              │ sandbox · Python + FastAPI :8000         │
              │ workspace / Agent / 文件 API / 静态预览   │
              │ 无状态计算，工作区位于共享 NFS/RWX        │
              └──────────────────────────────────────────┘
```

### 服务职责

| 服务 | 职责 | 不负责 |
|---|---|---|
| CP | 项目元数据与模式、团队角色校验、Harness 导入、阶段状态、个人通知渠道、登录与鉴权、运营后台、sandbox 注册与调度 | 文件字节、SSE、预览反代 |
| DP | 文件树、文件读写、聊天 SSE、项目日志、封面上传、预览反代 | 项目列表、admin、项目创建 |
| sandbox | 工作区、Agent 执行、文件 API、静态预览、并发上报 | 访问 CP/DP 内部实现或数据库 |
| web | 首页、标准编辑器、企业工程开发、聊天、预览、代码视图 | 直接访问 sandbox |
| ops-web | 运营后台，只访问 CP admin API | 访问 DP 或 sandbox |

依赖方向固定为：

```text
web → CP / DP
ops-web → CP
CP / DP → sandbox
```

## 企业工程开发

企业工程开发与标准项目共用项目、会话和工作区，但使用独立入口和独立编辑页：

```text
/enterprise              企业工程开发首页
/enterprise/editor/:id   企业工程编辑页
/editor/:id              标准项目编辑页（保持原样）
```

规则：

- 创建企业项目必须选择 `team` 团队角色；后端会展开为 `team + emma + bob + senior + qa + alex`，保证团队拥有实际子 Agent。
- 7 个阶段：需求梳理、概要设计、详细设计、测试用例、全栈开发、制品空间、交付空间。
- 当前版本开放前 5 个阶段；制品空间和交付空间为锁定空态。
- 阶段状态存 MySQL `project_stage`，标准项目不创建阶段。
- 每个阶段有固定角色和固定产物：
  - 需求：`docs/requirements.md`
  - 概要设计：`docs/architecture.md`
  - 详细设计：`docs/detailed-design.md`
  - 测试用例：`docs/test-plan.md`
  - 开发：`docs/development.md` + 代码
- 阶段必须人工确认推进；后端校验阶段顺序、是否已开始、Agent 是否空闲、产物是否存在且非空。
- 企业项目创建时从 S3 读取 Harness zip，校验后转成 sandbox 支持的 tar.gz 导入。标准项目不导入 Harness。

## 通知渠道

通知渠道是**个人级配置**，从「设置 → 通知渠道」进入：

- 支持 Webhook、飞书机器人、企微机器人、钉钉机器人、邮箱。
- 同一用户可配置多个渠道，渠道名唯一。
- 配置使用 AES-GCM 加密后存入 `notification_channel`，API 不回显加密字段。
- 可发送测试消息；测试结果记录 `last_test_at/status/error`。
- Webhook 只允许 HTTPS，并阻止私网、回环、链路本地和云元数据地址。
- 邮箱使用平台全局 SMTP；SMTP 未配置时保存成功，但测试发送返回 `503`。
- 当前只做配置与测试发送，系统通知尚未按渠道自动投递。

## 技术栈

- **前端**：Vue 3、Vite、TypeScript、Pinia、PrimeVue、Monaco Editor
- **后端**：Go 1.24、Echo、GORM、MySQL 8、Redis、asynq、goose
- **Sandbox**：Python 3.13、FastAPI、Uvicorn、Claude Agent SDK
- **观测**：slog JSON、OpenTelemetry OTLP、Prometheus
- **测试**：Go `testing` + `httptest` + `sqlmock`、pytest、vitest

## 快速开始

### 环境要求

- Go 1.24+
- Node.js 20+
- Python 3.13+ 与 `uv`
- MySQL 8
- Redis 6+

### 本地域名

本地开发使用固定域名，首次运行前配置 hosts：

```text
127.0.0.1 vibebiz-dev.com vibebiz-dev-ops.com
```

### 初始化

```bash
make init
make install
```

`make init` 会为各 package 生成 `.env`。按需修改：

- `packages/backend/.env`：MySQL、Redis、S3、sandbox 注册 token、企业 Harness S3 key
- `packages/sandbox/.env`：工作区目录、Agent 模型、CP 注册地址
- `packages/web/.env`、`packages/ops-web/.env`：前端 API 地址与演示登录开关

所有 `.env` 均已被 `.gitignore` 忽略，不要提交真实密钥。

### 初始化数据库

```bash
make db-create
make db-migrate-up
```

当前迁移到 v19，覆盖身份/工作区、项目/资源、AI/聊天、种子数据、设置、文件夹、通知、项目持久化、sandbox 注册、资料、角色、团队角色、项目封面、模板审核、企业项目模式、企业角色、项目阶段与个人通知渠道。

### 启动

```bash
make dev
```

| 入口 | 地址 |
|---|---|
| Web | http://vibebiz-dev.com:5173 |
| Ops Web | http://vibebiz-dev-ops.com:5174 |
| CP health | http://vibebiz-dev.com:8080/health |
| DP health | http://vibebiz-dev.com:8081/health |
| Sandbox health | http://localhost:8000/health |

也可以单独启动：

```bash
make dev-sandbox
make dev-cp
make dev-dp
make dev-web
make dev-ops-web
```

## 默认测试账号

仅用于本地演示与测试：

| 入口 | 账号 | 密码 |
|---|---|---|
| Web | `wanghg11` | `88888888` |
| Web | `vistor` | `88888888` |
| Ops Web | `admin` | `88888888` |

- web 登录页默认填入 `wanghg11 / 88888888`
- ops-web 登录页默认填入 `admin / 88888888`
- 设置 `VITE_DEMO_LOGIN=false` 可关闭登录页预填
- 生产环境必须删除默认账号或重置密码，并关闭演示登录

## 常用命令

```bash
make dev            # 启动五个服务
make test           # backend / sandbox / web / ops-web 全量测试
make quality        # Go / Python / TS / Vue 质量门禁
make validate       # 结构、契约、分层依赖、质量静态校验
make build          # 构建后端二进制与两个前端
make docker-build   # 构建合并版应用镜像
make docker-up      # Docker Compose 启动
make docker-down    # Docker Compose 停止
```

数据库：

```bash
make db-create
make db-migrate-up
make db-migrate-status
make db-migrate-down
make db-migrate-version
make db-migrate-redo
```

逐包执行：

```bash
make -C packages/backend test quality
make -C packages/sandbox test quality
make -C packages/web test quality
make -C packages/ops-web test quality
```

## Docker

```bash
make docker-build        # 合并版 backend + sandbox + web + ops-web
make docker-build-split  # 独立 cp / dp 镜像，供后续多容器部署
make docker-build-all    # 以上全部
make docker-up           # 启动合并版 docker compose
make docker-down         # 停止
```

- 默认 `backend` 镜像包含 `cp`、`dp` 两个二进制，由 `docker-entrypoint.sh` 在一个容器内启动，分别监听 `8080` / `8081`。
- 多阶段 target 保留拆分能力：`--target cp`、`--target dp`，对应 `make docker-build-split`。
- `backend` 与 `sandbox` 容器使用非 root 用户运行。
- `web` / `ops-web` 支持构建参数：`VITE_CP_BASE`、`VITE_DP_BASE`、`VITE_OPS_API_BASE`、`VITE_DEMO_LOGIN`。
- 例如关闭演示登录构建：`make docker-build VITE_DEMO_LOGIN=false`。
- 所有应用镜像带有 `/health` 健康检查；compose 中 web / ops-web 会等待合并版 backend healthy。
- compose 只启动四个应用服务，MySQL 与 Redis 需要外部提供，并在 backend `.env` 中填写容器可访问的地址，不能使用 `127.0.0.1`。

## Kubernetes / Helm

Helm chart 位于 `deployment/helm`：

```text
deployment/helm/
  backend/    CP/DP，默认合并，支持 split
  sandbox/    sandbox + RWX 工作区
  web/        用户端
  ops-web/    运营后台
  vibebiz/    umbrella
```

```bash
helm dependency build deployment/helm/vibebiz
helm upgrade --install vibebiz deployment/helm/vibebiz -n vibebiz --create-namespace
```

- backend 默认合并部署，Service `backend:8080` 与 `backend-dp:8081`
- `backend.mode=split` 可切换 CP/DP 独立 Deployment
- sandbox 支持 existingClaim 或创建 RWX PVC
- web / ops-web Ingress 默认关闭
- 真实密钥使用 `existingSecret`，chart 不保存 MySQL / Redis / LLM 密钥
- 详细 values、Secret 与安装说明见 `deployment/helm/README.md`

## 配置说明

配置按 package 下沉到各自的 `.env`，`.env.example` 只包含占位值。

### Backend

| 变量 | 说明 |
|---|---|
| `HOST` / `CP_PORT` / `DP_PORT` | 监听地址与端口 |
| `MYSQL_HOST` / `MYSQL_USERNAME` / `MYSQL_PASSWORD` / `MYSQL_DATABASE` | MySQL 连接 |
| `REDIS_HOST` / `REDIS_PASSWORD` / `REDIS_DB` | Session、缓存、通知、请求计数 |
| `SANDBOX_REGISTRY_TOKEN` | sandbox 心跳共享 token |
| `SANDBOX_HEARTBEAT_TTL` | 心跳过期时间，默认 `15s` |
| `SANDBOX_HEALTH_INTERVAL` / `SANDBOX_HEALTH_TIMEOUT` | 健康检查周期与超时 |
| `ASYNQ_REDIS_DB` / `ASYNQ_CONCURRENCY` | asynq 独立 Redis DB 与并发 |
| `S3_*` | 头像与项目资料存储；`S3_REMOTE_DIR_PREFIX` 为桶内统一前缀；`S3_FORCE_PATH_STYLE` MinIO 用 `true`、阿里云 OSS 用 `false`；浏览器直传失败时回退 CP 代理上传 |
| `ENTERPRISE_HARNESS_S3_KEY` | 企业 Harness zip 的完整 S3 key，默认 `tmp/vibebiz_prod/vibebiz-harness.zip`；不拼接 `S3_REMOTE_DIR_PREFIX` |
| `NOTIFICATION_SECRET_KEY` | 通知渠道配置加密密钥；为空时回退 `MODEL_SECRET_KEY` |
| `SMTP_HOST` / `SMTP_PORT` / `SMTP_USERNAME` / `SMTP_PASSWORD` / `SMTP_FROM` | 平台发信 SMTP 配置 |
| `SMTP_SECURITY` | SMTP 安全模式：`starttls` / `tls` / `none` |
| `SESSION_USER_TTL` / `SESSION_OPS_TTL` | 用户 / 运营 Session 有效期 |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | 为空时关闭 Trace 导出 |

### Sandbox

| 变量 | 说明 |
|---|---|
| `WORKSPACES_DIR` | 工作区根目录，多节点必须挂载同一共享 NFS/RWX |
| `SANDBOX_PORT` | sandbox 监听端口 |
| `SANDBOX_NAME` | 实例唯一名称，默认 `<hostname>:<port>` |
| `SANDBOX_ADVERTISE_HOST` / `SANDBOX_ADVERTISE_PORT` | 上报给 CP 的可访问地址 |
| `SANDBOX_MAX_CONCURRENCY` | 上报最大并发 |
| `CP_REGISTRY_URL` | CP 心跳地址；为空则关闭注册 |
| `SANDBOX_HEARTBEAT_INTERVAL` | 心跳周期，默认 `5s` |
| `AGENT_MODE` | `stub` 或 `claude` |
| `ANTHROPIC_BASE_URL` / `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_API_KEY` | Agent 模型凭据 |
| `AGENT_MODEL` / `AGENT_SKILLS` / `AGENT_PLUGINS` / `AGENT_MAX_TURNS` | Agent 行为配置；默认最多 100 轮 |
| `AGENT_MAX_BUFFER_BYTES` | Claude SDK 单次输出缓冲上限，默认 32MB |

### Frontend

| 变量 | 说明 |
|---|---|
| `VITE_CP_BASE` / `VITE_DP_BASE` | web 的 CP / DP API 前缀 |
| `VITE_OPS_API_BASE` | ops-web 的 admin API 前缀 |
| `VITE_CP_PROXY_TARGET` / `VITE_DP_PROXY_TARGET` | 本地 Vite 代理目标 |
| `VITE_DEMO_LOGIN` | 默认 `true`，预填本地测试账号；生产设 `false` |

## API 概览

所有业务 API 使用统一响应信封：

```json
{ "code": 0, "message": "ok", "result": {} }
```

### CP · `/cp/api/v1`

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/auth/login` | 用户登录 |
| POST | `/auth/logout` | 退出登录 |
| GET | `/auth/me` | 当前用户 |
| GET / POST | `/projects` | 项目列表 / 创建项目（`mode=standard/enterprise`） |
| GET / DELETE | `/projects/:id` | 项目详情 / 删除 |
| PUT | `/projects/:id/agent-roles` | 更新项目角色；企业项目必须保留团队角色 |
| GET | `/projects/:id/stages` | 企业项目阶段列表 |
| POST | `/projects/:id/stages/:stageKey/start` | 开始当前阶段 |
| POST | `/projects/:id/stages/:stageKey/complete` | 确认当前阶段并推进 |
| GET | `/projects/:id/chat/history` | 聊天记录 |
| GET | `/agent-roles` | 可用 Agent 角色 |
| GET / POST | `/me/notification-channels` | 个人通知渠道列表 / 创建 |
| PUT / DELETE | `/me/notification-channels/:id` | 更新 / 删除个人通知渠道 |
| POST | `/me/notification-channels/:id/test` | 发送渠道测试消息 |
| GET | `/templates` / POST `/templates/:id/use` | 模板列表 / 使用模板 |
| GET / PUT | `/workspaces/current` | 当前工作区 |
| GET / POST | `/workspaces/current/members` | 成员列表 / 邀请 |
| GET | `/storage` | 工作区存储统计 |
| GET | `/admin/overview` | 运营概览 |
| GET | `/admin/users` | 用户列表 |
| GET | `/admin/projects` | 管理视角项目列表 |
| GET | `/admin/sandboxes` | sandbox 实例列表 |
| GET | `/admin/quotas` | 配额列表 |
| GET | `/admin/audit` | 审计事件 |
| POST | `/internal/sandboxes/heartbeat` | sandbox 心跳注册 |

### DP · `/dp/api/v1`

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/projects/:id/files` | 文件树 |
| GET | `/projects/:id/file?path=` | 读取文件 |
| PUT | `/projects/:id/file` | 写入文件，支持 `baseVersion` 冲突检测 |
| POST | `/projects/:id/chat` | SSE 对话流 |
| POST | `/projects/:id/chat/stop` | 停止当前生成 |
| GET | `/projects/:id/chat/status` | 当前生成状态 |
| GET | `/projects/:id/logs` | 项目执行日志 |
| POST | `/projects/:id/cover` | 上传项目封面 PNG |
| POST | `/resources/presign` | workspace staging 预签名 |
| DELETE | `/resources/staging/:resourceId` | 删除未 adopt 的 staging |
| POST | `/projects/:id/resources/adopt` | staging 转入项目并落盘 |
| GET | `/projects/:id/resources` | 项目资料列表 |
| DELETE | `/projects/:id/resources/:resourceId` | 删除项目资料 |
| GET | `/projects/:id/preview/*` | 预览反代 |

SSE 帧格式：

```text
data: {"type":"agent","payload":{...}}

data: [DONE]
```

文件读取返回 `content`、`version`、`size`、`mtime`、`isBinary`；写入版本不一致返回 HTTP 409。项目资料元数据存 MySQL `project_resource`，文件在 `.resources/`，不进入文件树、普通文件 API、预览或 git。执行日志位于项目 `.logs/agent_YYYY-MM-DD.jsonl`，保留 30 天，不进入普通文件树。

阶段接口的附加约束：

- `start` 只允许当前 `active` 阶段，已开始则幂等。
- `complete` 需要阶段已开始、Agent 空闲、固定产物存在且非空。
- 跳阶段、重复确认、运行中确认返回 `409`。
- 阶段 6/7 为占位状态，不能开始或确认。

## Sandbox 注册与调度

- sandbox 每 5 秒向 CP 上报名称、地址、当前并发与最大并发。
- CP 使用 asynq 每 10 秒请求一次 sandbox `/health`。
- 心跳过期或健康检查失败会标记实例 `offline`。
- 新项目按 `active_requests / max_concurrency` 选择最低压力的 `online + healthy` 节点，并写入 `project.sandbox_id`。
- 项目在线期间固定使用该节点；节点离线后自动改绑到其他健康节点，旧节点恢复后不抢回。
- sandbox 是无状态编码服务，项目工作区必须位于所有节点共享的 NFS/RWX 目录；切换节点不需要复制工作区。
- 同一项目的跨节点并发写尚未加 `flock` / lease / epoch，属于后续硬化项。

## 数据与存储

- MySQL 8 保存用户、工作区、项目、项目模式、项目阶段、通知渠道、角色、模板、资料、配额、审计、通知与 sandbox 实例。
- Redis 保存 Session、项目鉴权元数据、通知缓存 / PubSub、今日请求计数。
- 项目代码只存在于 sandbox 工作区，是唯一真相源；前端只持有投影。
- 企业项目创建时从 S3 导入 Harness，zip 中 `__MACOSX`、`.DS_Store`、`._*` 会被跳过，统一根目录会被剥离。
- 本地默认工作区在 `data/workspaces`；多节点部署时改成统一 NFS/RWX 挂载。
- 数据库迁移使用 goose，`AutoMigrate` 默认关闭。

## 可观测性

| 服务 | 日志 | Trace | Metrics |
|---|---|---|---|
| CP | stdout JSON | OTLP | `GET /metrics` |
| DP | stdout JSON | OTLP | `GET /metrics` |
| sandbox | stdout JSON | OTLP | `GET /metrics` |
| web / ops-web | 不接入 | 不接入 | 不接入 |

- 日志、span、指标标签带 `project_id`。
- 不记录文件正文、聊天正文、密钥、Authorization。
- `OTEL_EXPORTER_OTLP_ENDPOINT` 为空时正常启动并关闭导出。
- `/metrics` 不应暴露到公网。

## 测试与质量

```bash
make test
make quality
make validate
```

覆盖范围：

- Go：CP/DP 路由隔离、鉴权、admin、sandbox registry、调度与改绑、MySQL repository、企业模式/角色校验/Harness、阶段状态机、通知渠道加密与测试发送、`/metrics`
- Python：路径穿越、文件版本冲突、SSE、聊天记录、停止/状态、sandbox 心跳与并发计数
- web：SSE 分帧、API 信封解包、聊天状态、预览 URL、演示登录、企业阶段状态映射、通知渠道校验
- ops-web：admin 信封解包、路由与导航、演示登录

## 目录结构

```text
packages/
  web/         用户端：首页、标准编辑器、企业工程开发、聊天、预览、设置
  ops-web/     运营后台：概览、用户、项目、沙箱、配额、审计、通知
  backend/     Go：cmd/cp、cmd/dp、cmd/migrate、共享 internal（harness/projectstage/notificationchannel 等）
  sandbox/     Python：workspace、Agent、文件 API、预览、心跳
deployment/    Docker Compose 与 Helm charts
docs/          设计规格、实施计划、数据库与架构文档
scripts/       本地开发、质量门禁、校验、harness 工具
templates/     Go / Python / TypeScript / Vue / 文档模板
.ai/           跨会话上下文、规则、记忆、检查点
```

## 安全说明

- 仓库只提交 `.env.example`，真实 `.env` 已被忽略。
- 不要提交 MySQL / Redis 密码、S3 Access Key、LLM API Key、Session token。
- 本地默认账号与演示登录仅用于测试；生产必须删除或重置，并设置 `VITE_DEMO_LOGIN=false`。
- ops-web 当前使用本地 admin 账号，生产必须接入 SSO + RBAC，且 admin API 不得直接暴露公网。
- CP / DP / sandbox 默认只监听 `127.0.0.1`；容器部署时通过内部网络访问。
- 预览与主站同 origin 仅限本地原型；生产应使用独立子域并限制 iframe 权限。

## 当前限制与后续方向

- Agent 目前支持 `stub` 与 Claude Agent SDK；真实模型能力取决于 sandbox 配置。
- 企业工程当前只开放前 5 个阶段；制品空间、交付空间、内部云/第三方云/K8s 注册仍为后续版本。
- 企业阶段只支持顺序推进，没有阶段回退、重跑或并行阶段。
- 通知渠道当前只支持配置和测试发送，尚未接入系统通知自动投递、重试和投递记录。
- Harness 导入失败会回滚项目记录和元数据；sandbox 工作区清理尚未接入。
- 项目跨节点并发写还没有 lease/epoch/fencing，共享 NFS 场景下需要后续补强。
- 已提供 Helm charts，但尚未在真实 Kubernetes 集群验证；项目租约、容器级资源隔离、预览独立 origin、CI/CD 仍未完成。
- 运营后台写操作与权限模型仍是本地原型级。
- 观测后端（Collector / Jaeger / Grafana）不在仓库内交付，只提供接入能力。

## 相关文档

- [技术设计](docs/AI-Coding-Platform-技术设计文档.md)
- [业务数据库设计](docs/superpowers/specs/2026-10-28-business-database-design.md)
- [Sandbox 多类型与扩容架构](.ai/rules/sandbox-architecture.md)
- [Sandbox 注册中心设计](docs/superpowers/specs/2026-10-29-sandbox-registry-design.md)
- [企业工程开发设计](docs/superpowers/specs/2026-10-30-enterprise-engineering-design.md)
- [架构与编码规则](.ai/rules/architecture.md)
- [Helm 部署说明](deployment/helm/README.md)
- [本地 sandbox 冒烟测试](docs/sandbox-smoke-test.md)

# Architecture Rules · Vibebiz

## 0. 变更分级

- 大任务：新子系统 / 新页面组 / 鉴权与权限 / 数据库 schema / 外部服务 / 跨 package 契约 / 多 agent 并行。必须先写 `docs/superpowers/specs/YYYY-MM-DD-<topic>-design.md`，确认后再写实施计划。
- 小修改：单点 bug、文案、样式、配置值、测试修复，且不改契约 / schema / 权限 / 依赖方向。直接开发，不写设计文档。
- 不确定按大任务处理；时间紧可以写短设计，但不能跳过。


## 1. 分层与依赖方向

```
┌──────────────────────────────┐   ┌──────────────────────────────┐
│ packages/web · Vue3 :5173    │   │ packages/ops-web · Vue3 :5174│
│ 首页 / 编辑器 / 聊天 / 预览   │   │ 运营后台（只读骨架）          │
└───────┬──────────────┬───────┘   └──────────────┬───────────────┘
        │ /cp          │ /dp                      │ /cp/api/v1/admin
        ▼              ▼                          ▼
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│ CP · Go :8080    │  │ DP · Go :8081    │  │ 只访问 CP admin   │
│ 项目元数据/admin  │  │ 文件/SSE/预览     │  │                  │
└────────┬─────────┘  └────────┬─────────┘  └──────────────────┘
         │ init                │ files / chat / preview
         └──────────┬──────────┘
                    ▼
┌────────────────────────────────────────────┐
│ packages/sandbox · Python + FastAPI :8000  │
│ 工作区 · 文件 API · Agent 循环 · 静态预览   │
└──────────────────┬─────────────────────────┘
                   │
        data/workspaces/<projectId>/
```

硬规则：

1. `web` 不得直接访问 sandbox。控制请求走 CP，文件 / SSE / 预览走 DP。
2. `ops-web` 只访问 CP admin API（`/cp/api/v1/admin/*`），不得访问 DP、sandbox 或 `/internal/projects`。
3. `web` 不得访问 admin；两个前端不得互相访问。
4. CP 不承载文件字节、SSE 流或预览反代；DP 不承载 admin、项目列表或项目创建。
5. CP / DP 不得解析 `agent.payload`，不得理解工具调用结构。
6. `sandbox` 不得访问 CP / DP 内部实现或数据库。
7. 禁止跨 package import 源码；只通过 HTTP / SSE 契约通信。
8. 代码唯一真相源在 sandbox 工作区；前端只持有投影。

## 2. 端口与进程

| 服务 | dev 端口 | 容器端口 | 健康检查 | 业务前缀 |
|---|---|---|---|---|
| sandbox | 8000 | 8000 | `/health` | `/internal/*`（仅内网） |
| cp | 8080 | 8080 | `/health` | `/cp/api/v1` |
| dp | 8081 | 8081 | `/health` | `/dp/api/v1` |
| web | 5173 | 80 | `/` | 代理 `/cp` `/dp` |
| ops-web | 5174 | 80 | `/` | 代理 `/cp` |

- `/health` 与 `/metrics` 无业务前缀。
- 不接受无前缀业务路由 `/api/v1/*`。
- 代理层不重写前缀。
- `make dev` 启动五个进程；每个进程设置独立 `OTEL_SERVICE_NAME`。

## 3. API 契约

统一信封：

```json
{ "code": 0, "message": "ok", "result": {} }
```

错误：

```json
{ "code": 409, "message": "version conflict" }
```

### CP · `/cp/api/v1`

| 方法 | 路径 | sandbox 内部路径 |
|---|---|---|
| POST | `/auth/login` | 公开，用户登录 |
| GET | `/auth/me` | 用户 Cookie |
| POST | `/auth/logout` | 用户 Cookie |
| PUT | `/auth/workspace` | 切换当前工作区 |
| GET/PUT | `/me/profile` | 当前用户资料 / 改用户名 |
| POST/PUT | `/me/avatar/presign`, `/me/avatar` | 用户头像预签名上传 |
| GET/PUT | `/workspaces/current` | 当前工作区 |
| GET/POST | `/workspaces/current/members` | 成员列表 / 邀请 |
| GET | `/storage` | 当前工作区磁盘统计 |
| POST | `/projects` | 当前工作区；`POST /internal/projects/{id}/init` |
| GET | `/projects` | 用户登录 + 工作区过滤 |
| GET | `/projects/:id` | 用户登录 + 工作区过滤；读取 MySQL `project` |
| DELETE | `/projects/:id` | 用户登录 + 工作区过滤 |
| GET | `/projects/:id/chat/history` | 用户登录 + 工作区过滤；`GET /internal/projects/{id}/chat/history` |
| POST | `/admin/auth/login` | 公开，ops 登录 |
| GET | `/admin/auth/me` | ops Cookie |
| POST | `/admin/auth/logout` | ops Cookie |
| GET | `/admin/overview` | - |
| GET | `/admin/users` | - |
| GET | `/admin/projects` | - |
| GET | `/admin/sandboxes` | - |
| GET | `/admin/quotas` | - |
| GET | `/admin/audit` | - |
| POST | `/internal/sandboxes/heartbeat` | sandbox 心跳，`X-Sandbox-Token` |

### DP · `/dp/api/v1`

| 方法 | 路径 | sandbox 内部路径 |
|---|---|---|
| GET | `/projects/:id/stats`（CP → sandbox） | `GET /internal/projects/{id}/stats` |
| GET | `/projects/:id/files` | `GET /internal/projects/{id}/files` |
| GET | `/projects/:id/file?path=` | `GET /internal/projects/{id}/file` |
| PUT | `/projects/:id/file` | `PUT /internal/projects/{id}/file` |
| POST | `/projects/:id/chat` | `POST /internal/projects/{id}/chat` |
| POST | `/projects/:id/chat/stop` | `POST /internal/projects/{id}/chat/stop` |
| GET | `/projects/:id/preview/*` | `GET /internal/projects/{id}/preview/*` |

DP 不注册项目列表 / 创建 / 详情，也不注册任何 admin 路由。CP 不注册任何 `/dp/*` 路由。

除登录、`/health`、`/metrics` 外，CP 项目接口与 admin 接口、DP 项目接口全部要求 HttpOnly Cookie + Redis Session。DP 项目接口还校验 Redis 项目元数据中的工作区成员关系。

文件读取返回：

```json
{
  "path": "index.html",
  "content": "<!doctype html>...",
  "version": "sha256前12位",
  "size": 679,
  "mtime": 1793191707598,
  "isBinary": false
}
```

写入带 `baseVersion` 时不匹配 → HTTP 409。

### Sandbox 注册中心

- `sandbox_instance` 保存实例名称、host/port、online/offline、healthy/unhealthy、当前/最大并发、最后心跳。
- sandbox 每 5s 向 CP `POST /cp/api/v1/internal/sandboxes/heartbeat` 上报，`X-Sandbox-Token` 鉴权。
- CP 用 asynq 每 10s 拉取 `/health`；心跳过期或健康失败标记 offline。
- 新项目选最低压力健康节点并写 `project.sandbox_id`；节点离线后自动改绑，工作区依赖共享 NFS。
- 本次不做 `project_leases` / epoch / flock。

## 4. SSE 契约

- 帧格式：`data: <json>\n\n`
- 结束：`data: [DONE]\n\n`
- DP 必须逐行读取并 `Flush`，禁止整段缓冲。
- 反代必须 `proxy_buffering off` / `FlushInterval = -1`。

Agent 事件形状（sandbox → web，CP / DP 原样透传）：

```json
{ "type": "agent", "payload": { "type": "assistant", "message": { "content": [] } } }
{ "type": "agent", "payload": { "type": "user", "message": { "content": [] } } }
{ "type": "agent", "payload": { "type": "result", "subtype": "success" } }
{ "type": "sandbox-status", "state": "ready", "commit": "a1b2c3d" }
{ "type": "error", "message": "..." }
```

事件内部结构属于 sandbox 所有；CP / DP 不得据此分支。

## 5. Sandbox 工作区规则

```
data/workspaces/<projectId>/
├── index.html
├── styles.css
├── app.js
├── README.md
├── .chathistory/session.jsonl  (内部，文件树/预览不可见)
└── .git/
```

- `projectId` 必须匹配 `^[A-Za-z0-9_-]{1,64}$`。
- 所有路径：`Path.resolve()` 后 `is_relative_to(project_dir)`。
- 拒绝 `.git` / `.agent` / `.chathistory` / `node_modules` / `__pycache__` 访问。
- 聊天历史由 sandbox 独占写；CP 只通过内部 history API 读取，sandbox 不访问 CP/DP。
- 写入使用临时文件 + `os.replace`，避免半写。
- 版本号：`sha256(content).hexdigest()[:12]`。
- 二进制：前 8KB 含 NUL 即 `isBinary=true`，正文不返回。
- 每轮 Agent 结束自动 `git commit`；commit sha 作为版本。

## 6. Agent 规则（M0 stub / M1 真实）

- `StubAgent.run(project_id, message, workspace, session_id)` 是稳定接口。
- M1 替换实现时保持签名与事件形状不变。
- 工具范围：
  - `Read` / `List` / `Glob` / `Grep`：只读，限制在项目内。
  - `Write` / `Edit`：写前校验 `baseVersion`，写后广播版本。
  - `Bash`：M0 禁用；M4 容器隔离后再评估白名单。
- 大文件（>5MB）拒绝 Read，引导使用 grep / head / tail。
- LLM key 只在 sandbox 环境变量；不得写入项目或返回前端。

## 7. 前端规则

- Vue Composition API + `<script setup lang="ts">`。
- web Pinia 按领域：`project` / `files` / `chat`；ops-web 使用 `ops` store。
- web API 统一走 `src/api/client.ts`：控制请求 `cpHttp`，数据请求 `dpHttp`，信封在 `unwrapEnvelope` 解包。
- ops-web API 统一走 `src/api/client.ts`，基础地址 `VITE_OPS_API_BASE=/cp/api/v1/admin`。
- SSE 分帧是纯函数，必须有 vitest。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 预览 iframe 不给 `allow-top-navigation`；生产使用独立 origin。
- 前端不缓存权威代码；打开文件从 sandbox 读取。
- web / ops-web 不引入 OpenTelemetry、Prometheus 或日志 SDK。

## 8. 配置与密钥

| 变量 | 服务 | 说明 |
|---|---|---|
| `CP_PORT` | dev | CP 默认 8080；进程内注入 `PORT` |
| `DP_PORT` | dev | DP 默认 8081；进程内注入 `PORT` |
| `SANDBOX_REGISTRY_TOKEN` | CP / sandbox | 心跳共享 token |
| `SANDBOX_HEARTBEAT_TTL` | CP | 默认 `15s` |
| `CP_REGISTRY_URL` | sandbox | CP 心跳地址；为空关闭注册 |
| `SANDBOX_ADVERTISE_HOST` / `SANDBOX_ADVERTISE_PORT` | sandbox | 上报的可访问地址 |
| `CORS_ORIGINS` | CP / DP | 默认允许 5173 与 5174 |
| `WORKSPACES_DIR` | sandbox | 工作区根目录 |
| `AGENT_MODE` | sandbox | `stub` / `llm` |
| `LLM_API_KEY` | sandbox | 仅服务端 |
| `LLM_BASE_URL` | sandbox | OpenAI 兼容 |
| `LLM_MODEL_NAME` | sandbox | 模型名 |
| `VITE_CP_BASE` | web | 默认 `/cp/api/v1` |
| `VITE_DP_BASE` | web | 默认 `/dp/api/v1` |
| `VITE_OPS_API_BASE` | ops-web | 默认 `/cp/api/v1/admin` |
| `LOG_LEVEL` | CP / DP / sandbox | 默认 `info` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | CP / DP / sandbox | 空 = 关闭 trace 导出 |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | CP / DP / sandbox | 默认 `http/protobuf` |
| `OTEL_SERVICE_NAME` | 各进程 | `vibebiz-cp` / `vibebiz-dp` / `vibebiz-sandbox` |
| `OTEL_RESOURCE_ATTRIBUTES` | CP / DP / sandbox | 默认 `deployment.environment=local,service.namespace=vibebiz` |
| `OTEL_TRACES_SAMPLER` | CP / DP / sandbox | 默认 `parentbased_always_on` |
| `MYSQL_*` | CP（后续） | 目标 MySQL 8，业务表设计见 `docs/superpowers/specs/2026-10-28-business-database-design.md`；本轮未接入驱动 |

每个 package 独立维护 `Makefile`、`.env.example` 与 `.dockerignore`；`make init` 在 package 内生成 `.env`。根目录不保存统一 `.env`，`scripts/dev.sh` 与 compose 分别加载对应 package 的配置。`.env` 不入库且必须被 `.dockerignore` 排除；`.env.example` 只放占位。密钥与数据库凭据不得进入前端、日志或仓库。

## 9. 可观测性

| 服务 | 日志 | Trace | Metrics |
|---|---|---|---|
| CP | stdout JSON | OTLP | `/metrics` |
| DP | stdout JSON | OTLP | `/metrics` |
| sandbox | stdout JSON | OTLP | `/metrics` |
| web / ops-web | 不接入 | 不接入 | 不接入 |

- 日志字段：`time`、`level`、`service`、`plane`、`request_id`、`trace_id`、`span_id`、`project_id`、`route`、`status`、`duration_ms`、`msg`。
- 项目作用域的请求、sandbox 调用、SSE、Agent 运行必须带 `project_id`。
- 禁止记录：文件正文、聊天正文、LLM key、数据库密码、Authorization。
- W3C `traceparent` 在 CP / DP → sandbox 调用链上传播。
- `/metrics` 不加前缀，也不要求鉴权；生产必须只允许内网或 Prometheus 访问。
- 指标：`vibebiz_http_requests_total`、`vibebiz_http_request_duration_seconds`、`vibebiz_sandbox_requests_total`、`vibebiz_project_agent_runs_total`、`vibebiz_sse_events_total`。
- `route` 使用路由模板，禁止原始 URL。
- 不交付 Collector / Jaeger / Grafana；endpoint 为空时应用正常启动。

## 10. 测试规则

| 层 | 命令 | 要求 |
|---|---|---|
| backend | `go test ./...` | CP/DP 路由隔离、admin、httptest、sandbox fake、`/metrics` |
| sandbox | `pytest` | 路径穿越、版本冲突、SSE `[DONE]`、`/metrics` |
| web | `vitest` | SSE 分帧、双客户端信封解包、`buildPreviewUrl` |
| ops-web | `vitest` | admin 信封解包、路由表、导航配置 |

先写失败测试，再实现；修复 bug 必须有回归测试。

## 11. 语言级规范

| 语言 | 规则文件 | 工具 |
|---|---|---|
| Go | `.ai/rules/go.md` | `gofmt` / `go vet` / `go test` |
| Python | `.ai/rules/python.md` | `ruff check` / `ruff format` / `pytest` |
| TypeScript / Vue | `.ai/rules/typescript.md` | ESLint / Prettier / `vue-tsc` / vitest |

代码骨架见 `templates/`，用 `scripts/generate_from_template.py` 渲染。生成后必须格式化并补测试。

## 12. 部署规则

- 五个服务：`sandbox` / `cp` / `dp` / `web` / `ops-web`。
- `cp` / `dp` 共用 `packages/backend` 构建上下文，通过 `build.args.SERVICE=cp|dp` 选择入口。
- 所有服务提供 `/health`；CP / DP / sandbox 提供 `/metrics`。
- web 容器 nginx 反代 `/cp/` → `cp:8080`、`/dp/` → `dp:8081`，SSE 关闭 buffering。
- ops-web 容器 nginx 只反代 `/cp/` → `cp:8080`。
- sandbox 使用非 root 用户，工作区挂载卷。
- 当前不做 K8s / CI/CD；M5 再评估。
- 多类型 sandbox、项目独占租约、RWX 共享工作区、DP/SSE 扩容的目标态见 `.ai/rules/sandbox-architecture.md`。

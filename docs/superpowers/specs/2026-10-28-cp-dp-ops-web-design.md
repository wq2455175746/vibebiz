# CP / DP 拆分与 ops-web 运营后台 · 设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-28 |
| 状态 | 待评审 |
| 范围 | 后端控制面 / 数据面拆分、ops-web 运营后台、后端与 sandbox 可观测性、部署与校验同步 |
| 关联文档 | `docs/AI-Coding-Platform-技术设计文档.md`、`.ai/rules/architecture.md`、`README.md` |
| 数据库 | 目标 MySQL 8；本轮只预留接口，不建表、不引入驱动 |

---

## 1. 背景与目标

当前 `packages/backend` 是单一 Go 进程，同时承担项目元数据、文件代理、SSE 透传、预览反代和未来的运营管理能力。随着系统演进，控制类流量与数据类流量的扩缩容诉求、故障域和安全边界完全不同。

本规格要达成三个目标：

1. **同一份 Go 代码拆成两个可独立部署的服务**：控制面 `cp` 与数据面 `dp`，共享内部包，不复制业务代码。
2. **新增运营管理后台 `packages/ops-web`**：使用与 `web` 相同的 Vue3 + TypeScript + PrimeVue + Pinia 技术栈，只访问 CP 的 admin API。
3. **把拆分落到仓库结构、配置、Docker、代理、Makefile、校验脚本和设计文档**，形成可运行的 M0 骨架。
4. **CP、DP、sandbox 都具备按项目关联的日志、OpenTelemetry Trace 与 Prometheus Metrics**；web / ops-web 不接入观测 SDK。

## 2. 非目标

- 本文件保留 M0 骨架设计；本地账号登录已由 `2026-10-29-auth-workspace-session-design.md` 补充，SSO、RBAC 仍不在本轮范围。
- 不实现 MySQL 表结构、迁移、ORM 或数据访问层；业务表由后续独立任务设计。
- 不实现沙箱调度器、配额扣费、真实审计存储；admin API 返回固定形状的内存数据。
- 不改变 sandbox 的工作区、文件版本、SSE、预览契约。
- 不引入 K8s、CDN、签名直连 URL；这些属于后续里程碑。
- 不做 web 端 UI 视觉改版。
- 不交付 OpenTelemetry Collector、Jaeger、Tempo、Prometheus、Grafana 等观测后端；只保证数据可上报、指标可拉取。
- 不在 web / ops-web 引入浏览器端 OpenTelemetry、日志或指标 SDK。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 单 Go module，新增 `cmd/cp` 与 `cmd/dp` 两个入口 | 满足“同一份代码、分别部署”，避免拆 module 后基础设施重复 |
| D2 | CP 管控制与元数据，DP 管用户数据字节流 | 与设计文档 4.7 的控制面/数据面分离一致 |
| D3 | web 控制请求走 CP，数据请求直连 DP | 让预览、SSE、文件流量不经过 CP；为后续独立扩容留出位置 |
| D4 | ops-web 只访问 CP admin API，不访问 DP 与 sandbox | 运营后台不应进入用户数据路径 |
| D5 | 本轮 CP 继续使用内存 `ProjectStore`；admin 使用内存 Repository | 遵守 M0 无数据库边界，同时固定 HTTP 契约 |
| D6 | ops-web 使用本地 admin 账号登录，仅监听 localhost | 先验证信息架构与接口形状；生产安全要求写入文档 |
| D7 | 目标数据库改为 MySQL 8 | 用户已确认后续使用 MySQL；本轮不落表结构 |
| D8 | CP/DP 使用服务原生 URL 前缀 | CP 固定 `/cp/api/v1/*`，DP 固定 `/dp/api/v1/*`；代理与直连语义一致，不依赖路径重写 |
| D9 | 观测只覆盖 CP / DP / sandbox | 日志 stdout JSON、Trace 走 OTLP、Metrics 暴露 Prometheus `/metrics`；前端不接入 SDK |
| D10 | `project_id` 贯穿日志、span 与指标标签 | 满足按项目排查；接受骨架阶段的指标基数成本 |

## 4. 目标架构

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

### 4.1 依赖方向

```
ops-web ──▶ CP
web ──▶ CP（控制）
web ──▶ DP（数据）
CP ──▶ sandbox（初始化 / 编排）
DP ──▶ sandbox（文件 / SSE / 预览）
CP / DP / sandbox ──▶ OTLP endpoint（可选）
Prometheus ──▶ CP / DP / sandbox `/metrics`
```

禁止：

- web / ops-web 直接访问 sandbox。
- ops-web 访问 DP。
- DP 承载 admin、项目列表或项目创建。
- CP 承载文件字节、SSE 流或预览反代。
- 两个前端直接互相访问。

## 5. 服务职责

### 5.1 CP 控制面

负责：

- 项目创建、列表、详情。
- 创建项目时调用 sandbox `POST /internal/projects/{id}/init`。
- 运营后台 API：概览、用户、项目、沙箱、配额、审计。
- 后续的鉴权、配额、编排、审计入口。

不负责：

- 文件读取 / 写入。
- SSE 流式转发。
- 预览 HTTP 反代。

### 5.2 DP 数据面

负责：

- 文件树、文件读取、文件写入（含 409 版本冲突透传）。
- SSE 对话流透传与逐行 flush。
- 预览反代，含 Upgrade / HMR 透传。

不负责：

- 项目元数据存储。
- 项目创建 / 列表 / 详情。
- admin API。
- 用户、配额、审计。

DP 不依赖 CP；项目不存在时由 sandbox 返回 404，DP 原样透传。

### 5.3 sandbox

保持现有职责不变：工作区、文件 API、Agent 循环、静态预览。sandbox 不感知 CP/DP 的存在。

## 6. 仓库结构

```text
packages/
  backend/
    cmd/
      cp/main.go
      dp/main.go
    internal/
      admin/
        repository.go        # 运营后台 Repository 接口
        memory.go            # 本轮内存实现 + seed 数据
      client/
        sandbox.go
      config/
        config.go            # 两个入口共用，按入口设置默认端口
      handler/
        admin.go
        chat.go
        file.go
        preview.go
        project.go
      model/
      observability/
        logging.go           # JSON slog + 请求字段
        tracing.go           # OTel SDK / OTLP exporter
        metrics.go           # Prometheus exporter + HTTP 指标
      response/
      router/
        control.go           # 注册 CP 路由
        data.go              # 注册 DP 路由
      server/
      store/
  web/
  ops-web/                   # 新增
    index.html
    package.json
    vite.config.ts
    src/
      api/
      components/
      layouts/
      router/
      stores/
      styles/
      views/
  sandbox/
    app/
      observability.py       # JSON 日志 / OTel / Prometheus
```

约束：

- `cmd/cp` 只装配 CP 路由；`cmd/dp` 只装配 DP 路由。
- 两个入口共享 `internal/`，不复制 handler / client / response。
- 删除 `cmd/server`，避免出现第三个部署形态。
- 后端仍使用单 `go.mod`，不新增第二个 Go module。

## 7. API 契约

统一信封保持：

```json
{ "code": 0, "message": "ok", "result": {} }
```

### 7.0 URL 前缀

前缀是服务契约的一部分，不是部署层约定：

| 服务 | 业务 API 前缀 | 直连示例 |
|---|---|---|
| CP | `/cp/api/v1` | `http://localhost:8080/cp/api/v1/projects` |
| DP | `/dp/api/v1` | `http://localhost:8081/dp/api/v1/projects/{id}/files` |
| CP admin | `/cp/api/v1/admin` | `http://localhost:8080/cp/api/v1/admin/overview` |

- 两个服务都保留无前缀的 `/health` 作为探针。
- 不接受 `/api/v1/*` 作为业务路由。
- 代理层不重写前缀；Vite 与 nginx 都原样转发。
- 前端环境变量与服务前缀一致：`VITE_CP_BASE=/cp/api/v1`、`VITE_DP_BASE=/dp/api/v1`。

### 7.1 CP 路由

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | CP 健康检查 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/cp/api/v1/projects` | 项目列表 |
| POST | `/cp/api/v1/projects` | 创建项目 |
| GET | `/cp/api/v1/projects/:id` | 项目详情 |
| GET | `/cp/api/v1/admin/overview` | 运营概览 |
| GET | `/cp/api/v1/admin/users` | 用户列表 |
| GET | `/cp/api/v1/admin/projects` | 管理视角项目列表 |
| GET | `/cp/api/v1/admin/sandboxes` | 沙箱节点列表 |
| GET | `/cp/api/v1/admin/quotas` | 配额列表 |
| GET | `/cp/api/v1/admin/audit` | 审计事件列表 |

### 7.2 DP 路由

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/health` | DP 健康检查 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/dp/api/v1/projects/:id/files` | 文件树 |
| GET | `/dp/api/v1/projects/:id/file?path=` | 读取文件 |
| PUT | `/dp/api/v1/projects/:id/file` | 写入文件，409 透传 |
| POST | `/dp/api/v1/projects/:id/chat` | SSE 对话流 |
| GET | `/dp/api/v1/projects/:id/preview/*` | 预览反代 |

DP 不注册 `/dp/api/v1/projects` 列表 / 创建 / 详情，也不注册任何 `/cp/*` 或 admin 路由。CP 不注册任何 `/dp/*` 路由。

### 7.3 admin 响应形状

统一信封的 `result` 直接承载实体或数组，不再额外包一层 key：

| 端点 | `result` |
|---|---|
| `GET /cp/api/v1/admin/overview` | `Overview` 对象 |
| `GET /cp/api/v1/admin/users` | `User[]` |
| `GET /cp/api/v1/admin/sandboxes` | `Sandbox[]` |
| `GET /cp/api/v1/admin/quotas` | `Quota[]` |
| `GET /cp/api/v1/admin/audit` | `AuditEvent[]` |
| `GET /cp/api/v1/admin/projects` | `{ "projects": AdminProject[] }` |

示例：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "projects": 3,
    "users": 8,
    "runningSandboxes": 2,
    "todayRequests": 128
  }
}
```

`GET /cp/api/v1/admin/projects` 返回 CP 内存中的真实项目，并附加管理字段：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "projects": [
      { "id": "...", "name": "...", "state": "ready", "owner": "wangh11", "createdAt": "..." }
    ]
  }
}
```

ops-web 的 API 层可兼容 `result` 直接数组与 `{ users: [...] }` 两种形状，但 CP 自身固定返回上表形状。

## 8. web 前端改动

### 8.1 API 客户端

`src/api/client.ts` 保留统一信封解包，新增两个 axios 实例：

```ts
export const cpHttp = axios.create({ baseURL: import.meta.env.VITE_CP_BASE || '/cp/api/v1' })
export const dpHttp = axios.create({ baseURL: import.meta.env.VITE_DP_BASE || '/dp/api/v1' })
```

领域 API 归属：

| 文件 | 客户端 | 端点 |
|---|---|---|
| `src/api/projects.ts` | `cpHttp` | `/projects` |
| `src/api/files.ts` | `dpHttp` | `/projects/:id/files`、`/projects/:id/file` |
| `src/api/chat.ts` | `dpHttp` | `/projects/:id/chat` |
| `PreviewPanel` | `dpHttp` 基础地址 | `/projects/:id/preview/` |

### 8.2 预览 URL

新增纯函数 `buildPreviewUrl(projectId)`，从 `VITE_DP_BASE` 生成 iframe 地址，并补 vitest。禁止在组件内硬编码 `localhost:8000` 或 sandbox 内部路径。

### 8.3 环境变量

```env
VITE_CP_BASE=/cp/api/v1
VITE_DP_BASE=/dp/api/v1
```

原 `VITE_API_BASE` 移除，由上述两个变量取代。

### 8.4 行为保持不变

- 首页创建项目 → CP → sandbox 初始化 → 跳转编辑器。
- 编辑器文件树 / 读写 / SSE / 预览全部走 DP。
- 409 版本冲突、SSE `[DONE]`、统一信封语义不变。

## 9. ops-web 设计

### 9.1 技术栈

Vue3 + Vite + TypeScript + PrimeVue + Pinia，与 `web` 共用语言规范与调色板。依赖集合与 `web` 对齐（axios、PrimeVue、Pinia、vitest、ESLint、Prettier），不引入新的 UI 框架、状态库或图表库。

### 9.2 信息架构

| 路由 | 页面 | 数据来源 |
|---|---|---|
| `/` | 概览 | `GET /admin/overview` + `GET /admin/projects` |
| `/users` | 用户管理 | `GET /admin/users` |
| `/projects` | 项目管理 | `GET /admin/projects` |
| `/sandboxes` | 沙箱节点 | `GET /admin/sandboxes` |
| `/quotas` | 配额管理 | `GET /admin/quotas` |
| `/audit` | 审计日志 | `GET /admin/audit` |
| `/settings` | 系统设置 | 静态只读信息：端口、环境、数据库目标 |

### 9.3 布局

- 左侧固定侧栏：Logo、运营后台标题、模块导航。
- 顶部栏：当前页面标题、环境标识 `LOCAL`。
- 内容区：卡片 + 表格骨架，黑白灰蓝配色。
- 骨架阶段表格支持空状态、加载态与错误态；不做编辑、删除、筛选等写操作。

### 9.4 API 客户端

- 基础地址 `VITE_OPS_API_BASE=/cp/api/v1/admin`。
- 复用统一信封解包逻辑；纯函数补 vitest。
- 不引用 `dp`、sandbox 或 `web` 的源码。

### 9.5 鉴权边界

本地 admin 登录已实现：`/cp/api/v1/admin/auth/login`、`/me`、`/logout`，Cookie 为 HttpOnly，Session 存 Redis。生产环境仍必须接入 SSO + RBAC，admin 路由不得裸露在公网。详见 `2026-10-29-auth-workspace-session-design.md`。

## 10. 部署与端口

| 服务 | dev 端口 | 容器端口 | 健康检查 |
|---|---|---|---|
| sandbox | 8000 | 8000 | `/health` |
| cp | 8080 | 8080 | `/health` |
| dp | 8081 | 8081 | `/health` |
| web | 5173 | 80 | `/` |
| ops-web | 5174 | 80 | `/` |

### 10.1 开发代理

web `vite.config.ts`：

- `/cp` → `http://localhost:8080`，保留前缀，不重写。
- `/dp` → `http://localhost:8081`，保留前缀，不重写。

ops-web `vite.config.ts`：

- `/cp` → `http://localhost:8080`，保留前缀，不重写。

### 10.2 生产 nginx

web：

- `location /cp/ { proxy_pass http://cp:8080; }`，保留前缀。
- `location /dp/ { proxy_pass http://dp:8081; }`，保留前缀并关闭 buffering。
- `/` 走 SPA fallback。

ops-web：

- `location /cp/ { proxy_pass http://cp:8080; }`，保留前缀。
- `/` 走 SPA fallback。

### 10.3 Docker

后端保留一份 `packages/backend/Dockerfile`，通过构建参数选择入口：

```dockerfile
ARG SERVICE=cp
RUN CGO_ENABLED=0 go build -o /out/server ./cmd/${SERVICE}
```

compose 中：

- `cp` 使用 `build.args.SERVICE=cp`，端口 8080。
- `dp` 使用 `build.args.SERVICE=dp`，端口 8081。
- `web` 依赖 `cp` 与 `dp`，发布 `5173:80`。
- `ops-web` 依赖 `cp`，发布 `5174:80`。

### 10.4 环境变量

```env
PORT=8080                    # CP
SANDBOX_BASE_URL=http://localhost:8000
CORS_ORIGINS=http://localhost:5173,http://localhost:5174

VITE_CP_BASE=/cp/api/v1
VITE_DP_BASE=/dp/api/v1
VITE_OPS_API_BASE=/cp/api/v1/admin

LOG_LEVEL=info
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_SERVICE_NAME=vibebiz-cp
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=local,service.namespace=vibebiz
OTEL_TRACES_SAMPLER=parentbased_always_on
```

DP 进程单独使用 `PORT=8081` 启动；其余配置与 CP 一致。观测变量说明见 13.6。

## 11. 数据与存储

- CP 持有 `store.ProjectStore`，本轮仍为内存实现，重启即丢。
- admin 数据通过 `internal/admin.Repository` 读取；本轮实现为内存 seed。
- DP 不持有项目元数据。
- 目标数据库为 MySQL 8；本轮不引入 MySQL 驱动、迁移或表结构。
- 业务表结构由后续独立设计；接入时替换 `ProjectStore` 与 `admin.Repository` 实现，HTTP 契约保持不变。
- 不在前端、日志或配置示例中写入数据库密码。

## 12. 安全

- ops-web 与 admin API 使用本地账号登录；生产必须接 SSO + RBAC。
- `CORS_ORIGINS` 只允许 web 与 ops-web 的开发地址。
- 前端不得直接访问 sandbox；DP 是唯一数据出口。
- DP 透传 sandbox 的 404 / 409 / 502，不把内部错误细节改写成成功响应。
- 文件路径安全、版本校验仍由 sandbox 负责，DP 不做字符串前缀判断。
- MySQL 凭据只存在于 CP 服务端环境变量，禁止进入前端或仓库。

## 13. 可观测性

### 13.1 范围

| 服务 | 日志 | Trace | Metrics |
|---|---|---|---|
| CP | stdout JSON | OTLP | `/metrics` |
| DP | stdout JSON | OTLP | `/metrics` |
| sandbox | stdout JSON | OTLP | `/metrics` |
| web | 不接入 | 不接入 | 不接入 |
| ops-web | 不接入 | 不接入 | 不接入 |

观测后端不在本仓库交付。只要配置 OTLP endpoint 与 Prometheus scrape，就能接入任意兼容后端。

### 13.2 日志

- Go 使用 `slog` JSON handler 输出到 stdout；Python 使用 JSON formatter 输出到 stdout。
- 每行一条 JSON，固定字段：

```json
{
  "time": "2026-10-28T12:00:00.000Z",
  "level": "info",
  "service": "vibebiz-dp",
  "plane": "dp",
  "request_id": "...",
  "trace_id": "...",
  "span_id": "...",
  "project_id": "...",
  "route": "GET /dp/api/v1/projects/:id/file",
  "status": 200,
  "duration_ms": 12,
  "msg": "request.complete"
}
```

- 项目作用域的请求、sandbox 调用、SSE、Agent 运行必须带 `project_id`。
- 必须记录的事件：
  - 请求开始 / 结束：method、route、status、duration。
  - sandbox 调用：operation、status、duration。
  - `project.create`、`file.read`、`file.write`、`chat.start`、`chat.end`、`sse.event`、`agent.run.start`、`agent.run.end`。
  - 错误：错误码、错误消息、plane、project_id。
- 禁止记录：文件正文、聊天正文、LLM key、数据库密码、Authorization。
- 日志级别由 `LOG_LEVEL` 控制，默认 `info`。

### 13.3 Trace

- 统一使用 OpenTelemetry SDK，W3C `traceparent` 传播。
- 导出协议默认 OTLP/HTTP protobuf；通过标准环境变量覆盖。
- `service.name` 固定为 `vibebiz-cp`、`vibebiz-dp`、`vibebiz-sandbox`。
- `deployment.environment` 从 `OTEL_RESOURCE_ATTRIBUTES` 注入，本地默认 `local`。
- Span 规划：

| 服务 | Span | 关键属性 |
|---|---|---|
| CP | HTTP server span | `http.route`、`http.status_code` |
| CP | `project.create` | `project.id`、`sandbox.operation=init` |
| DP | HTTP server span | `http.route`、`http.status_code`、`project.id` |
| DP | `sandbox.request` | `project.id`、`sandbox.operation`、`http.status_code` |
| DP | `chat.stream` | `project.id`、`sse.done=true` |
| DP | `preview.proxy` | `project.id`、`http.route` |
| sandbox | HTTP server span | `http.route`、`project.id` |
| sandbox | `workspace.read` / `workspace.write` | `project.id`、`file.path`、`file.version` |
| sandbox | `agent.run` | `project.id`、`agent.mode`、`agent.status` |

- CP / DP 调 sandbox 时注入 `traceparent`，sandbox 继续同一 trace。
- 所有项目相关 span 必须带 `project.id`。
- `OTEL_EXPORTER_OTLP_ENDPOINT` 为空时不初始化 exporter，应用正常运行且不报错；设置后自动上报。
- 采样策略由标准 `OTEL_TRACES_SAMPLER` 控制，本地默认 `parentbased_always_on`。

### 13.4 Metrics

- 三个服务都在同一业务端口暴露 `GET /metrics`，返回 Prometheus 文本格式。
- 路径固定为 `/metrics`，不加 `/cp` 或 `/dp` 前缀。
- 指标端点不要求鉴权；生产环境必须只允许内网或 Prometheus 实例访问，不通过公网 nginx 暴露。
- 指标命名与标签：

| 指标 | 类型 | 标签 |
|---|---|---|
| `vibebiz_http_requests_total` | counter | `service`、`plane`、`route`、`method`、`status`、`project_id` |
| `vibebiz_http_request_duration_seconds` | histogram | `service`、`plane`、`route`、`method` |
| `vibebiz_sandbox_requests_total` | counter | `operation`、`status`、`project_id`；Go 另带 `service`、`plane` |
| `vibebiz_project_agent_runs_total` | counter | `project_id`、`mode`、`status` |
| `vibebiz_sse_events_total` | counter | `project_id`、`type` |

- `route` 使用路由模板，禁止使用原始 URL；避免路径参数爆炸。
- `project_id` 是高基数标签，骨架阶段接受；后续按 project 聚合或引入 exemplar 降低基数。
- 不提供 push 写入接口；Prometheus 只通过 `/metrics` 拉取。

### 13.5 依赖

- Go：`go.opentelemetry.io/otel`、`otel/sdk`、OTLP trace exporter、`otelhttp`、`prometheus/client_golang`（指标直接使用 Prometheus client，避免指标名经过二次转换）。
- Python：`opentelemetry-api`、`opentelemetry-sdk`、`opentelemetry-exporter-otlp`、`opentelemetry-instrumentation-fastapi`、`prometheus-client`。
- web / ops-web 不新增任何观测依赖。

### 13.6 配置

```env
LOG_LEVEL=info
OTEL_EXPORTER_OTLP_ENDPOINT=
OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
OTEL_SERVICE_NAME=vibebiz-cp
OTEL_RESOURCE_ATTRIBUTES=deployment.environment=local,service.namespace=vibebiz
OTEL_TRACES_SAMPLER=parentbased_always_on
```

- 每个进程使用独立的 `OTEL_SERVICE_NAME`：

| 进程 | `OTEL_SERVICE_NAME` |
|---|---|
| CP | `vibebiz-cp` |
| DP | `vibebiz-dp` |
| sandbox | `vibebiz-sandbox` |

- 本地不配置 endpoint 时，仅输出日志与 `/metrics`，不产生 trace 导出错误。
- 生产密钥与 endpoint 只通过环境变量注入，不写入仓库。

## 14. 测试策略

| 层 | 测试 | 要求 |
|---|---|---|
| CP | `go test ./...` | 项目创建/列表/详情、admin 六个端点、统一信封、`/metrics`、trace 属性 |
| DP | `go test ./...` | 文件树/读写、409、SSE `[DONE]`、预览反代、无 admin 路由、`/metrics` |
| sandbox | `pytest` | 路径穿越、409、SSE、`/metrics`、span `project.id` |
| web | vitest | 双客户端信封解包、`buildPreviewUrl`、SSE 分帧 |
| ops-web | vitest | admin 信封解包、路由表与导航配置 |
| 集成 | 脚本/手工 | CP 创建项目 → DP 文件/SSE/预览 → ops 概览出现项目 |

观测测试要求：

- Go 用 `tracetest.SpanRecorder` 断言 span 名称与 `project.id`，用 `prometheus/testutil` 断言计数器。
- Python 用 `InMemorySpanExporter` 断言 span 属性，用 `prometheus_client.generate_latest` 断言指标文本。
- `/metrics` 测试必须覆盖 CP、DP、sandbox 三个端点。
- 未配置 OTLP endpoint 时，应用启动、请求与测试不得产生 exporter 错误。

质量门禁扩展到 ops-web：ESLint、Prettier、调色板扫描。

## 15. 校验与文档同步

### 15.1 `scripts/validate.py`

- backend 必需文件改为 `cmd/cp/main.go`、`cmd/dp/main.go`，删除 `cmd/server/main.go`。
- 新增 `packages/ops-web` 结构校验。
- 契约检查：
  - CP router 注册 `/cp/api/v1`，含 `/admin`，不含 `/preview`。
  - DP router 注册 `/dp/api/v1`，含 `/preview` 与 `/chat`，不含 `/admin` 或 `/cp`。
  - 两个 router 都不注册无前缀的业务 `/api/v1`。
  - CP / DP router 都注册 `/metrics`；sandbox 应用注册 `/metrics`。
  - web 同时引用 CP / DP 基础地址。
  - ops-web 只引用 CP admin 基础地址。

### 15.2 `scripts/check_dependencies.py`

- web 不得出现 sandbox 地址或 `/internal/projects`。
- ops-web 不得出现 `/dp/`、sandbox 地址或 `/internal/projects`。
- `cmd/cp` 不得注册 DP 路由，`cmd/dp` 不得注册 CP 路由；由 `validate.py` 对 router 注册内容做静态检查。
- 后端仍禁止 AI SDK。
- web / ops-web 的 `package.json` 不得包含 `@opentelemetry/*`、`prom-client` 或其他观测 SDK。
- CP / DP / sandbox 必须保留 `/metrics` 注册。

### 15.3 `scripts/check_quality.py`

- TypeScript/Vue 检查覆盖 `packages/web/src` 与 `packages/ops-web/src`。
- 两个前端都执行 ESLint 与调色板扫描。

### 15.4 文档

- `docs/AI-Coding-Platform-技术设计文档.md`：更新架构图、CP/DP 边界、ops-web、MySQL 目标、端口与部署。
- `README.md`：目录、端口、命令、最小链路。
- `AGENTS.md` 与 `.ai/AGENTS.md`：模块路由、边界、命令。
- `.ai/rules/architecture.md`：分层图、端口、契约、测试、部署。
- 各 package 的 `.env.example` / `Makefile`、根 `docker-compose.yml`、根聚合 `Makefile`、`scripts/dev.sh`。

## 16. 迁移步骤

1. 更新文档、规则、环境变量与端口表。
2. 后端新增 `cmd/cp`、`cmd/dp`，拆分 router 为 `control.go` / `data.go`，分别注册 `/cp/api/v1` 与 `/dp/api/v1`，删除 `cmd/server`。
3. 新增 `internal/admin` 内存 Repository 与 admin handler。
4. 新增 `internal/observability`：JSON 日志、OTel trace、Prometheus `/metrics`。
5. 调整后端测试：CP 与 DP 分别覆盖，并断言 span 属性与指标。
6. web 拆分 `cpHttp` / `dpHttp`，更新 projects/files/chat/PreviewPanel。
7. 新增 `packages/ops-web` 骨架与页面。
8. sandbox 新增 `app/observability.py`、OTel FastAPI instrumentation 与 `/metrics`。
9. 更新 Vite 代理、nginx、Dockerfile、compose、各 package Makefile/.env.example、根聚合 Makefile、dev.sh。
10. 扩展 validate / deps-check / quality。
11. 跑 `make test`、`make validate`、`make build` 与端到端冒烟。

## 17. 验收标准

1. `make dev` 同时启动 sandbox、CP、DP、web、ops-web。
2. `http://localhost:8080/health`、`http://localhost:8081/health`、`http://localhost:5174/` 可访问。
3. 首页创建项目成功，项目出现在 CP 的项目列表。
4. 编辑器文件树、读写、SSE、预览全部通过 DP 工作，409 与 `[DONE]` 行为不变。
5. ops-web 概览显示项目数，项目页显示刚创建的项目；其余页面显示固定 seed 数据。
6. 前缀隔离生效：DP 访问 `/dp/api/v1/admin/overview` 与 `/cp/api/v1/projects` 均返回 404；CP 访问 `/cp/api/v1/projects/:id/preview/` 与 `/dp/api/v1/projects` 均返回 404。
7. 直连 `http://localhost:8080/cp/api/v1/projects` 与 `http://localhost:8081/dp/api/v1/projects/{id}/files` 均可用，不依赖代理重写。
8. web 与 ops-web 源码不包含 sandbox 内部地址；ops-web 不访问 DP。
9. CP `/metrics`、DP `/metrics`、sandbox `/metrics` 均返回 200，且包含 `vibebiz_http_requests_total`。
10. 项目相关请求日志包含 `project_id`、`trace_id`、`span_id`、`request_id`；不包含正文或密钥。
11. 配置 `OTEL_EXPORTER_OTLP_ENDPOINT` 后，CP / DP / sandbox 的 span 可到达外部 OTLP 接收端；未配置时应用正常运行。
12. web 与 ops-web 的依赖和源码不包含 OpenTelemetry / Prometheus SDK。
13. `make test`、`make quality`、`make validate`、`make build` 全部通过。
14. 技术设计文档、README、AGENTS、architecture 规则与代码一致。

## 18. 风险与后续

| 风险 | 影响 | 处置 |
|---|---|---|
| CP 内存存储重启丢失 | 本地开发数据不稳定 | 保持 M0 边界；MySQL 接入是下一步 |
| ops-web 无鉴权 | 管理接口暴露 | 仅监听 localhost；生产接 SSO/RBAC |
| 前端直连 DP | CORS 与生产域名配置复杂 | 开发用 Vite 代理，生产用同域 `/dp/` 反代 |
| 拆分后项目元数据与 sandbox 状态不一致 | DP 仍可访问已删除项目 | sandbox 404 兜底；后续由 CP 编排回收 |
| 两端共享 internal 包 | 边界可能被无意打破 | validate 检查路由注册，code review 检查职责 |
| `project_id` 指标基数增长 | Prometheus 内存与查询成本上升 | 骨架阶段接受；后续按项目聚合或只保留 exemplar |
| 日志仅 stdout、不落盘 | 没有外部采集时无法追溯 | 文档明确需接日志采集；本轮不交付存储 |
| 未交付 Collector / Grafana | 无现成 Trace 查看界面 | 只保证 OTLP 可上报；用户可接任意兼容后端 |

后续任务：

1. 用户提供 MySQL 业务表结构后，替换 `ProjectStore` 与 `admin.Repository`。
2. 接入管理员登录、SSO、RBAC 与审计落库。
3. 引入短期签名票据，让文件与预览进一步绕开同域代理。
4. 可选交付 Collector / Grafana / alert rules 模板与采样策略调优。

## 19. 需求映射

| 用户要求 | 落地 |
|---|---|
| backend 同一份代码拆成 CP/DP | 单 Go module、`cmd/cp` + `cmd/dp`、共享 internal |
| 两个服务分别部署 | 两个 Docker 构建参数、两个 compose 服务、独立端口与健康检查 |
| 使用同样的技术栈 | CP/DP 均为 Go + Echo；ops-web 复用 Vue3 + TS + PrimeVue + Pinia |
| 新增 ops-web 管理整个系统 | `packages/ops-web` + CP admin API + 七个管理模块 |
| MySQL 后续设计 | 目标 MySQL 8；本轮只预留 Repository，不建表 |
| 每个项目有完整日志 / Trace / Metrics | `project_id` 贯穿日志、span 与指标；CP/DP/sandbox 输出 JSON 日志、OTLP trace、`/metrics` |
| 前端不接入观测 | web / ops-web 不新增 OTel、日志或指标 SDK |

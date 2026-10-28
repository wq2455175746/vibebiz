# CP / DP 拆分、ops-web 与可观测性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把单一 Go backend 拆成 CP/DP 两个可独立部署的服务，新增 ops-web 运营后台，并让 CP/DP/sandbox 输出按项目关联的 JSON 日志、OTLP Trace 和 Prometheus `/metrics`。

**Architecture:** 单 Go module，`cmd/cp` 管项目元数据与 admin，`cmd/dp` 管文件/SSE/预览；web 控制请求走 `/cp/api/v1`，数据请求走 `/dp/api/v1`；ops-web 只访问 `/cp/api/v1/admin`。观测只覆盖 CP/DP/sandbox。

**Tech Stack:** Go 1.24 + Echo、OpenTelemetry Go、prometheus/client_golang；Python 3.13 + FastAPI + OpenTelemetry Python + prometheus-client；Vue3 + Vite + TS + PrimeVue + Pinia。

**Spec:** `docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`

## Global Constraints

- 单 Go module：`packages/backend/go.mod`；禁止新增第二个 go.mod。
- CP 业务前缀 `/cp/api/v1`，DP 业务前缀 `/dp/api/v1`；不使用无前缀业务 `/api/v1`。
- `/health` 与 `/metrics` 无前缀；三个服务都必须暴露 `/metrics`。
- 端口：sandbox 8000、CP 8080、DP 8081、web 5173、ops-web 5174。
- `project_id` 必须出现在项目作用域的日志、span 和指标标签中。
- 日志 stdout JSON；禁止记录文件正文、聊天正文、密钥、Authorization。
- web / ops-web 不新增 OpenTelemetry、Prometheus 或日志 SDK。
- 不交付 Collector/Jaeger/Grafana；`OTEL_EXPORTER_OTLP_ENDPOINT` 为空时 trace 导出关闭且不报错。
- 不引入 MySQL 驱动或建表；admin 数据用内存 seed。
- 当前目录不是 git 仓库，跳过所有 commit 步骤。
- 格式化：Go `gofmt`；Python `.venv/bin/python -m ruff format app tests`；两个前端 `npm run format`。
- 质量门禁：`make quality`、`make validate`、`make test`、`make build` 必须通过。

---

## 并行工作流与文件所有权

| 工作流 | Agent | 只允许改动的路径 |
|---|---|---|
| A · Backend CP/DP + 观测 | backend-agent | `packages/backend/**` |
| B · Sandbox 观测 | sandbox-agent | `packages/sandbox/**` |
| C · Web 双客户端 | web-agent | `packages/web/**` |
| D · ops-web 骨架 | ops-agent | `packages/ops-web/**` |
| E · 集成/配置/文档 | integration-agent | 根目录、`scripts/**`、`docs/**`、`.ai/**`、`docker-compose.yml`、各 package `Makefile`/`.env.example`、根聚合 `Makefile` |
| F · 验证 | verifier | 只读 + 少量修复；不得改变接口 |

A/B/C/D 可完全并行。E 依赖 A/B/C/D 的目录与接口形状，但可按本计划先行编写；F 在 A-E 完成后执行。

---

## 工作流 A · Backend CP/DP + 可观测性

**Files:**
- Create: `packages/backend/cmd/cp/main.go`
- Create: `packages/backend/cmd/dp/main.go`
- Create: `packages/backend/internal/observability/logging.go`
- Create: `packages/backend/internal/observability/tracing.go`
- Create: `packages/backend/internal/observability/metrics.go`
- Create: `packages/backend/internal/observability/middleware.go`
- Create: `packages/backend/internal/router/control.go`
- Create: `packages/backend/internal/router/data.go`
- Create: `packages/backend/internal/router/errors.go`
- Create: `packages/backend/internal/handler/admin.go`
- Create: `packages/backend/internal/admin/repository.go`
- Create: `packages/backend/internal/admin/memory.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/server/server.go`
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/router/router.go`（删除或改为共享辅助）
- Modify: `packages/backend/internal/handler/*_test.go`
- Modify: `packages/backend/Dockerfile`
- Delete: `packages/backend/cmd/server/main.go`
- Delete: `packages/backend/internal/router/router.go` 中的旧 `Register`（可整体替换）

**Interfaces:**
- `config.Load(plane string) Config`，`Config` 新增 `Plane string`、`ServiceName string`、`Port` 默认值：cp=8080、dp=8081。
- `observability.New(ctx context.Context, serviceName, plane string) (*Provider, error)`。
- `Provider` 内嵌 `*Metrics`，并暴露：`Logger *slog.Logger`、`MetricsHandler http.Handler`、`Tracer trace.Tracer`、`Shutdown func(context.Context) error`。
- `Metrics` 暴露 `HTTPRequests`、`HTTPDuration`、`SandboxRequests`、`SSEEvents`。
- `Provider.Middleware() echo.MiddlewareFunc`：负责 otelecho、请求日志、HTTP 指标、`project.id` span 属性。
- `client.NewSandbox(baseURL string, obs *observability.Provider) *Sandbox`；`obs` 可为 nil。
- `handler.NewProjectHandler(projects, sandbox, obs)`、`NewFileHandler(sandbox, obs)`、`NewChatHandler(sandbox, obs)`、`NewPreviewHandler(sandbox, obs)`。
- `router.RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, sandbox *client.Sandbox, obs *observability.Provider)`。
- `router.RegisterData(e *echo.Echo, sandbox *client.Sandbox, obs *observability.Provider)`。
- `admin.NewMemoryRepository() admin.Repository`。
- `admin.Repository`：`Overview(projectCount int) Overview`、`Users() []User`、`Sandboxes() []Sandbox`、`Quotas() []Quota`、`Audit() []AuditEvent`。

- [ ] **A1. 添加 Go 依赖**

```bash
cd packages/backend
go get go.opentelemetry.io/otel \
  go.opentelemetry.io/otel/sdk \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/otel/propagation \
  go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp \
  github.com/prometheus/client_golang/prometheus \
  github.com/prometheus/client_golang/prometheus/promhttp
go mod tidy
```

- [ ] **A2. 先写观测失败测试**

Create `packages/backend/internal/observability/observability_test.go`:

```go
package observability

import (
    "context"
    "net/http/httptest"
    "strings"
    "testing"
)

func TestMetricsHandlerExposesProjectCounter(t *testing.T) {
    ctx := context.Background()
    p, err := New(ctx, "vibebiz-test", "cp")
    if err != nil {
        t.Fatal(err)
    }
    defer p.Shutdown(ctx)

    p.HTTPRequests.WithLabelValues("vibebiz-test", "cp", "GET /projects/:id", "GET", "200", "p1").Inc()

    rec := httptest.NewRecorder()
    req := httptest.NewRequest("GET", "/metrics", nil)
    p.MetricsHandler.ServeHTTP(rec, req)

    body := rec.Body.String()
    if !strings.Contains(body, "vibebiz_http_requests_total") {
        t.Fatalf("metrics missing counter: %s", body)
    }
    if !strings.Contains(body, `project_id="p1"`) {
        t.Fatalf("metrics missing project_id: %s", body)
    }
}
```

Run: `go test ./internal/observability -run TestMetricsHandlerExposesProjectCounter -v`
Expected: FAIL because package does not exist.

- [ ] **A3. 实现 observability 包**

`metrics.go`：用独立 `prometheus.NewRegistry()` 注册：

```go
type Metrics struct {
    HTTPRequests    *prometheus.CounterVec
    HTTPDuration    *prometheus.HistogramVec
    SandboxRequests *prometheus.CounterVec
    SSEEvents       *prometheus.CounterVec
}
```

标签：

- `HTTPRequests`：`service, plane, route, method, status, project_id`
- `HTTPDuration`：`service, plane, route, method`
- `SandboxRequests`：`service, plane, operation, status, project_id`
- `SSEEvents`：`project_id, type`

`tracing.go`：读取 `OTEL_EXPORTER_OTLP_ENDPOINT`；为空返回 noop tracer provider；非空用 `otlptracehttp.New(ctx)` + `sdktrace.NewBatchSpanProcessor`，resource 包含 `service.name`、`service.namespace=vibebiz`、`deployment.environment`、`plane`。安装全局 TextMapPropagator。

`logging.go`：`slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))`，默认 attrs `service`、`plane`；`LOG_LEVEL` 支持 debug/info/warn/error。

`middleware.go`：按顺序：

1. `otelecho.Middleware(serviceName)`
2. 自定义中间件：跳过 `/metrics`、`/health`；开始时间；`next(c)`；`route := c.Path()`；`projectID := c.Param("id")`；设置 span attribute `project.id`；记录 `vibebiz_http_requests_total`、`vibebiz_http_request_duration_seconds`；输出 `request.complete` JSON 日志，字段含 `request_id`、`trace_id`、`span_id`、`project_id`、`route`、`method`、`status`、`duration_ms`。

`Provider.Middleware()` 返回组合 middleware。

Run: `go test ./internal/observability -v`
Expected: PASS.

- [ ] **A4. 拆分 config 与 server**

`config.Load(plane)` 默认端口：cp 8080、dp 8081；保留 `PORT` 覆盖。新增 `Plane`、`ServiceName`（cp=`vibebiz-cp`，dp=`vibebiz-dp`）。

`server.New(cfg config.Config, obs *observability.Provider) *echo.Echo`：

- 注册 `obs.Middleware()`。
- `e.GET("/metrics", echo.WrapHandler(obs.MetricsHandler))`。
- 保留 `/health`、CORS、RequestID、Recover。
- 保留统一错误处理注册（从旧 router 移出）。

- [ ] **A5. 实现 admin Repository 与 handler**

`internal/admin/repository.go` 定义 `Overview/User/Sandbox/Quota/AuditEvent/Repository`。

`internal/admin/memory.go` 提供 seed 数据：

- 2 个用户、2 个沙箱、2 条配额、3 条审计。
- `Overview(projectCount)` 返回 `projects=projectCount`、`users=2`、`runningSandboxes=1`、`todayRequests=128`。

`handler/admin.go`：

- `Overview`：从 `ProjectStore.List()` 取数量。
- `Users/Sandboxes/Quotas/Audit`：直接返回 Repository。
- `Projects`：映射 store 项目为 `{id,name,state,owner:"wangh11",createdAt}`。
- 全部使用 `response.OK`。

- [ ] **A6. 拆分 router**

`router/control.go`：

```go
func RegisterControl(e *echo.Echo, repo admin.Repository, projects store.ProjectStore, sandbox *client.Sandbox, obs *observability.Provider) {
    projectHandler := handler.NewProjectHandler(projects, sandbox, obs)
    adminHandler := handler.NewAdminHandler(repo, projects)
    api := e.Group("/cp/api/v1")
    api.GET("/projects", projectHandler.List)
    api.POST("/projects", projectHandler.Create)
    api.GET("/projects/:id", projectHandler.Get)
    adminAPI := api.Group("/admin")
    adminAPI.GET("/overview", adminHandler.Overview)
    adminAPI.GET("/users", adminHandler.Users)
    adminAPI.GET("/projects", adminHandler.Projects)
    adminAPI.GET("/sandboxes", adminHandler.Sandboxes)
    adminAPI.GET("/quotas", adminHandler.Quotas)
    adminAPI.GET("/audit", adminHandler.Audit)
    registerErrorHandler(e)
}
```

`router/data.go`：只注册 `/dp/api/v1` 下的 files/file/chat/preview，并 `registerErrorHandler(e)`。构造 handler 时传入 `obs`：`handler.NewFileHandler(sandbox, obs)`、`NewChatHandler(sandbox, obs)`、`NewPreviewHandler(sandbox, obs)`。

Handler 事件日志要求：`project.create`（CP）、`file.read` / `file.write`（DP）、`chat.start` / `chat.end` / `sse.event`（DP），均带 `project_id`；`obs == nil` 时跳过日志，不影响测试。

`router/errors.go`：把旧 `Register` 中的 `e.HTTPErrorHandler` 逻辑迁移为 `registerErrorHandler`。

Delete `internal/router/router.go`。

- [ ] **A7. 更新 sandbox client 观测**

`client.NewSandbox(baseURL string, obs *observability.Provider)`：

- HTTP transport 使用 `otelhttp.NewTransport(http.DefaultTransport)`。
- 每个方法记录 `sandbox.<operation>` span，属性 `project.id`、`sandbox.operation`、`http.status_code`。
- 输出 `sandbox.call` JSON 日志（开始/结束），含 `project_id`、`operation`、`status`、`duration_ms`。
- `obs == nil` 时退化为原行为，测试可传 nil。

- [ ] **A8. 更新入口**

`cmd/cp/main.go`：

```go
cfg := config.Load("cp")
obs, err := observability.New(context.Background(), cfg.ServiceName, cfg.Plane)
defer obs.Shutdown(context.Background())
projects := store.NewMemoryProjectStore()
sandbox := client.NewSandbox(cfg.SandboxBaseURL, obs)
e := server.New(cfg, obs)
router.RegisterControl(e, admin.NewMemoryRepository(), projects, sandbox, obs)
// 启动与优雅退出沿用旧 main.go
```

`cmd/dp/main.go`：

```go
cfg := config.Load("dp")
obs, err := observability.New(context.Background(), cfg.ServiceName, cfg.Plane)
sandbox := client.NewSandbox(cfg.SandboxBaseURL, obs)
e := server.New(cfg, obs)
router.RegisterData(e, sandbox, obs)
```

Delete `cmd/server/main.go`。

- [ ] **A9. 更新 Dockerfile**

```dockerfile
ARG SERVICE=cp
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/${SERVICE}
```

- [ ] **A10. 更新/新增测试**

- `project_test.go` 改为 CP 路由测试：`/cp/api/v1/projects`。
- `file_test.go`、`chat_test.go` 改为 DP 路由：`/dp/api/v1/...`。
- 新增 `admin_test.go`：六个端点返回信封与预期字段。
- 新增 `prefix_test.go`：CP 对 `/dp/api/v1/projects` 返回 404；DP 对 `/cp/api/v1/admin/overview` 返回 404。
- 新增 `metrics_test.go`：CP/DP `/metrics` 200 且包含 `vibebiz_http_requests_total`。
- 新增 span 测试：用 `sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(recorder)))`，断言 `project.id` 属性。

Run: `go test ./...`
Expected: PASS.

- [ ] **A11. 格式化与静态检查**

```bash
gofmt -w ./cmd ./internal
go vet ./...
go test ./...
```

---

## 工作流 B · Sandbox 观测

**Files:**
- Create: `packages/sandbox/app/observability.py`
- Create: `packages/sandbox/tests/test_observability.py`
- Modify: `packages/sandbox/app/main.py`
- Modify: `packages/sandbox/app/routers/files.py`
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/app/services/agent.py`
- Modify: `packages/sandbox/requirements.txt`
- Modify: `packages/sandbox/pyproject.toml`

**Interfaces:**
- `configure_observability(app: FastAPI) -> None`：JSON 日志、OTel trace、Prometheus `/metrics`、HTTP 中间件。
- `get_tracer() -> Tracer`。
- `SANDBOX_REQUESTS`、`AGENT_RUNS`、`SSE_EVENTS`、`HTTP_REQUESTS`、`HTTP_DURATION` 指标对象。

- [ ] **B1. 添加依赖**

`requirements.txt` 增加：

```text
opentelemetry-api>=1.27,<2.0
opentelemetry-sdk>=1.27,<2.0
opentelemetry-exporter-otlp>=1.27,<2.0
opentelemetry-instrumentation-fastapi>=0.48b0
prometheus-client>=0.21,<1.0
```

同步 `pyproject.toml` dependencies。安装：

```bash
cd packages/sandbox
uv pip install --python .venv/bin/python -r requirements-dev.txt
```

- [ ] **B2. 先写失败测试**

`tests/test_observability.py`：

```python
from fastapi.testclient import TestClient

from app.main import create_app


def test_metrics_endpoint_exposes_project_counter(tmp_path):
    app = create_app()
    client = TestClient(app)
    response = client.get("/metrics")
    assert response.status_code == 200
    assert "vibebiz_http_requests_total" in response.text
```

Run: `.venv/bin/python -m pytest tests/test_observability.py -q`
Expected: FAIL with 404.

- [ ] **B3. 实现 observability.py**

- JSON logging formatter：字段 `time, level, service, plane, request_id, trace_id, span_id, project_id, route, method, status, duration_ms, msg`。
- `OTEL_EXPORTER_OTLP_ENDPOINT` 为空：不创建 exporter，仅设置 noop provider。
- 非空：`TracerProvider(resource=Resource.create({...}))` + `BatchSpanProcessor(OTLPSpanExporter(endpoint=...))`。
- `FastAPIInstrumentor.instrument_app(app, tracer_provider=provider)`。
- `@app.get("/metrics")` 返回 `generate_latest()`，`CONTENT_TYPE_LATEST`。
- HTTP 中间件：提取 `/internal/projects/{project_id}`，记录 HTTP 指标与 `request.complete` 日志；设置当前 span 的 `project.id`。

- [ ] **B4. 业务 span 与指标**

- `files.py`：`workspace.read` / `workspace.write` span，属性 `project.id`、`file.path`、`file.version`。
- `chat.py`：`chat.stream` span，属性 `project.id`；每个 SSE 帧 `SSE_EVENTS.labels(project_id, type).inc()`；结束时 `sse.done=true`。
- `agent.py`：`agent.run` span，属性 `project.id`、`agent.mode`、`agent.status`；`AGENT_RUNS` 计数。
- `SANDBOX_REQUESTS` 在 files/chat handler 中按 operation/status/project_id 计数。

- [ ] **B5. 运行测试**

```bash
.venv/bin/python -m ruff check app tests
.venv/bin/python -m ruff format app tests
.venv/bin/python -m pytest -q
```

Expected: 原 13 项 + 新增观测测试全部通过。

---

## 工作流 C · Web 双客户端

**Files:**
- Modify: `packages/web/src/api/client.ts`
- Modify: `packages/web/src/api/client.test.ts`
- Modify: `packages/web/src/api/projects.ts`
- Modify: `packages/web/src/api/files.ts`
- Modify: `packages/web/src/api/chat.ts`
- Modify: `packages/web/src/components/editor/PreviewPanel.vue`
- Create: `packages/web/src/utils/preview.ts`
- Create: `packages/web/src/utils/preview.test.ts`
- Modify: `packages/web/vite.config.ts`
- Modify: `packages/web/nginx.conf`
- Modify: 各 package `.env.example` 由 E 负责；本工作流只读

**Interfaces:**
- `cpHttp`、`dpHttp` 两个 axios 实例。
- `request<T>(client: AxiosInstance, config: AxiosRequestConfig): Promise<T>`。
- `buildPreviewUrl(projectId: string): string`。

- [ ] **C1. 先写失败测试**

`src/utils/preview.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { buildPreviewUrl } from './preview'

describe('buildPreviewUrl', () => {
  it('uses the DP prefix and encodes the project id', () => {
    expect(buildPreviewUrl('p 1')).toBe('/dp/api/v1/projects/p%201/preview/')
  })
})
```

Run: `npm run test -- preview`
Expected: FAIL because module missing.

- [ ] **C2. 实现 preview 工具**

```ts
export function buildPreviewUrl(projectId: string): string {
  const base = (import.meta.env.VITE_DP_BASE || '/dp/api/v1').replace(/\/$/, '')
  return `${base}/projects/${encodeURIComponent(projectId)}/preview/`
}
```

- [ ] **C3. 拆分 client**

`client.ts`：

```ts
const common = { timeout: 30_000 }
export const cpHttp = axios.create({ ...common, baseURL: import.meta.env.VITE_CP_BASE || '/cp/api/v1' })
export const dpHttp = axios.create({ ...common, baseURL: import.meta.env.VITE_DP_BASE || '/dp/api/v1' })
export async function request<T>(client: AxiosInstance, config: AxiosRequestConfig): Promise<T>
```

`projects.ts` 用 `cpHttp`；`files.ts` 用 `dpHttp`。

`chat.ts` 的 `API_BASE` 改为 `import.meta.env.VITE_DP_BASE || '/dp/api/v1'`。

- [ ] **C4. 更新 PreviewPanel**

`src` 改为 `computed(() => buildPreviewUrl(props.projectId))`；页面展示路径同步改为 DP 前缀。

- [ ] **C5. 更新测试**

`client.test.ts` 保持 `unwrapEnvelope` 测试；新增一个断言：`cpHttp.defaults.baseURL` 以 `/cp/api/v1` 结尾，`dpHttp` 以 `/dp/api/v1` 结尾（通过 `request` mock 或直接断言实例配置）。

- [ ] **C6. 更新代理与 nginx**

`vite.config.ts`：

```ts
proxy: {
  '/cp': { target: env.VITE_CP_PROXY_TARGET || 'http://localhost:8080', changeOrigin: true },
  '/dp': { target: env.VITE_DP_PROXY_TARGET || 'http://localhost:8081', changeOrigin: true },
}
```

不写 `rewrite`。

`nginx.conf`：

```nginx
location /cp/ { proxy_pass http://cp:8080; }
location /dp/ {
    proxy_pass http://dp:8081;
    proxy_buffering off;
    proxy_cache off;
    proxy_read_timeout 3600s;
}
```

- [ ] **C7. 校验**

```bash
npm run format
npm run lint
npm run test
npm run build
```

---

## 工作流 D · ops-web 骨架

**Files:** 全部位于 `packages/ops-web/**`（新建）。

**Interfaces:**
- 基础路径 `/cp/api/v1/admin`。
- 路由：`/`、`/users`、`/projects`、`/sandboxes`、`/quotas`、`/audit`、`/settings`。
- 复用 `{code,message,result}` 信封解包。

- [ ] **D1. 创建项目骨架**

`package.json` 依赖与 web 对齐：`vue`、`vue-router`、`pinia`、`axios`、`primevue`、`@primevue/themes`、`primeicons`；devDeps 对齐 ESLint/Prettier/TypeScript/Vite/vitest/vue-tsc。脚本：`dev/build/preview/test/lint/format/format:check`。dev 端口 5174。

创建 `index.html`、`tsconfig.json`、`vite.config.ts`、`eslint.config.js`、`.prettierrc.json`、`src/main.ts`、`src/App.vue`、`src/env.d.ts`。

`vite.config.ts` 代理 `/cp` 到 `http://localhost:8080`，不重写。

- [ ] **D2. API 客户端与测试**

`src/api/client.ts`：axios 实例 baseURL `import.meta.env.VITE_OPS_API_BASE || '/cp/api/v1/admin'`，`unwrapEnvelope` 与 `request` 复用 web 语义。

`src/api/admin.ts`：`getOverview`、`getUsers`、`getProjects`、`getSandboxes`、`getQuotas`、`getAudit`。

`src/api/client.test.ts`：信封解包成功/失败。

- [ ] **D3. 布局与路由**

- `src/layouts/OpsLayout.vue`：左侧栏（Vibebiz Ops、概览/用户/项目/沙箱节点/配额/审计日志/系统设置）、顶部 `LOCAL` 标识、内容区 `<router-view />`。
- `src/router/index.ts` 注册七个页面，`document.title` 同步。
- `src/styles/global.css`：黑白灰蓝，复用 web 的 CSS 变量命名。
- `src/components/DataTable.vue`：接收 `columns` 与 `rows`，空态/加载态/错误态。

- [ ] **D4. 七个页面**

- `OverviewView.vue`：四张指标卡 + 最近项目表；并行请求 overview/projects。
- `UsersView.vue`、`ProjectsView.vue`、`SandboxesView.vue`、`QuotasView.vue`、`AuditView.vue`：调用对应 API + `DataTable`。
- `SettingsView.vue`：静态展示 CP/DP/sandbox 端口、`OTEL_EXPORTER_OTLP_ENDPOINT` 状态、MySQL 目标，只读。

- [ ] **D5. 容器配置**

`Dockerfile`：Node 构建 + nginx。`nginx.conf`：`/cp/` 原样代理到 `http://cp:8080`，SPA fallback。

- [ ] **D6. 校验**

```bash
cd packages/ops-web
npm install
npm run format
npm run lint
npm run test
npm run build
```

---

## 工作流 E · 集成、配置与文档

**Files:**
- Modify: 根 `Makefile`（薄聚合）
- Create: `packages/{backend,sandbox,web,ops-web}/Makefile`
- Create: `packages/{backend,sandbox,web,ops-web}/.env.example`
- Modify: `scripts/dev.sh`
- Modify: `docker-compose.yml`
- Modify: `scripts/validate.py`
- Modify: `scripts/check_dependencies.py`
- Modify: `scripts/check_quality.py`
- Modify: `README.md`
- Modify: `AGENTS.md`
- Modify: `.ai/AGENTS.md`
- Modify: `.ai/rules/architecture.md`
- Modify: `docs/AI-Coding-Platform-技术设计文档.md`
- Modify: `docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`（仅必要的一致性修正）

- [ ] **E1. Makefile 与 dev.sh**

新增变量 `CP`、`DP`、`OPS_WEB`。目标：

```text
install: install-backend install-sandbox install-web install-ops-web
dev: sandbox + cp + dp + web + ops-web
dev-cp / dev-dp / dev-ops-web
test-backend (go test ./...)
test-ops-web
build: go build ./cmd/cp、./cmd/dp、web build、ops-web build
```

`dev.sh` 启动五个进程，注入：

```text
CP: PORT=8080 OTEL_SERVICE_NAME=vibebiz-cp
DP: PORT=8081 OTEL_SERVICE_NAME=vibebiz-dp
sandbox: OTEL_SERVICE_NAME=vibebiz-sandbox
```

- [ ] **E2. compose 与环境变量**

compose 服务：`sandbox`、`cp`、`dp`、`web`、`ops-web`。

- `cp`/`dp` 共享 `packages/backend` 构建上下文，`build.args.SERVICE=cp|dp`。
- `web` 依赖 `cp`、`dp`，端口 `5173:80`。
- `ops-web` 依赖 `cp`，端口 `5174:80`。
- 观测变量从各 package 的 `.env` 透传；`OTEL_EXPORTER_OTLP_ENDPOINT` 默认空。

`.env.example` 按 package 拆分：backend（端口/CORS/日志/OTel）、sandbox（工作区/LLM/日志/OTel）、web（CP/DP base 与代理）、ops-web（admin base 与代理）。根目录不保留统一 `.env`。

- [ ] **E3. 扩展校验脚本**

`validate.py`：

- backend 必需文件：`cmd/cp/main.go`、`cmd/dp/main.go`；不再要求 `cmd/server/main.go`。
- 新增 `packages/ops-web` 结构检查。
- 契约检查：CP `control.go` 含 `/cp/api/v1` 与 `admin`；DP `data.go` 含 `/dp/api/v1`、`chat`、`preview`，不含 `admin`。
- CP/DP 都含 `/metrics`；sandbox `main.py` 含 `/metrics`。
- web 含 `/cp/api/v1` 与 `/dp/api/v1`；ops-web 含 `/cp/api/v1/admin`。
- `CURRENT_FOCUS.service` 允许值增加 `cp`、`dp`、`ops-web`。

`check_dependencies.py`：

- ops-web 源码不得出现 `/dp/`、`localhost:8000`、`/internal/projects`。
- web/ops-web package.json 不得包含 `@opentelemetry/`、`prom-client`、`pino`、`winston`。
- 保留 AI SDK 禁令。

`check_quality.py`：

- `check_typescript` 与 `check_palette` 遍历 `packages/web/src` 和 `packages/ops-web/src`。
- ESLint 在两个前端目录分别执行。

- [ ] **E4. 文档同步**

- `README`：架构图、目录、端口、五服务命令、API 前缀、可观测性、MySQL 后续。
- 根 `AGENTS.md` 与 `.ai/AGENTS.md`：路由表、端口、命令、观测约束。
- `.ai/rules/architecture.md`：CP/DP 边界、前缀、观测、测试、部署。
- 技术设计文档：增加 CP/DP/ops-web 架构图、MySQL 目标、观测章节；修正旧“网关单进程”描述。
- 所有文档中的旧 `/api/v1` 业务路径改为 `/cp/api/v1` 或 `/dp/api/v1`。

- [ ] **E5. 校验**

```bash
python3 scripts/validate.py
python3 scripts/check_dependencies.py
python3 scripts/check_quality.py
```

---

## 工作流 F · 验证

- [ ] **F1. 全量测试**

```bash
make test
make quality
make validate
make build
```

- [ ] **F2. 前缀与端点冒烟**

```bash
curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/health
curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/health
curl -s -o /dev/null -w '%{http_code}' http://localhost:8000/health

curl -s http://localhost:8080/metrics | grep vibebiz_http_requests_total
curl -s http://localhost:8081/metrics | grep vibebiz_http_requests_total
curl -s http://localhost:8000/metrics | grep vibebiz_http_requests_total

curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/dp/api/v1/projects
curl -s -o /dev/null -w '%{http_code}' http://localhost:8081/cp/api/v1/projects
```

期望：health 200；metrics 有计数；跨平面前缀 404。

- [ ] **F3. 端到端**

1. `POST http://localhost:8080/cp/api/v1/projects` 创建项目。
2. `GET http://localhost:8081/dp/api/v1/projects/{id}/files` 返回文件树。
3. `POST http://localhost:8081/dp/api/v1/projects/{id}/chat` 返回 SSE 并以 `[DONE]` 结束。
4. `GET http://localhost:8080/cp/api/v1/admin/overview` 的项目数增加。
5. `GET http://localhost:5174/` 能看到项目。
6. 日志包含 `project_id`、`trace_id`、`span_id`；配置 OTLP endpoint 后外部接收端能看到 `vibebiz-cp`、`vibebiz-dp`、`vibebiz-sandbox` span。

- [ ] **F4. 视觉检查**

web 首页、编辑器与 ops-web 七个页面在 1440 与 390 宽度无横向溢出；颜色仅黑白灰蓝。

---

## 完成定义

- CP/DP 两个入口可独立启动与构建，前缀隔离正确。
- ops-web 七个页面可用，数据来自 CP。
- CP/DP/sandbox 日志、Trace、Metrics 满足规格。
- 全部测试、质量门禁、构建通过。
- 文档与代码一致。

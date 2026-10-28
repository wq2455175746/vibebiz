# vibebiz-backend AI Agent Guide

## Quick Start

```bash
cat ../../.ai/AGENTS.md
cat ../../.ai/rules/architecture.md
cat ../../.ai/rules/go.md
make init
make test
make quality
```

## Module Map

| 任务 | 目录 | 模式 |
|---|---|---|
| CP 启动与装配 | `cmd/cp/main.go` | 配置 → store/admin/client → `RegisterControl` → 优雅退出 |
| DP 启动与装配 | `cmd/dp/main.go` | 配置 → client → `RegisterData` → 优雅退出 |
| 配置 | `internal/config/` | `Load("cp"|"dp")`，默认端口 8080/8081 |
| HTTP 框架 | `internal/server/` | Echo + CORS + `/health` + `/metrics` |
| 观测 | `internal/observability/` | JSON slog / OTLP Trace / Prometheus |
| CP 路由 | `internal/router/control.go` | `/cp/api/v1` + `/admin/*` |
| DP 路由 | `internal/router/data.go` | `/dp/api/v1` + files/chat/preview |
| 运营后台 | `internal/admin/` | Repository 接口 + 内存 seed |
| 项目 | `internal/handler/project.go` | 创建时先初始化 sandbox 工作区，再落 store |
| 文件 | `internal/handler/file.go` | 透传 sandbox 结果；409 原样返回 |
| SSE | `internal/handler/chat.go` | `bufio.Reader` 逐行读取 + Flush |
| 预览 | `internal/handler/preview.go` | `httputil.ReverseProxy`，FlushInterval=-1 |
| sandbox 客户端 | `internal/client/sandbox.go` | 唯一外部依赖出口，统一解析信封 |
| 存储 | `internal/store/` | 接口 + 内存实现；M3 替换持久化 |
| 模型 | `internal/model/` | JSON 字段与前端 camelCase 对齐 |
| 响应 | `internal/response/` | `OK / Created / Fail` |

## Patterns

### 新增 API

1. 在 `internal/handler/` 新建 handler struct，依赖窄接口。
2. 请求体放在 handler 文件内的小结构体，使用 `c.Bind`。
3. 成功用 `response.OK` / `response.Created`。
4. 外部错误用 `failFromSandbox`，保留 sandbox 的 HTTP 状态。
5. 按归属在 `internal/router/control.go` 或 `data.go` 注册。
6. 在 `*_test.go` 用 `httptest` + fake 覆盖成功 / 400 / 409 / 502。
7. 项目作用域请求必须通过 `obs` 输出 `project_id` 日志并记录 span / metrics。

### 新增观测

1. span / 指标 / 日志统一走 `internal/observability`，不要在 handler 里新建 SDK。
2. `OTEL_EXPORTER_OTLP_ENDPOINT` 为空时不得报错。
3. `/metrics` 必须保持无前缀可访问。

### 新增 sandbox 调用

1. 在 `internal/client/sandbox.go` 增加方法。
2. 内部路径统一 `/internal/projects/{id}/...`。
3. 返回值用 `json.RawMessage` 解到具体结构。
4. `doJSON` 自动把 `code != 0` 转成 `*client.APIError`。
5. handler 只通过窄接口依赖，便于 fake。

## Contracts

- 信封：`{ "code": 0, "message": "ok", "result": ... }`
- 文件读取：`content / version / size / mtime / isBinary`
- 写入：`{ path, content, baseVersion }`，冲突 409
- SSE：逐行透传，不解析 `payload`

## Forbidden

- 在 backend import LLM SDK 或解析 Agent 消息
- 直接读写 `data/workspaces`（工作区归 sandbox）
- 把 sandbox 内部 URL 暴露给前端
- 在 handler 里复制业务逻辑；需要时下沉 service

## Before Committing

```bash
gofmt -w .
go vet ./...
go test ./...
```

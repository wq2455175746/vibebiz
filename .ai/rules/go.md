# Go Rules · packages/backend

## 1. Toolchain

- Go 1.24+，`go.mod` 模块名固定 `vibebiz`。
- 提交前必须：`gofmt -w .`、`go vet ./...`、`go test ./...`。
- 禁止引入 LLM / Agent SDK；AI 能力只在 sandbox。

## 2. Layout

```
cmd/cp/              # 控制面装配与启动
cmd/dp/              # 数据面装配与启动
internal/config/     # 环境变量
internal/server/     # Echo 实例、全局中间件
internal/observability/ # JSON 日志 / OTel Trace / Prometheus
internal/router/     # control.go / data.go / errors.go
internal/handler/    # HTTP 边界：绑定、校验、状态码
internal/admin/      # 运营后台 Repository 接口 + 内存实现
internal/client/     # 外部 HTTP 客户端
internal/store/      # 存储接口 + 实现
internal/model/      # 领域模型
internal/response/   # 统一信封
```

- CP 业务前缀 `/cp/api/v1`，DP 业务前缀 `/dp/api/v1`；禁止无前缀业务 `/api/v1`。
- `/health`、`/metrics` 无前缀；两个入口都必须注册。

- handler 不写业务算法；复杂逻辑下沉 service。
- 单个 handler 文件建议 < 200 行，单个函数 < 50 行。

## 3. Naming

- 包名小写，无下划线。
- 导出类型/函数 PascalCase；未导出 camelCase。
- 错误哨兵：`ErrNotFound` / `ErrConflict`。
- 接口名表达能力：`ProjectInitializer`、`FileClient`，不要 `IProject`。
- 缩写一致：`ID`、`URL`、`HTTP`、`SSE`。

## 4. Errors

```go
if err != nil {
    return fmt.Errorf("load project: %w", err)
}
```

- 必须用 `%w` 包装，保留错误链。
- handler 统一 `response.Fail(c, status, code, message)`。
- 外部错误用 `errors.As` 识别 `*client.APIError`，保留 409 / 404。
- 禁止在 handler 里 `panic`；`main` 以外禁止 `log.Fatal`。
- 禁止吞错：`_ = doSomething()` 仅用于明确无害的清理。

## 5. Context

- IO 函数第一个参数必须是 `ctx context.Context`。
- 不要把 `context.Context` 存进 struct 字段。
- 外部请求统一 `http.NewRequestWithContext`。

## 6. Interfaces

- 接口定义在消费方（handler/client），保持最小方法集。
- 返回具体类型，接收接口。
- 禁止 `interface{}`；需要泛型容器时用 `any` 并加注释说明。
- 测试用 fake 实现窄接口，避免 mock 框架。

## 7. HTTP

- 请求体结构体带 `json` tag；使用 `c.Bind`。
- 校验必填字段后返回 400，不把空字符串传给下游。
- 限制请求体大小：`io.LimitReader`。
- 成功：200 / 201；参数：400；不存在：404；冲突：409；下游：502。
- 响应只通过 `response.OK / Created / Fail`。

## 8. SSE

- 设置 `Content-Type: text/event-stream`、`Cache-Control: no-cache`、`X-Accel-Buffering: no`。
- 逐行 `ReadString('\n')` + `Flush()`，禁止 `io.ReadAll` 后一次性写。
- 不解析 `agent.payload`；只转发字节。
- 客户端断开时立即返回，不再写响应。

## 9. Tests

- 表驱动优先；子测试用 `t.Run`。
- HTTP 用 `httptest.NewRequest` / `httptest.NewRecorder`。
- 外部依赖用窄接口 + fake。
- 覆盖成功、参数错误、下游错误、409。
- 禁止 `time.Sleep` 等异步；需要时用 channel 同步。
- 测试断言 JSON 时先解码，不要匹配转义后的原始字符串。

## 10. Forbidden

- `fmt.Print` / `log.Print`（使用 `log/slog`）。
- 全局可变状态（store 内部除外，必须加锁）。
- 跨层 import：backend 不得 import sandbox 源码。
- 在 backend 解析 Agent 工具调用。
- 硬编码密钥、端口、外部地址（用 config）。
- 无上下文说明的 `TODO` / `FIXME`。

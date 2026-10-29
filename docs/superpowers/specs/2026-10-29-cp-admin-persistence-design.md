# CP 运营数据持久化设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-29 |
| 状态 | 已确认 |
| 范围 | CP admin 用户/配额/审计从内存切到 MySQL；admin 写操作审计落库；概览计数真实化；沙箱列表暂返回空 |
| 关联 | `docs/superpowers/specs/2026-10-28-business-database-design.md`、`docs/superpowers/specs/2026-10-29-cp-project-persistence-design.md` |

## 1. 现状

CP 重启后仍会丢失的运营数据：

- `admin.MemoryRepository.users`：写死 2 个用户
- `admin.MemoryRepository.quotas`：写死 2 条配额
- `admin.MemoryRepository.audit`：写死 3 条审计，且没有真实写入
- `admin.MemoryRepository.sandboxes`：写死 2 个假沙箱
- `admin.MemoryRepository.Overview`：用户数、沙箱数、今日请求写死

项目已由 WIP 的 `MySQLProjectStore` 持久化；Session、项目 meta、通知缓存与已读状态在 Redis，不因 CP 重启丢失。

## 2. 决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | admin 用户、配额、审计改为 MySQL Repository | 对应 `user`、`user_quota`、`ops_audit_log` 已有表 |
| D2 | 沙箱列表返回空 | CP 没有沙箱运行态注册表；不持久化假数据 |
| D3 | 运行中沙箱数固定 0 | 同上 |
| D4 | 今日请求用 Redis 日计数 | Redis 已是 CP 依赖；跨 CP 重启保留，48h TTL |
| D5 | admin 写操作由中间件统一落审计 | 覆盖当前和后续 admin 写接口，避免每个 handler 重复调用 |
| D6 | 审计不记录请求正文 | 避免密码、Token、消息正文等敏感数据入库 |

## 3. admin Repository

接口统一带 `context.Context` 和 `error`：

```go
type Repository interface {
    Overview(ctx context.Context, projectCount int) (Overview, error)
    Users(ctx context.Context) ([]User, error)
    Sandboxes(ctx context.Context) ([]Sandbox, error)
    Quotas(ctx context.Context) ([]Quota, error)
    Audit(ctx context.Context) ([]AuditEvent, error)
    RecordAudit(ctx context.Context, event AuditRecord) error
}
```

`MySQLRepository` 查询：

- Users：`user` LEFT JOIN `user_quota`，`plan_code` 缺省 `free`；`status=1` → `active`，否则 `disabled`
- Quotas：`user_quota` 按 `user_id` 排序；LLM/沙箱返回已用值，存储返回 `storage_used_bytes / 1048576`
- Audit：`ops_audit_log` 倒序，最多 200 条
- Overview：`user` 计数 + 项目数 + 0 沙箱 + Redis 今日请求
- Sandboxes：空切片

`MemoryRepository` 保留为测试 fixture，适配新签名，`RecordAudit` 为 no-op。

## 4. 审计中间件

挂在 `RequireOps` 之后，只记录 POST/PUT/PATCH/DELETE：

- 操作人：`auth.SessionFrom(c)` 的 `UserID`、`Username`
- 动作：按已注册路由映射，例如 `notification.create`、`notification.update`、`notification.publish`、`notification.unpublish`、`notification.delete`；未知路由用 `admin.<method>.<path-slug>`
- 资源：路由参数 `:id`
- 请求：method、path、request_id、trace_id、IP、User-Agent
- 结果：HTTP 状态 < 400 为 `success`，否则 `failed`，记录错误信息
- 写审计失败只记录日志，不改变原接口结果

## 5. 今日请求计数

- 全局 CP 中间件，跳过 `/health`、`/metrics`、OPTIONS
- Redis key：`vibebiz:cp:requests:YYYY-MM-DD`
- `INCR` 后设置 48h TTL
- `admin.Overview` 读取当天计数

## 6. 测试

- admin MySQL Repository：sqlmock 覆盖用户、配额、审计、概览、审计写入
- 审计中间件：成功/失败、动作映射、敏感正文不落库
- 请求计数：miniredis 覆盖跨重启保留、TTL、健康检查跳过
- 现有 router/handler/admin 测试适配新签名
- `make test`、`make validate`

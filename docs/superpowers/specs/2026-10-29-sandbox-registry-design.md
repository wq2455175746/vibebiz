# Sandbox 注册中心与项目粘性调度设计

> 状态：待实现
> 日期：2026-10-29
> 范围：CP / DP / sandbox / MySQL / Redis / ops-web
> 关联：`.ai/rules/architecture.md`、`.ai/rules/sandbox-architecture.md`

## 1. 背景与目标

当前 CP 和 DP 都只配置一个 `SANDBOX_BASE_URL`，无法感知多个 sandbox 实例，也无法在节点下线时切换。本次引入 sandbox 注册中心，解决：

1. sandbox 实例注册、心跳、健康检查与并发上报。
2. 项目首次创建时选择压力最低的健康节点。
3. 项目默认固定在同一节点；节点下线后自动改绑到其他健康节点。
4. ops-web 展示真实 sandbox 实例状态。
5. CP / DP 的文件、聊天、预览、存储统计全部按项目解析节点。

前提：sandbox 是无状态编码服务，项目工作区位于所有 sandbox 共享的 NFS/RWX 目录；当前单机部署也满足该前提。工作区不随节点切换而迁移。

## 2. 核心决策

- **方案 A：项目粘性绑定 + 自动故障转移。**
- sandbox 主动心跳，CP 通过 asynq 定时健康复核。
- 项目表保存 `sandbox_id` 作为粘性提示，不作为强租约。
- 绑定节点在线时始终使用该节点。
- 绑定节点离线、未绑定或实例不存在时，选择新的健康节点并 CAS 更新项目绑定。
- 没有健康节点时返回 `503 sandbox unavailable`，不静默转发到未知节点。
- 旧节点恢复后不抢回项目，保持新绑定，避免抖动。
- 本次不实现 `project_leases`、epoch fencing、项目 flock；同一项目跨节点并发写仍是已知风险，后续硬化。

## 3. 数据模型

### 3.1 新表 `sandbox_instance`

迁移文件：`packages/backend/migrations/00010_sandbox_registry.sql`

```sql
CREATE TABLE `sandbox_instance` (
  `id` bigint NOT NULL AUTO_INCREMENT,
  `public_id` char(36) NOT NULL COMMENT '对外 UUID',
  `name` varchar(128) NOT NULL COMMENT 'sandbox 实例唯一名称',
  `scheme` varchar(8) NOT NULL DEFAULT 'http',
  `host` varchar(255) NOT NULL COMMENT '上报的可访问主机/IP',
  `port` int NOT NULL,
  `status` varchar(16) NOT NULL DEFAULT 'offline' COMMENT 'online/offline',
  `health_status` varchar(16) NOT NULL DEFAULT 'unknown' COMMENT 'unknown/healthy/unhealthy',
  `active_requests` int NOT NULL DEFAULT 0,
  `max_concurrency` int NOT NULL DEFAULT 8,
  `last_heartbeat_at` datetime(3) NULL,
  `last_health_at` datetime(3) NULL,
  `health_error` varchar(255) NOT NULL DEFAULT '',
  `last_selected_at` datetime(3) NULL,
  `created_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  `deleted_at` datetime(3) NULL,
  `extra_p` json NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_sandbox_instance_public_id` (`public_id`),
  UNIQUE KEY `uk_sandbox_instance_name` (`name`),
  KEY `idx_sandbox_instance_status` (`status`, `health_status`, `deleted_at`),
  KEY `idx_sandbox_instance_heartbeat` (`last_heartbeat_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
```

Go model：`packages/backend/internal/dao/model/sandbox.go`

```go
type SandboxInstance struct {
    Base
    PublicID        string     `gorm:"column:public_id"`
    Name            string     `gorm:"column:name"`
    Scheme          string     `gorm:"column:scheme"`
    Host            string     `gorm:"column:host"`
    Port            int        `gorm:"column:port"`
    Status          string     `gorm:"column:status"`
    HealthStatus    string     `gorm:"column:health_status"`
    ActiveRequests  int        `gorm:"column:active_requests"`
    MaxConcurrency  int        `gorm:"column:max_concurrency"`
    LastHeartbeatAt *time.Time `gorm:"column:last_heartbeat_at"`
    LastHealthAt    *time.Time `gorm:"column:last_health_at"`
    HealthError     string     `gorm:"column:health_error"`
    LastSelectedAt  *time.Time `gorm:"column:last_selected_at"`
}
```

### 3.2 `project.sandbox_id`

在同一迁移中追加：

```sql
ALTER TABLE `project`
  ADD COLUMN `sandbox_id` bigint NULL AFTER `local_path`,
  ADD KEY `idx_project_sandbox_id` (`sandbox_id`);
```

- 不加外键，避免软删除与实例更换时约束复杂化。
- 已有项目为 `NULL`，首次按项目解析时自动分配。
- 迁移 Down 先删除 `project.sandbox_id`，再删除 `sandbox_instance`。

## 4. 状态模型

### 4.1 心跳

sandbox 默认每 5 秒调用：

```http
POST /cp/api/v1/internal/sandboxes/heartbeat
X-Sandbox-Token: <token>
Content-Type: application/json

{
  "name": "sandbox-a",
  "scheme": "http",
  "host": "10.0.0.11",
  "port": 8000,
  "activeRequests": 2,
  "maxConcurrency": 8
}
```

返回：

```json
{
  "code": 0,
  "message": "ok",
  "result": { "instanceId": "uuid" }
}
```

处理规则：

- 按 `name` upsert；首次注册生成 `public_id`。
- 更新 `scheme/host/port/active_requests/max_concurrency/last_heartbeat_at`。
- 心跳本身不把 `health_status` 改为 healthy。
- 若当前 `health_status=healthy` 且心跳新鲜，则 `status=online`。
- 新实例初始为 `status=offline, health_status=unknown`，等待健康任务确认。
- 参数校验：`name` 1-128、`host` 非空、`port` 1-65535、`maxConcurrency >= 1`、`activeRequests >= 0`。
- token 不匹配返回 401；CP 未配置 token 时返回 503，拒绝心跳。

### 4.2 健康检查

CP 内 asynq Scheduler 每 10 秒投递 `sandbox:healthcheck`：

1. 清理过期心跳：
   ```sql
   UPDATE sandbox_instance
   SET status = 'offline'
   WHERE deleted_at IS NULL
     AND status = 'online'
     AND last_heartbeat_at < NOW(3) - INTERVAL heartbeat_ttl;
   ```
2. 查询所有 `deleted_at IS NULL` 且 `last_heartbeat_at >= NOW - heartbeat_ttl` 的实例，以及当前 `online` 的实例。
3. 并发上限 4，逐个请求 `scheme://host:port/health`，超时 2 秒。
4. 成功条件：HTTP 2xx 且 JSON `code=0` 且 `result.status="ok"`。
5. 成功：
   - `health_status=healthy`
   - `last_health_at=NOW`
   - `health_error=''`
   - 心跳仍在 TTL 内则 `status=online`，否则 `offline`
6. 失败：
   - `health_status=unhealthy`
   - `status=offline`
   - `last_health_at=NOW`
   - `health_error` 截断到 255 字符

asynq 配置：

- 任务类型：`sandbox:healthcheck`
- Scheduler 周期：`@every 10s`
- `MaxRetry(0)`、`Timeout(15s)`、`Unique(10s)`
- 使用独立 Redis DB：`ASYNQ_REDIS_DB`，默认 `1`
- CP 单副本阶段 Scheduler 与 Worker 同进程启动；多 CP 副本前需改为单 scheduler 或 leader 选举

### 4.3 可调度条件

一个实例可被选中必须同时满足：

```text
deleted_at IS NULL
status = 'online'
health_status = 'healthy'
active_requests < max_concurrency
last_heartbeat_at >= now - heartbeat_ttl
```

## 5. 选节点与项目绑定

### 5.1 选节点 SQL

```sql
SELECT *
FROM sandbox_instance
WHERE deleted_at IS NULL
  AND status = 'online'
  AND health_status = 'healthy'
  AND active_requests < max_concurrency
  AND last_heartbeat_at >= ?
ORDER BY
  (active_requests * 1.0 / GREATEST(max_concurrency, 1)) ASC,
  active_requests ASC,
  COALESCE(last_selected_at, '1970-01-01 00:00:00') ASC,
  id ASC
LIMIT 1;
```

选中后更新 `last_selected_at=NOW(3)`。同等压力下按最久未选择排序，实现轮询效果。

### 5.2 创建项目

1. CP 取最多 3 个候选节点，按压力从低到高尝试。
2. 对候选节点调用 `POST /internal/projects/{id}/init`。
3. 成功后把候选实例内部 `id` 写入 `project.sandbox_id`，再保存项目。
4. 某个候选 init 失败时记录错误，继续尝试下一个候选；全部失败返回 `502 sandbox init failed`。
5. 没有可调度节点返回 `503 sandbox unavailable`。
6. 已存在的项目不重新分配。

### 5.3 DP / CP 请求解析项目节点

新增解析器 `ProjectSandboxResolver`：

1. 按 `project.public_id` 查询项目及其 `sandbox_id`。
2. 若绑定的实例存在、`status=online`、`health_status=healthy`、心跳新鲜：直接使用。
3. 否则在事务中执行：
   - 锁定项目行：`SELECT ... FROM project WHERE public_id=? FOR UPDATE`
   - 若期间绑定已变化，重新判断
   - 选新的健康节点
   - `UPDATE project SET sandbox_id=? WHERE id=?`
   - 提交
4. 若没有健康节点，返回 `ErrSandboxUnavailable`。
5. 项目不存在返回 `ErrNotFound`。
6. 绑定节点恢复后不自动切回。

### 5.4 请求路由

- CP 项目创建：使用选中的节点做 init。
- CP 存储统计：按项目解析节点。
- DP 文件列表、读文件、写文件、聊天、预览：按项目解析节点。
- 预览 URL 由解析出的节点 `baseURL` 生成。
- 节点切换只改变路由，不复制工作区；前提是 NFS 共享目录路径一致。

## 6. 组件与接口

### 6.1 `internal/sandboxregistry`

职责：实例 CRUD、心跳、健康检查、选择、项目绑定解析。

```go
type Instance struct {
    ID              int64
    PublicID        string
    Name            string
    BaseURL         string
    Status          string
    HealthStatus    string
    ActiveRequests  int
    MaxConcurrency  int
    LastHeartbeatAt *time.Time
    LastHealthAt    *time.Time
    HealthError     string
}

type Repository interface {
    UpsertHeartbeat(ctx context.Context, heartbeat Heartbeat) (Instance, error)
    List(ctx context.Context) ([]Instance, error)
    Get(ctx context.Context, id int64) (Instance, error)
    Select(ctx context.Context, now time.Time) (Instance, error)
    MarkStaleOffline(ctx context.Context, now time.Time, ttl time.Duration) error
    UpdateHealth(ctx context.Context, id int64, healthy bool, at time.Time, errMessage string) error
}

type ProjectBindingStore interface {
    GetProjectBinding(ctx context.Context, projectID string) (Binding, error)
    UpdateProjectBinding(ctx context.Context, projectInternalID, oldSandboxID *int64, newSandboxID int64) error
}
```

服务：

- `HeartbeatService`：校验并 upsert。
- `HealthService`：asynq handler。
- `Selector`：选最低压力健康节点。
- `Provisioner`：创建项目时选节点并 init。
- `Resolver`：解析项目节点并在离线时改绑。

### 6.2 `internal/client.Router`

为兼容现有 handler，提供与 `*client.Sandbox` 相同的方法，但内部按项目解析节点：

```go
type SandboxResolver interface {
    ResolveBaseURL(ctx context.Context, projectID string) (string, error)
}

type Router struct {
    resolver SandboxResolver
    obs      *observability.Provider
    clients  sync.Map // baseURL -> *Sandbox
}

func (r *Router) ListFiles(ctx context.Context, id string) ([]FileEntry, error)
func (r *Router) ReadFile(ctx context.Context, id, path string) (*FileContent, error)
func (r *Router) WriteFile(ctx context.Context, id, path, content, baseVersion string) (*WriteResult, error)
func (r *Router) ProjectStats(ctx context.Context, id string) (ProjectStats, error)
func (r *Router) Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error)
func (r *Router) PreviewURL(ctx context.Context, id string) (string, error)
```

`PreviewClient` 接口改为：

```go
type PreviewClient interface {
    PreviewURL(ctx context.Context, id string) (string, error)
}
```

`ProjectInitializer` 改为返回分配的 sandbox：

```go
type ProjectInitializer interface {
    InitProject(ctx context.Context, id string) (sandboxID int64, err error)
}
```

### 6.3 sandbox 侧

新增模块：`packages/sandbox/app/services/registry.py`

- `ActiveRequestCounter`：线程安全计数器。
- `RegistryHeartbeat`：后台协程，定时 POST 心跳。
- FastAPI lifespan 启动/停止心跳协程。
- 新增 ASGI 中间件统计 `/internal/` 请求并发：
  - 请求进入时 +1
  - 响应最后一个 body chunk（`more_body=False`）或 `http.disconnect` 时 -1
  - 覆盖 SSE 长连接，避免流式响应提前减计数
- 心跳请求失败只记录 warning，不阻塞 sandbox 主服务。

## 7. API 契约

### 7.1 内部心跳

```http
POST /cp/api/v1/internal/sandboxes/heartbeat
X-Sandbox-Token: <token>
```

请求：

```json
{
  "name": "sandbox-a",
  "scheme": "http",
  "host": "10.0.0.11",
  "port": 8000,
  "activeRequests": 2,
  "maxConcurrency": 8
}
```

成功：

```json
{ "code": 0, "message": "ok", "result": { "instanceId": "uuid" } }
```

错误：

- `401 40102 invalid sandbox token`
- `400 40007 invalid sandbox heartbeat`
- `503 50303 sandbox registry disabled`

### 7.2 管理端沙箱列表

`GET /cp/api/v1/admin/sandboxes`

```json
{
  "code": 0,
  "message": "ok",
  "result": [
    {
      "id": "uuid",
      "name": "sandbox-a",
      "host": "10.0.0.11",
      "port": 8000,
      "status": "online",
      "healthStatus": "healthy",
      "activeRequests": 2,
      "maxConcurrency": 8,
      "lastHeartbeatAt": "2026-10-29T05:00:00Z",
      "lastHealthAt": "2026-10-29T05:00:02Z",
      "healthError": ""
    }
  ]
}
```

`/admin/overview` 的 `runningSandboxes` 改为在线实例数。

## 8. 配置

### 8.1 CP

| 变量 | 默认 | 说明 |
|---|---|---|
| `SANDBOX_REGISTRY_TOKEN` | 空 | 心跳共享 token；为空时拒绝心跳 |
| `SANDBOX_HEARTBEAT_TTL` | `15s` | 心跳过期时间 |
| `SANDBOX_HEALTH_INTERVAL` | `10s` | asynq 健康检查周期 |
| `SANDBOX_HEALTH_TIMEOUT` | `2s` | 单节点健康请求超时 |
| `ASYNQ_REDIS_DB` | `1` | asynq 独立 Redis DB |
| `ASYNQ_CONCURRENCY` | `4` | 健康检查 worker 并发 |

### 8.2 DP

DP 增加 MySQL 连接，复用 `MYSQL_*` 配置。

### 8.3 sandbox

| 变量 | 默认 | 说明 |
|---|---|---|
| `SANDBOX_NAME` | `<hostname>:<port>` | 实例唯一名称 |
| `SANDBOX_ADVERTISE_SCHEME` | `http` | 上报协议 |
| `SANDBOX_ADVERTISE_HOST` | `127.0.0.1` | 上报可访问地址 |
| `SANDBOX_ADVERTISE_PORT` | `SANDBOX_PORT` / `8000` | 上报端口 |
| `CP_REGISTRY_URL` | 空 | 心跳地址；为空则关闭注册 |
| `SANDBOX_REGISTRY_TOKEN` | 空 | 与 CP 共享 token |
| `SANDBOX_HEARTBEAT_INTERVAL` | `5s` | 心跳周期 |
| `SANDBOX_MAX_CONCURRENCY` | `8` | 上报最大并发 |

本地 `.env.example` 提供占位 token；生产必须显式覆盖。

## 9. 依赖

- Go：新增 `github.com/hibiken/asynq`。
- Python：新增 `httpx>=0.27` 作为运行时依赖。
- 不新增数据库或观测后端。

## 10. 错误处理

| 场景 | 行为 |
|---|---|
| 心跳 token 错误 | 401，不写库 |
| CP 未配置 token | 503，拒绝心跳 |
| 实例心跳过期 | 健康任务标记 offline |
| 健康请求超时/失败 | healthy=false，offline，记录 health_error |
| 创建项目无健康节点 | 503 `sandbox unavailable` |
| 创建项目 init 全部失败 | 502 `sandbox init failed` |
| 项目绑定节点离线 | 自动改绑；失败则 503 |
| DP 解析项目不存在 | 404 |
| 改绑并发冲突 | 重新读取绑定后重试一次 |
| 旧节点恢复 | 不抢回项目 |

## 11. 可观测性

- 心跳接口日志：`sandbox.heartbeat`，字段 `sandbox_name`、`active_requests`、`status`。
- 健康任务日志：`sandbox.healthcheck`，字段 `instance_id`、`healthy`、`duration_ms`、`error`。
- 选节点日志：`sandbox.select`，字段 `project_id`、`sandbox_id`、`active_requests`、`max_concurrency`。
- 改绑日志：`sandbox.rebind`，字段 `project_id`、`old_sandbox_id`、`new_sandbox_id`。
- Metrics 复用 `vibebiz_sandbox_requests_total`；不新增观测后端。
- 不记录 token。

## 12. 测试

### 12.1 Go

- migrations：迁移数量 10；`sandbox_instance` 表和 `project.sandbox_id` 存在；Down 可回滚。
- registry repo：
  - 心跳按 name upsert
  - 列表排序
  - 选择最低压力
  - 过滤 offline/unhealthy/满载/心跳过期
  - 同压力按 `last_selected_at` 轮询
  - 过期心跳标记 offline
  - 健康结果更新状态
- heartbeat handler：
  - token 正确/错误/未配置
  - 参数校验
  - 首次注册生成 public_id
- resolver：
  - 绑定在线直接返回
  - 绑定离线改绑
  - 未绑定自动分配
  - 无节点返回 503
  - 并发改绑 CAS 冲突重试
- client router：
  - 不同项目解析不同 baseURL
  - 同一 baseURL 复用客户端
  - PreviewURL 返回项目节点地址
- CP 项目创建：
  - 保存 sandbox_id
  - init 失败尝试下一节点
  - 无节点返回 503
- DP：
  - 文件/聊天/预览按项目节点路由
  - 节点切换后使用新节点
- admin：
  - 沙箱列表字段
  - runningSandboxes = online 数

### 12.2 Python

- 心跳 payload 正确。
- `CP_REGISTRY_URL` 为空时不启动心跳。
- token header 正确。
- active request 计数覆盖普通请求与 SSE 流。
- 心跳失败不影响 `/health`。

### 12.3 前端

- `AdminSandbox` 新字段类型。
- 沙箱页列配置和状态高亮。
- 空列表与离线状态展示。

## 13. 验收标准

1. 启动两个 sandbox，均能在 ops-web 看到，状态为 online、healthy。
2. 创建项目时选择 `active_requests/max_concurrency` 最低的节点，并写入 `project.sandbox_id`。
3. 项目后续文件、聊天、预览持续命中同一节点。
4. 停止绑定节点后，心跳过期并被标记 offline；下一次请求自动改绑到另一健康节点，项目文件仍可读取。
5. 旧节点恢复后，项目仍使用新节点，不自动切回。
6. 所有节点离线时，新项目创建和项目请求返回 503，不返回错误节点地址。
7. ops-web 显示名称、host、port、状态、健康状态、当前并发、最大并发、最后心跳。
8. `make test`、`make validate` 通过。

## 14. 不在本次范围

- `project_leases`、epoch fencing、旧节点双写防护。
- 项目级 flock 与跨节点写入串行化。
- RWX/NFS 挂载本身与 Kubernetes/Helm。
- sandbox 实例的手工下线、摘除、权重调整 API。
- 多 CP 副本下的 scheduler leader 选举。
- 工作区快照、迁移、回滚。

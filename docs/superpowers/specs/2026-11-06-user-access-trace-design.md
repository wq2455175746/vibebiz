# 用户访问痕迹采集与展示设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-06 |
| 状态 | 设计稿 |
| 范围 | CP / DP 登录与请求埋点、本地日志缓冲、MySQL 批量同步、ops-web 访问日志页 |
| 关联 | `.ai/rules/architecture.md`、`docs/AI-Coding-Platform-技术设计文档.md` |

---

## 1. 背景与目标

当前系统缺少对终端用户访问行为的记录：登录、IP、终端类型、设备码、请求 URL、状态与耗时都无法在运营后台查看。本期先以最小成本落地，数据量不大时全部存 MySQL；后续再考虑同步到消息队列和 Elasticsearch。

目标：

1. 登录成功与失败都记录一条事件，包含用户、IP、终端、设备码、请求信息。
2. CP / DP 所有已登录请求记录访问事件，包含 URL、方法、状态码、耗时、项目上下文。
3. 记录先按天、按用户追加到本地日志文件，每 10 分钟批量同步到 MySQL。
4. ops-web 新增“访问日志”页，可筛选、分页查看。
5. 采集链路可替换：以后把 MySQL sink 换成 MQ / ES sink，不改埋点与日志格式。

采集边界：以 web 用户端接口访问路径为主，记录“谁在什么时间、从哪个 IP / 终端、访问了哪个接口、结果如何”，用于后端查看与统计；不解析请求体和响应内容。

非目标：

- 不解析 IP 地理位置，区域字段本期留空。
- 不记录请求体、聊天正文、密码、Token。
- 不记录运维账号（`ops_user`）的登录与 admin 请求。
- 不引入 nginx 日志采集、Filebeat、Kafka、Elasticsearch。
- 不做实时流式写入，接受最长约 10 分钟的入库延迟。

## 2. 关键决策

| 决策 | 结论 | 理由 |
|---|---|---|
| 主采集链路 | 应用层 `usertrace` | 天然带登录态、工作区、项目等业务字段；dev / prod 一致 |
| nginx 日志 | 本期不用作主链路 | 缺用户身份、登录失败账号、项目上下文；dev 环境不经过 nginx |
| 事件表 | 单表 `user_access_log` | 登录与请求共用，`event_type` 区分，ops-web 一页展示 |
| 区域 | 字段预留，本期为空 | 用户确认先只存 IP |
| 设备码 | 前端生成 UUID + UA 解析 | 服务端无法可靠生成设备标识；UA 可解析终端信息 |
| 日志文件 | `USER_TRACE_DIR/<plane>/<date>/<userId>.log` | CP/DP 各自本地写、各自同步，不依赖共享盘 |
| 同步方式 | 进程内 10 分钟 ticker + 幂等批量插入 | DP 没有 asynq worker，避免引入额外调度依赖 |
| 未来扩展 | `Sink` 接口 | MySQL → MQ / ES 只替换 sink |

## 3. 数据模型

### 3.1 迁移

新增 `packages/backend/migrations/00024_user_access_log.sql`，包含 goose Up / Down。

### 3.2 表结构

```sql
CREATE TABLE `user_access_log` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `event_id` char(36) NOT NULL COMMENT '事件UUID，同步幂等键',
  `event_type` varchar(16) NOT NULL COMMENT '事件类型：login/request',
  `user_id` bigint UNSIGNED DEFAULT NULL COMMENT '用户id，登录失败未知用户时为空',
  `username` varchar(64) NOT NULL DEFAULT '' COMMENT '用户名快照',
  `workspace_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前工作区id',
  `project_id` varchar(64) NOT NULL DEFAULT '' COMMENT '项目公开id',
  `ip` varchar(64) NOT NULL DEFAULT '' COMMENT '客户端IP',
  `region_country` varchar(64) NOT NULL DEFAULT '' COMMENT '国家，本期留空',
  `region_province` varchar(64) NOT NULL DEFAULT '' COMMENT '省份，本期留空',
  `region_city` varchar(64) NOT NULL DEFAULT '' COMMENT '城市，本期留空',
  `device_id` varchar(64) NOT NULL DEFAULT '' COMMENT '前端设备码',
  `device_type` varchar(32) NOT NULL DEFAULT '' COMMENT 'desktop/mobile/tablet/bot/other',
  `os` varchar(64) NOT NULL DEFAULT '' COMMENT '操作系统',
  `browser` varchar(64) NOT NULL DEFAULT '' COMMENT '浏览器',
  `user_agent` varchar(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `referer` varchar(1024) NOT NULL DEFAULT '' COMMENT '来源页',
  `request_method` varchar(16) NOT NULL DEFAULT '' COMMENT 'HTTP方法',
  `request_path` varchar(512) NOT NULL DEFAULT '' COMMENT '原始请求路径',
  `request_query` varchar(1024) NOT NULL DEFAULT '' COMMENT '脱敏后的query',
  `route` varchar(255) NOT NULL DEFAULT '' COMMENT '路由模板',
  `status_code` int NOT NULL DEFAULT 0 COMMENT '响应状态码',
  `duration_ms` int NOT NULL DEFAULT 0 COMMENT '耗时毫秒',
  `request_id` varchar(64) NOT NULL DEFAULT '' COMMENT '请求id',
  `trace_id` varchar(64) NOT NULL DEFAULT '' COMMENT 'trace id',
  `login_result` varchar(16) NOT NULL DEFAULT '' COMMENT 'success/failed，仅登录事件',
  `error_message` varchar(255) NOT NULL DEFAULT '' COMMENT '登录失败原因',
  `event_at` datetime(3) NOT NULL COMMENT '事件发生时间（毫秒）',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '入库时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_access_log_event_id` (`event_id`),
  KEY `idx_user_access_log_event_at` (`event_at`),
  KEY `idx_user_access_log_user_time` (`user_id`, `event_at`),
  KEY `idx_user_access_log_type_time` (`event_type`, `event_at`),
  KEY `idx_user_access_log_ip` (`ip`),
  KEY `idx_user_access_log_device` (`device_id`),
  KEY `idx_user_access_log_project` (`project_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户访问痕迹日志表';
```

### 3.3 GORM 模型

新增 `packages/backend/internal/dao/model/trace.go`：

```go
type UserAccessLog struct {
    ID              int64     `gorm:"column:id;primaryKey;autoIncrement"`
    EventID         string    `gorm:"column:event_id"`
    EventType       string    `gorm:"column:event_type"`
    UserID          *int64    `gorm:"column:user_id"`
    Username        string    `gorm:"column:username"`
    WorkspaceID     int64     `gorm:"column:workspace_id"`
    ProjectID       string    `gorm:"column:project_id"`
    IP              string    `gorm:"column:ip"`
    RegionCountry   string    `gorm:"column:region_country"`
    RegionProvince  string    `gorm:"column:region_province"`
    RegionCity      string    `gorm:"column:region_city"`
    DeviceID        string    `gorm:"column:device_id"`
    DeviceType      string    `gorm:"column:device_type"`
    OS              string    `gorm:"column:os"`
    Browser         string    `gorm:"column:browser"`
    UserAgent       string    `gorm:"column:user_agent"`
    Referer         string    `gorm:"column:referer"`
    RequestMethod   string    `gorm:"column:request_method"`
    RequestPath     string    `gorm:"column:request_path"`
    RequestQuery    string    `gorm:"column:request_query"`
    Route           string    `gorm:"column:route"`
    StatusCode      int       `gorm:"column:status_code"`
    DurationMs      int       `gorm:"column:duration_ms"`
    RequestID       string    `gorm:"column:request_id"`
    TraceID         string    `gorm:"column:trace_id"`
    LoginResult     string    `gorm:"column:login_result"`
    ErrorMessage    string    `gorm:"column:error_message"`
    EventAt         time.Time `gorm:"column:event_at"`
    CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (UserAccessLog) TableName() string { return "user_access_log" }
```

表为追加写日志表，不使用软删除与 `updated_at`/`extra_p`。

## 4. `internal/usertrace` 模块

### 4.1 事件结构

```go
type Event struct {
    EventID        string    `json:"eventId"`
    EventType      string    `json:"eventType"` // login | request
    UserID         int64     `json:"userId"`
    Username       string    `json:"username"`
    WorkspaceID    int64     `json:"workspaceId"`
    ProjectID      string    `json:"projectId"`
    IP             string    `json:"ip"`
    RegionCountry  string    `json:"regionCountry"`
    RegionProvince string    `json:"regionProvince"`
    RegionCity     string    `json:"regionCity"`
    DeviceID       string    `json:"deviceId"`
    DeviceType     string    `json:"deviceType"`
    OS             string    `json:"os"`
    Browser        string    `json:"browser"`
    UserAgent      string    `json:"userAgent"`
    Referer        string    `json:"referer"`
    RequestMethod  string    `json:"requestMethod"`
    RequestPath    string    `json:"requestPath"`
    RequestQuery   string    `json:"requestQuery"`
    Route          string    `json:"route"`
    StatusCode     int       `json:"statusCode"`
    DurationMs     int       `json:"durationMs"`
    RequestID      string    `json:"requestId"`
    TraceID        string    `json:"traceId"`
    LoginResult    string    `json:"loginResult"`
    ErrorMessage   string    `json:"errorMessage"`
    EventAt        time.Time `json:"eventAt"`
}
```

`UserID=0` 表示未知用户，写库时转为 `NULL`。

### 4.2 文件布局

```
USER_TRACE_DIR/
└── <plane>/                     # cp 或 dp
    └── 2026-11-06/
        ├── 1.log                # 用户 id=1
        ├── anonymous.log        # 登录失败且用户未知
        └── 1.1730940000000000.syncing   # 同步中的快照
```

- 文件名只用用户 id 或 `anonymous`，避免用户名进入路径。
- 目录按 `eventAt` 的本地日期划分。
- 每行一条 JSON，UTF-8，单行以 `\n` 结尾。
- 追加使用 `O_CREATE|O_APPEND|O_WRONLY`，每次写入打开再关闭，保证与同步重命名不冲突。

### 4.3 Recorder

```go
type Recorder struct {
    dir    string
    plane  string
    logger *slog.Logger
    now    func() time.Time
}
```

- `Append(event Event)`：补齐 `EventID`（UUID）、`EventAt`、终端字段；序列化为单行 JSON 后追加。
- 每进程按目录加互斥锁，避免并发打开写错位。
- 写日志失败只记 `slog.Warn`，不影响业务请求。
- `Append` 对 `nil` Recorder 安全返回，便于测试和关闭采集；关闭采集时构造 no-op recorder。

### 4.4 字段约束

| 字段 | 处理 |
|---|---|
| `request_query` | 对 `password/passwd/token/access_token/secret/api_key/key/authorization` 的 value 替换为 `***`；超长截断到 1024 |
| `user_agent` | 截断到 512 |
| `referer` | 截断到 1024 |
| `request_path` | 截断到 512 |
| `error_message` | 只写通用原因，不写底层错误细节；截断到 255 |
| `route` | 使用 `c.Path()` 模板，不写原始 URL |
| `device_id` | 只允许 `[A-Za-z0-9_-]`，最长 64，否则置空 |

### 4.5 Sink 与 MySQL 同步

```go
type Sink interface {
    Write(ctx context.Context, events []Event) error
}
```

`MySQLSink`：

- 每批最多 500 条，单事务写入。
- 使用 `clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}`，重复事件直接忽略。
- `UserID=0` 写 `NULL`。

### 4.6 Syncer

```go
type Syncer struct {
    dir      string
    plane    string
    sink     Sink
    interval time.Duration
    logger   *slog.Logger
    now      func() time.Time
}
```

流程：

1. 启动后先执行一次同步，之后每 `interval`（默认 10 分钟）执行。
2. 扫描 `USER_TRACE_DIR/<plane>/<所有日期>/` 下的 `*.log` 与 `*.syncing`。
3. 对每个 `*.log` 先原子重命名为 `<name>.<unixnano>.syncing`；新写入会落到新的 `*.log`。
4. 读取 `*.syncing`，逐行解析；坏行跳过并计数。
5. 按 500 条一批调用 `sink.Write`；全部成功后删除快照文件。
6. 任一批失败则保留该 `*.syncing`，下次重试；`event_id` 唯一键保证不会重复入库。
7. 同一时刻只允许一个同步任务运行。
8. 进程退出时取消 ticker；不做强制最终同步，残留文件下次启动继续处理。

## 5. 采集点

### 5.1 CP / DP 请求中间件

新增 `usertrace.Middleware(recorder, logger)`，在 CP / DP 的 `server.New` 之后、业务路由之前注册：

```go
e.Use(usertrace.Middleware(recorder, logger))
```

行为：

1. 跳过 `OPTIONS`、`/health`、`/metrics`。
2. 执行 `next(c)` 后读取 `auth.SessionFrom(c)`；没有会话说明未登录或鉴权失败，跳过。
3. 只记录 `session.Realm == auth.RealmUser` 的请求；admin / ops 不记录。
4. 状态码：`next` 返回非空错误时，若为 `*echo.HTTPError` 取 `Code`，否则记 500；无错误时取 `c.Response().Status`。
5. `project_id` 仅在路由包含 `/projects/:id` 时取 `c.Param("id")`，其他路由为空。
6. `workspace_id` 取 `session.CurrentWorkspaceID`。
7. `request_id` 优先响应头，其次请求头；`trace_id` 取当前 span。
8. 耗时从进入中间件到 `next` 返回。

中间件在 CP、DP 各注册一次。CP 记录登录之外的 `/cp/api/v1/*` 用户请求；DP 记录 `/dp/api/v1/*` 用户请求。SSE 请求在流结束后落一条事件。

### 5.2 登录埋点

`handler.AuthHandler` 新增 `Trace *usertrace.Recorder` 字段，在 `LoginUser` 中显式记录：

- 请求体绑定失败：不记录（不是有效登录尝试）。
- 成功：`event_type=login`、`login_result=success`、`status_code=200`、用户 id / username / 当前工作区。
- 失败：`event_type=login`、`login_result=failed`、`status_code` 对应 401/403/503、`error_message` 使用统一原因：
  - `invalid credentials`
  - `account disabled`
  - `no workspace available`
  - `session store unavailable`
- 失败时用户 id 为空，`username` 记录提交的 `identifier`。
- 登录事件同样记录 IP、UA、设备码、终端解析、请求路径、request id、trace id。
- `LoginOps` 不写访问日志。

### 5.3 终端解析

新增 `internal/usertrace/useragent.go`，纯函数 `ParseUserAgent(ua string) DeviceInfo`：

| 字段 | 取值 |
|---|---|
| `device_type` | `bot` / `tablet` / `mobile` / `desktop` / `other` |
| `os` | `Windows` / `macOS` / `iOS` / `Android` / `Linux` / `other` |
| `browser` | `Edge` / `Chrome` / `Safari` / `Firefox` / `other` |

规则：Bot/Spider/Crawler → bot；iPad 或 Android 且不含 Mobile → tablet；iPhone/Android/Mobile → mobile；空 UA → other；其余 desktop。浏览器按 Edge → Chrome → Firefox → Safari 的顺序匹配。

## 6. 前端设备码

### 6.1 web 生成设备码

新增 `packages/web/src/utils/device.ts`：

- localStorage key：`vibebiz.deviceId`。
- 首次读取生成 UUID：优先 `crypto.randomUUID()`，不可用时用随机串回退。
- 导出 `getDeviceId(): string`，进程内缓存。

### 6.2 请求头

修改 `packages/web/src/api/client.ts`：

- 为 `cpHttp` 与 `dpHttp` 添加 request 拦截器，统一设置 `X-Device-Id`。
- 登录请求也走 `cpHttp`，因此自动带上。

修改 `packages/backend/internal/server/server.go`：

- `CORSConfig.AllowHeaders` 增加 `X-Device-Id`。

ops-web 不需要设备码，不改造。

## 7. 查询接口

新增 `GET /cp/api/v1/admin/access-logs`，注册在 `adminProtected`（`authn.RequireOps` + 审计中间件）内。

查询参数：

| 参数 | 说明 |
|---|---|
| `page` | 默认 1 |
| `pageSize` | 默认 10，最大 100 |
| `keyword` | 匹配 `username` / `ip` / `request_path` / `device_id` / `project_id` |
| `eventType` | `login` / `request`，空表示全部 |
| `from` | 起始时间，支持 `YYYY-MM-DD` 或 RFC3339 |
| `to` | 结束时间，支持 `YYYY-MM-DD` 或 RFC3339 |

返回：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "logs": [
      {
        "id": "1",
        "eventType": "request",
        "userId": 1,
        "username": "wanghg11",
        "workspaceId": 1,
        "projectId": "abc",
        "ip": "127.0.0.1",
        "region": "",
        "deviceId": "uuid",
        "deviceType": "desktop",
        "os": "macOS",
        "browser": "Chrome",
        "userAgent": "...",
        "method": "GET",
        "path": "/cp/api/v1/projects",
        "query": "",
        "route": "/cp/api/v1/projects",
        "statusCode": 200,
        "durationMs": 12,
        "loginResult": "",
        "errorMessage": "",
        "eventAt": "2026-11-06T10:00:00+08:00"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 10
  }
}
```

- 排序：`event_at DESC, id DESC`。
- `region` 由非空的国家/省/市拼接，本期为空字符串。
- 查询实现放在 `internal/usertrace/repository.go`，与 sink 共用 `*gorm.DB`。

## 8. ops-web

### 8.1 路由与导航

- `packages/ops-web/src/views/AccessLogsView.vue`
- 路由：`/access-logs`，name `access-logs`，title “访问日志”
- 导航插入在“审计日志”之前，图标 `pi pi-eye`

### 8.2 页面

- 顶部：标题、说明、总数。
- 筛选栏：关键字输入、事件类型下拉（全部/登录/请求）、起止日期、搜索按钮。
- 表格列：时间、类型、用户、IP、请求（方法 + 路径）、状态、耗时、终端（OS / 浏览器 / 设备类型）、设备码、项目。
- 登录行类型显示“登录”，请求行显示“请求”；失败状态码使用蓝色加粗标记（遵循仓库黑/白/灰/蓝调色板）。
- 复用 `DataTable` / `TablePagination`；分页与关键字沿用 `useAdminPage`，事件类型与时间范围通过扩展查询参数传入。

### 8.3 API 与类型

`packages/ops-web/src/api/admin.ts` 新增 `getAccessLogs(query)`，`packages/ops-web/src/types/index.ts` 新增 `AccessLog`、`AccessLogQuery`。

## 9. 配置

各 package `.env.example` 与 `config.Config` 新增：

| 变量 | 默认 | 说明 |
|---|---|---|
| `USER_TRACE_ENABLED` | `true` | 是否采集 |
| `USER_TRACE_DIR` | `../../data/user-trace` | 日志根目录 |
| `USER_TRACE_SYNC_INTERVAL` | `10m` | 同步间隔 |

- CP 使用 `<dir>/cp/...`，DP 使用 `<dir>/dp/...`。
- 关闭采集时 Recorder 为 no-op，不同步、不建目录。
- `data/user-trace` 加入 `.gitignore`。

## 10. 启动与装配

CP：

1. `usertrace.NewMySQLRepository(db)` 同时实现 `Sink` 与查询仓库。
2. 创建 `Recorder`，注册请求中间件。
3. 注入 `AuthHandler.Trace`。
4. 启动 `Syncer` goroutine，随进程退出取消。
5. 新增 `handler.NewAccessLogHandler(repo)`，在 `router.RegisterControl` 的 admin 保护组注册 `GET /access-logs`。

DP：

1. 创建 `Recorder` 与 `Syncer`，注册请求中间件。
2. 不注册查询接口、不做登录埋点。

## 11. 测试

| 层 | 用例 |
|---|---|
| usertrace | 文件路径按 plane/日期/用户拆分；追加内容为合法单行 JSON；query 脱敏；字段截断；device_id 非法置空 |
| usertrace | UA 解析表驱动：Chrome/macOS、Safari/iOS、Edge/Windows、Firefox/Linux、Android 手机、iPad、Bot、空 UA |
| usertrace | Syncer：重命名快照、坏行跳过、批量大小、成功后删除、失败保留、重复 event_id 幂等 |
| usertrace | 中间件：已登录用户记录；匿名跳过；admin 跳过；OPTIONS/health/metrics 跳过；项目 id 只在项目路由提取 |
| handler | 登录成功写事件；invalid credentials / disabled / no workspace 写 failed；ops 登录不写 |
| admin | `access-logs` 分页、关键字、eventType、时间范围、排序 |
| migrations | `user_access_log` 加入 schema 覆盖；迁移命名与 Up/Down 校验 |
| model | `UserAccessLog.TableName()` |
| web | 设备码生成稳定、请求头带 `X-Device-Id` |
| ops-web | `getAccessLogs` 解包、路由与导航包含“访问日志” |

## 12. 演进

- 消息队列 / Elasticsearch：新增实现 `usertrace.Sink`，同步器逻辑不变；MySQL sink 可保留做双写或降级。
- 区域解析：在 Recorder 内接 GeoIP provider，填充 `region_*`；表结构不变。
- nginx 补充源：后续可开 JSON access log，用独立采集器写入同一张表或 ES，`event_id` 需使用 nginx request id 派生的唯一值。
- 数据保留：后续按 `event_at` 做分区或归档到 ES，本期不清理。

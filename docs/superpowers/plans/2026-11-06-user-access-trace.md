# 用户访问痕迹采集与展示实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 记录 web 用户登录与接口访问路径，先写按天/按用户的本地 JSONL，每 10 分钟批量入库 MySQL，并在 ops-web 提供可筛选的访问日志页。

**Architecture:** 新增 `internal/usertrace` 包，包含 Recorder（本地追加）、Syncer（定时批量入库）、MySQL Repository（写入与查询）、Echo 中间件与 UA 解析。CP/DP 各自写本地 plane 子目录并各自同步，MySQL 用 `event_id` 唯一键保证幂等。ops-web 通过 CP admin 只读接口展示。

**Tech Stack:** Go 1.24、Echo、GORM MySQL、goose、slog、Vue 3 + PrimeVue + Vitest、Axios。

**Spec:** `docs/superpowers/specs/2026-11-06-user-access-trace-design.md`

## Global Constraints

- 只记录 web 用户（`auth.RealmUser`）；不记录 ops 运维登录与 `/admin/*` 请求。
- 不记录请求体、响应体、密码、Token、聊天正文。
- 区域字段本期全部留空，只存 IP。
- 采集粒度是接口路径；不解析响应内容。
- 日志目录按 `<plane>/<YYYY-MM-DD>/<userId>.log` 拆分；`userId=0` 使用 `anonymous.log`。
- MySQL 为唯一存储；`event_id` 唯一键保证同步幂等。
- 默认 10 分钟同步一次；单批最多 500 条。
- 不引入新的数据库、消息队列或 Elasticsearch。
- 不修改 sandbox。
- 每个任务结束运行对应测试；**不要创建 git commit，除非用户在当前会话明确要求。** 任务收尾以“测试通过 + 变更文件清单”为检查点。
- 代码风格遵循 `.ai/rules/go.md` 与 `.ai/rules/typescript.md`，不写无意义注释。

---

## 文件结构

### 后端新增

| 文件 | 职责 |
|---|---|
| `packages/backend/migrations/00024_user_access_log.sql` | 访问日志表 |
| `packages/backend/internal/dao/model/trace.go` | GORM 模型 |
| `packages/backend/internal/usertrace/event.go` | 事件类型与常量 |
| `packages/backend/internal/usertrace/useragent.go` | UA 解析 |
| `packages/backend/internal/usertrace/request.go` | 请求信息提取与 query 脱敏 |
| `packages/backend/internal/usertrace/recorder.go` | 本地 JSONL 追加 |
| `packages/backend/internal/usertrace/repository.go` | MySQL 写入与查询 |
| `packages/backend/internal/usertrace/syncer.go` | 定时批量同步 |
| `packages/backend/internal/usertrace/middleware.go` | 已登录请求埋点 |
| `packages/backend/internal/handler/access_log.go` | admin 查询接口 |

### 后端修改

| 文件 | 改动 |
|---|---|
| `packages/backend/internal/config/config.go` | 新增 3 个配置项 |
| `packages/backend/internal/handler/auth.go` | 登录成功/失败埋点 |
| `packages/backend/internal/server/server.go` | CORS 放行 `X-Device-Id` |
| `packages/backend/internal/router/control.go` | 注册 `GET /admin/access-logs` |
| `packages/backend/cmd/cp/main.go` | Recorder、Syncer、中间件、handler 装配 |
| `packages/backend/cmd/dp/main.go` | Recorder、Syncer、中间件装配 |
| `packages/backend/.env.example` | 新增 3 个配置项占位 |

### 前端新增/修改

| 文件 | 改动 |
|---|---|
| `packages/web/src/utils/device.ts` | 生成/读取 deviceId |
| `packages/web/src/utils/device.test.ts` | 设备码测试 |
| `packages/web/src/api/client.ts` | 请求头注入 deviceId |
| `packages/ops-web/src/views/AccessLogsView.vue` | 访问日志页 |
| `packages/ops-web/src/api/admin.ts` | `getAccessLogs` |
| `packages/ops-web/src/types/index.ts` | `AccessLog` 类型 |
| `packages/ops-web/src/router/routes.ts` | 新路由 |
| `packages/ops-web/src/router/navigation.ts` | 新导航项 |
| `packages/ops-web/src/router/navigation.test.ts` | 更新期望 |

---

## Task 1: 迁移、模型与配置

**Files:**
- Create: `packages/backend/migrations/00024_user_access_log.sql`
- Create: `packages/backend/internal/dao/model/trace.go`
- Modify: `packages/backend/internal/dao/model/table_test.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Produces: `model.UserAccessLog`（字段与迁移一致）
- Produces: `config.Config.UserTraceEnabled bool`、`UserTraceDir string`、`UserTraceSyncInterval time.Duration`

- [ ] **Step 1: 写迁移**

创建 `packages/backend/migrations/00024_user_access_log.sql`：

```sql
-- user access trace log

-- +goose Up

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

-- +goose Down

DROP TABLE IF EXISTS `user_access_log`;
```

- [ ] **Step 2: 写模型**

创建 `packages/backend/internal/dao/model/trace.go`：

```go
package model

import "time"

type UserAccessLog struct {
	ID             int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	EventID        string    `gorm:"column:event_id" json:"eventId"`
	EventType      string    `gorm:"column:event_type" json:"eventType"`
	UserID         *int64    `gorm:"column:user_id" json:"userId"`
	Username       string    `gorm:"column:username" json:"username"`
	WorkspaceID    int64     `gorm:"column:workspace_id" json:"workspaceId"`
	ProjectID      string    `gorm:"column:project_id" json:"projectId"`
	IP             string    `gorm:"column:ip" json:"ip"`
	RegionCountry  string    `gorm:"column:region_country" json:"regionCountry"`
	RegionProvince string    `gorm:"column:region_province" json:"regionProvince"`
	RegionCity     string    `gorm:"column:region_city" json:"regionCity"`
	DeviceID       string    `gorm:"column:device_id" json:"deviceId"`
	DeviceType     string    `gorm:"column:device_type" json:"deviceType"`
	OS             string    `gorm:"column:os" json:"os"`
	Browser        string    `gorm:"column:browser" json:"browser"`
	UserAgent      string    `gorm:"column:user_agent" json:"userAgent"`
	Referer        string    `gorm:"column:referer" json:"referer"`
	RequestMethod  string    `gorm:"column:request_method" json:"requestMethod"`
	RequestPath    string    `gorm:"column:request_path" json:"requestPath"`
	RequestQuery   string    `gorm:"column:request_query" json:"requestQuery"`
	Route          string    `gorm:"column:route" json:"route"`
	StatusCode     int       `gorm:"column:status_code" json:"statusCode"`
	DurationMs     int       `gorm:"column:duration_ms" json:"durationMs"`
	RequestID      string    `gorm:"column:request_id" json:"requestId"`
	TraceID        string    `gorm:"column:trace_id" json:"traceId"`
	LoginResult    string    `gorm:"column:login_result" json:"loginResult"`
	ErrorMessage   string    `gorm:"column:error_message" json:"errorMessage"`
	EventAt        time.Time `gorm:"column:event_at" json:"eventAt"`
	CreatedAt      time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
}

func (UserAccessLog) TableName() string { return "user_access_log" }
```

- [ ] **Step 3: 写失败测试**

在 `packages/backend/internal/dao/model/table_test.go` 的 `cases` 中加入：

```go
{&UserAccessLog{}, "user_access_log"},
```

在 `packages/backend/migrations/migrations_test.go` 的 `TestInitialSchemaCoversAllBusinessTables` 表列表中加入：

```go
"user_access_log",
```

再追加：

```go
func TestUserAccessLogMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00024_user_access_log.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `user_access_log`", "event_id", "event_type",
		"uk_user_access_log_event_id", "idx_user_access_log_user_time",
		"DROP TABLE IF EXISTS `user_access_log`", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("user access log migration missing %q", want)
		}
	}
}
```

- [ ] **Step 4: 写配置测试**

在 `packages/backend/internal/config/config_test.go` 追加：

```go
func TestLoadUserTraceConfig(t *testing.T) {
	t.Setenv("USER_TRACE_ENABLED", "false")
	t.Setenv("USER_TRACE_DIR", "/tmp/user-trace")
	t.Setenv("USER_TRACE_SYNC_INTERVAL", "5m")

	cfg := Load("cp")
	if cfg.UserTraceEnabled {
		t.Fatal("user trace enabled = true, want false")
	}
	if cfg.UserTraceDir != "/tmp/user-trace" {
		t.Fatalf("user trace dir = %q", cfg.UserTraceDir)
	}
	if cfg.UserTraceSyncInterval != 5*time.Minute {
		t.Fatalf("user trace interval = %v", cfg.UserTraceSyncInterval)
	}
}

func TestLoadUserTraceDefaults(t *testing.T) {
	t.Setenv("USER_TRACE_ENABLED", "")
	t.Setenv("USER_TRACE_DIR", "")
	t.Setenv("USER_TRACE_SYNC_INTERVAL", "")

	cfg := Load("cp")
	if !cfg.UserTraceEnabled {
		t.Fatal("user trace enabled = false, want true")
	}
	if cfg.UserTraceDir != "../../data/user-trace" {
		t.Fatalf("user trace dir = %q", cfg.UserTraceDir)
	}
	if cfg.UserTraceSyncInterval != 10*time.Minute {
		t.Fatalf("user trace interval = %v", cfg.UserTraceSyncInterval)
	}
}
```

- [ ] **Step 5: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/dao/model ./migrations ./internal/config
```

Expected: 模型/迁移测试因缺少类型或表名失败，配置测试因字段不存在编译失败。

- [ ] **Step 6: 实现配置字段**

在 `packages/backend/internal/config/config.go` 的 `Config` 结构体加入：

```go
	UserTraceEnabled      bool
	UserTraceDir          string
	UserTraceSyncInterval time.Duration
```

在 `Load` 返回的结构体字面量中加入：

```go
		UserTraceEnabled:      getBool("USER_TRACE_ENABLED", true),
		UserTraceDir:          getString("USER_TRACE_DIR", "../../data/user-trace"),
		UserTraceSyncInterval: getDuration("USER_TRACE_SYNC_INTERVAL", 10*time.Minute),
```

- [ ] **Step 7: 更新 .env.example**

在 `packages/backend/.env.example` 末尾加入：

```bash
# ---- user access trace ----
USER_TRACE_ENABLED=true
USER_TRACE_DIR=../../data/user-trace
USER_TRACE_SYNC_INTERVAL=10m
```

- [ ] **Step 8: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/dao/model ./migrations ./internal/config
```

Expected: PASS。

- [ ] **Step 9: 检查点**

记录变更文件；不创建 commit（除非用户明确要求）。

---

## Task 2: 事件结构、UA 解析与请求信息提取

**Files:**
- Create: `packages/backend/internal/usertrace/event.go`
- Create: `packages/backend/internal/usertrace/useragent.go`
- Create: `packages/backend/internal/usertrace/request.go`
- Test: `packages/backend/internal/usertrace/useragent_test.go`
- Test: `packages/backend/internal/usertrace/request_test.go`

**Interfaces:**
- Produces: `usertrace.Event`、`usertrace.EventLogin`、`usertrace.EventRequest`
- Produces: `usertrace.Appender interface { Append(Event) }`
- Produces: `usertrace.DeviceInfo{DeviceType, OS, Browser string}`
- Produces: `usertrace.ParseUserAgent(ua string) DeviceInfo`
- Produces: `usertrace.RequestInfo`、`usertrace.CaptureRequest(c echo.Context) RequestInfo`

- [ ] **Step 1: 写失败测试**

创建 `packages/backend/internal/usertrace/useragent_test.go`：

```go
package usertrace

import "testing"

func TestParseUserAgent(t *testing.T) {
	cases := []struct {
		name string
		ua   string
		want DeviceInfo
	}{
		{"chrome mac", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36", DeviceInfo{"desktop", "macOS", "Chrome"}},
		{"safari ios", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", DeviceInfo{"mobile", "iOS", "Safari"}},
		{"edge windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0 Safari/537.36 Edg/120.0", DeviceInfo{"desktop", "Windows", "Edge"}},
		{"firefox linux", "Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0", DeviceInfo{"desktop", "Linux", "Firefox"}},
		{"android phone", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 Chrome/120.0 Mobile Safari/537.36", DeviceInfo{"mobile", "Android", "Chrome"}},
		{"ipad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1", DeviceInfo{"tablet", "iOS", "Safari"}},
		{"bot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", DeviceInfo{"bot", "other", "other"}},
		{"empty", "", DeviceInfo{"other", "other", "other"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseUserAgent(tc.ua); got != tc.want {
				t.Fatalf("ParseUserAgent() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
```

创建 `packages/backend/internal/usertrace/request_test.go`：

```go
package usertrace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCaptureRequestSanitizesQueryAndDevice(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects?keyword=abc&token=secret&password=p", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36")
	req.Header.Set("X-Device-Id", "dev_123-ABC")
	req.Header.Set("Referer", "https://vibebiz-dev.com/")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	info := CaptureRequest(c)
	if info.Query == "" || strings.Contains(info.Query, "secret") || strings.Contains(info.Query, "password=p") {
		t.Fatalf("query = %q", info.Query)
	}
	if info.DeviceID != "dev_123-ABC" || info.DeviceType != "desktop" || info.OS != "Windows" || info.Browser != "Chrome" {
		t.Fatalf("info = %+v", info)
	}
	if info.Referer != "https://vibebiz-dev.com/" {
		t.Fatalf("referer = %q", info.Referer)
	}
}

func TestCaptureRequestDropsInvalidDeviceID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil)
	req.Header.Set("X-Device-Id", "bad id with spaces")
	c := e.NewContext(req, httptest.NewRecorder())

	if info := CaptureRequest(c); info.DeviceID != "" {
		t.Fatalf("device id = %q, want empty", info.DeviceID)
	}
}
```

> 测试直接使用 `strings.Contains`，实现 `request.go` 也需要 `strings`。

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/usertrace
```

Expected: 包或函数未定义，编译失败。

- [ ] **Step 3: 实现 event.go**

```go
package usertrace

import "time"

const (
	EventLogin   = "login"
	EventRequest = "request"
)

type Event struct {
	EventID        string    `json:"eventId"`
	EventType      string    `json:"eventType"`
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

type Appender interface {
	Append(event Event)
}
```

- [ ] **Step 4: 实现 useragent.go**

```go
package usertrace

import "strings"

type DeviceInfo struct {
	DeviceType string
	OS         string
	Browser    string
}

func ParseUserAgent(ua string) DeviceInfo {
	if strings.TrimSpace(ua) == "" {
		return DeviceInfo{DeviceType: "other", OS: "other", Browser: "other"}
	}
	lower := strings.ToLower(ua)
	info := DeviceInfo{DeviceType: "desktop", OS: "other", Browser: "other"}

	switch {
	case strings.Contains(lower, "bot"), strings.Contains(lower, "spider"), strings.Contains(lower, "crawler"):
		info.DeviceType = "bot"
	case strings.Contains(lower, "ipad"), strings.Contains(lower, "tablet"):
		info.DeviceType = "tablet"
	case strings.Contains(lower, "android") && !strings.Contains(lower, "mobile"):
		info.DeviceType = "tablet"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "android"), strings.Contains(lower, "mobile"):
		info.DeviceType = "mobile"
	}

	switch {
	case strings.Contains(lower, "windows"):
		info.OS = "Windows"
	case strings.Contains(lower, "iphone"), strings.Contains(lower, "ipad"), strings.Contains(lower, "ios"):
		info.OS = "iOS"
	case strings.Contains(lower, "mac os x"), strings.Contains(lower, "macintosh"):
		info.OS = "macOS"
	case strings.Contains(lower, "android"):
		info.OS = "Android"
	case strings.Contains(lower, "linux"):
		info.OS = "Linux"
	}

	switch {
	case strings.Contains(lower, "edg/"):
		info.Browser = "Edge"
	case strings.Contains(lower, "chrome/"), strings.Contains(lower, "crios/"):
		info.Browser = "Chrome"
	case strings.Contains(lower, "firefox/"), strings.Contains(lower, "fxios/"):
		info.Browser = "Firefox"
	case strings.Contains(lower, "safari/"):
		info.Browser = "Safari"
	}
	return info
}
```

- [ ] **Step 5: 实现 request.go**

```go
package usertrace

import (
	"net/url"
	"strings"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"
)

var sensitiveQueryKeys = map[string]struct{}{
	"password": {}, "passwd": {}, "token": {}, "access_token": {},
	"secret": {}, "api_key": {}, "key": {}, "authorization": {},
}

type RequestInfo struct {
	IP         string
	Method     string
	Path       string
	Query      string
	Route      string
	DeviceID   string
	DeviceType string
	OS         string
	Browser    string
	UserAgent  string
	Referer    string
	RequestID  string
	TraceID    string
}

func CaptureRequest(c echo.Context) RequestInfo {
	req := c.Request()
	info := RequestInfo{
		IP:        c.RealIP(),
		Method:    req.Method,
		Path:      truncate(req.URL.Path, 512),
		Query:     sanitizeQuery(req.URL.RawQuery),
		Route:     truncate(c.Path(), 255),
		DeviceID:  normalizeDeviceID(req.Header.Get("X-Device-Id")),
		UserAgent: truncate(req.UserAgent(), 512),
		Referer:   truncate(req.Referer(), 1024),
		RequestID: requestID(c),
		TraceID:   traceID(c),
	}
	if info.Route == "" {
		info.Route = info.Path
	}
	device := ParseUserAgent(info.UserAgent)
	info.DeviceType = device.DeviceType
	info.OS = device.OS
	info.Browser = device.Browser
	return info
}

func sanitizeQuery(raw string) string {
	if raw == "" {
		return ""
	}
	values, err := url.ParseQuery(raw)
	if err != nil {
		return truncate(raw, 1024)
	}
	for key := range values {
		if _, ok := sensitiveQueryKeys[strings.ToLower(key)]; ok {
			values[key] = []string{"***"}
		}
	}
	return truncate(values.Encode(), 1024)
}

func normalizeDeviceID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 64 {
		return ""
	}
	for _, r := range raw {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return ""
	}
	return raw
}

func requestID(c echo.Context) string {
	if id := c.Response().Header().Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}

func traceID(c echo.Context) string {
	spanContext := trace.SpanContextFromContext(c.Request().Context())
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
```

- [ ] **Step 6: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/usertrace
```

Expected: PASS。若测试中使用 `strings.Contains`，删除自定义 helper 与 `indexOf`。

- [ ] **Step 7: 检查点**

记录变更文件；不创建 commit。

---

## Task 3: 本地 Recorder

**Files:**
- Create: `packages/backend/internal/usertrace/recorder.go`
- Test: `packages/backend/internal/usertrace/recorder_test.go`

**Interfaces:**
- Consumes: `usertrace.Event`、`usertrace.ParseUserAgent`
- Produces: `usertrace.NewRecorder(dir, plane string, logger *slog.Logger) *Recorder`
- Produces: `(*Recorder).Append(Event)`，满足 `Appender`

- [ ] **Step 1: 写失败测试**

```go
package usertrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderAppendsPerUserPerDay(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder(dir, "cp", nil)
	r.now = func() time.Time { return time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local) }
	r.newID = func() string { return "event-1" }

	r.Append(Event{
		EventType: EventRequest, UserID: 7, Username: "wang",
		IP: "1.2.3.4", RequestMethod: "GET", RequestPath: "/cp/api/v1/projects",
		StatusCode: 200, UserAgent: "Chrome/120.0",
	})

	path := filepath.Join(dir, "cp", "2026-11-06", "7.log")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("decode event: %v (%s)", err, body)
	}
	if event.EventID != "event-1" || event.UserID != 7 || event.DeviceType != "desktop" || event.Browser != "Chrome" {
		t.Fatalf("event = %+v", event)
	}
	if event.EventAt.IsZero() {
		t.Fatal("event time not filled")
	}
}

func TestRecorderUsesAnonymousForUnknownUser(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder(dir, "cp", nil)
	r.now = func() time.Time { return time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local) }
	r.newID = func() string { return "event-2" }

	r.Append(Event{EventType: EventLogin, LoginResult: "failed"})

	if _, err := os.Stat(filepath.Join(dir, "cp", "2026-11-06", "anonymous.log")); err != nil {
		t.Fatalf("anonymous log: %v", err)
	}
}

func TestNilRecorderDoesNotPanic(t *testing.T) {
	var r *Recorder
	r.Append(Event{})
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run Recorder
```

Expected: `NewRecorder` 未定义。

- [ ] **Step 3: 实现 recorder.go**

```go
package usertrace

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Recorder struct {
	dir    string
	plane  string
	logger *slog.Logger
	now    func() time.Time
	newID  func() string
	mu     sync.Mutex
}

func NewRecorder(dir, plane string, logger *slog.Logger) *Recorder {
	return &Recorder{
		dir:    dir,
		plane:  plane,
		logger: logger,
		now:    time.Now,
		newID:  uuid.NewString,
	}
}

func (r *Recorder) Append(event Event) {
	if r == nil {
		return
	}
	if event.EventAt.IsZero() {
		event.EventAt = r.now()
	}
	if event.EventID == "" {
		event.EventID = r.newID()
	}
	event.DeviceID = normalizeDeviceID(event.DeviceID)
	event.UserAgent = truncate(event.UserAgent, 512)
	event.Referer = truncate(event.Referer, 1024)
	event.RequestPath = truncate(event.RequestPath, 512)
	event.RequestQuery = truncate(event.RequestQuery, 1024)
	event.Route = truncate(event.Route, 255)
	event.ErrorMessage = truncate(event.ErrorMessage, 255)
	event.Username = truncate(event.Username, 64)
	if event.DeviceType == "" || event.OS == "" || event.Browser == "" {
		device := ParseUserAgent(event.UserAgent)
		if event.DeviceType == "" {
			event.DeviceType = device.DeviceType
		}
		if event.OS == "" {
			event.OS = device.OS
		}
		if event.Browser == "" {
			event.Browser = device.Browser
		}
	}

	line, err := json.Marshal(event)
	if err != nil {
		r.warn("marshal user trace failed", err)
		return
	}
	path := r.path(event)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		r.warn("create user trace dir failed", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		r.warn("open user trace file failed", err)
		return
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		r.warn("write user trace file failed", err)
	}
}

func (r *Recorder) path(event Event) string {
	name := "anonymous"
	if event.UserID > 0 {
		name = fmt.Sprintf("%d", event.UserID)
	}
	return filepath.Join(r.dir, r.plane, event.EventAt.Format("2006-01-02"), name+".log")
}

func (r *Recorder) warn(msg string, err error) {
	if r.logger != nil {
		r.logger.Warn(msg, "error", err)
	}
}

func TrimPlane(plane string) string {
	plane = strings.ToLower(strings.TrimSpace(plane))
	if plane != "dp" {
		return "cp"
	}
	return plane
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/usertrace
```

Expected: PASS。

- [ ] **Step 5: 检查点**

记录变更文件；不创建 commit。

---

## Task 4: MySQL Repository（写入 + 查询）

**Files:**
- Create: `packages/backend/internal/usertrace/repository.go`
- Test: `packages/backend/internal/usertrace/repository_test.go`

**Interfaces:**
- Consumes: `usertrace.Event`、`model.UserAccessLog`
- Produces: `usertrace.Sink interface { Write(ctx, []Event) error }`
- Produces: `usertrace.NewMySQLRepository(db *gorm.DB) *MySQLRepository`
- Produces: `(*MySQLRepository).Write(ctx, []Event) error`
- Produces: `usertrace.Query`、`usertrace.Page`、`usertrace.Log`、`usertrace.Queryer`
- Produces: `(*MySQLRepository).List(ctx, Query) (Page, error)`

- [ ] **Step 1: 写失败测试**

创建 `packages/backend/internal/usertrace/repository_test.go`：

```go
package usertrace

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newTraceMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func TestMySQLRepositoryWrite(t *testing.T) {
	db, mock := newTraceMockDB(t)
	repo := NewMySQLRepository(db)
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `user_access_log`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := repo.Write(context.Background(), []Event{{
		EventID: "e1", EventType: EventRequest, UserID: 7, Username: "wang",
		RequestPath: "/cp/api/v1/projects", StatusCode: 200, EventAt: at,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRepositoryList(t *testing.T) {
	db, mock := newTraceMockDB(t)
	repo := NewMySQLRepository(db)
	at := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	mock.ExpectQuery("SELECT count\\(\\*\\) FROM `user_access_log` WHERE event_type = \\?").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT \\* FROM `user_access_log` WHERE event_type = \\?").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "event_id", "event_type", "user_id", "username", "ip",
			"device_id", "device_type", "os", "browser", "request_method",
			"request_path", "request_query", "route", "status_code",
			"duration_ms", "event_at",
		}).AddRow(1, "e1", "request", 7, "wang", "1.2.3.4", "d1", "desktop", "macOS", "Chrome", "GET", "/cp/api/v1/projects", "", "/cp/api/v1/projects", 200, 12, at))

	page, err := repo.List(context.Background(), Query{EventType: "request", Page: 1, PageSize: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("page = %+v", page)
	}
	if page.Items[0].ID != "1" || page.Items[0].EventType != "request" || page.Items[0].Path != "/cp/api/v1/projects" || page.Items[0].DurationMs != 12 {
		t.Fatalf("log = %+v", page.Items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run MySQLRepository
```

Expected: `NewMySQLRepository` 未定义。

- [ ] **Step 3: 实现 repository.go**

```go
package usertrace

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type Sink interface {
	Write(ctx context.Context, events []Event) error
}

type Query struct {
	Keyword   string
	EventType string
	From      *time.Time
	To        *time.Time
	Page      int
	PageSize  int
}

type Log struct {
	ID             string    `json:"id"`
	EventType      string    `json:"eventType"`
	UserID         int64     `json:"userId"`
	Username       string    `json:"username"`
	WorkspaceID    int64     `json:"workspaceId"`
	ProjectID      string    `json:"projectId"`
	IP             string    `json:"ip"`
	Region         string    `json:"region"`
	DeviceID       string    `json:"deviceId"`
	DeviceType     string    `json:"deviceType"`
	OS             string    `json:"os"`
	Browser        string    `json:"browser"`
	UserAgent      string    `json:"userAgent"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	Query          string    `json:"query"`
	Route          string    `json:"route"`
	StatusCode     int       `json:"statusCode"`
	DurationMs     int       `json:"durationMs"`
	LoginResult    string    `json:"loginResult"`
	ErrorMessage   string    `json:"errorMessage"`
	EventAt        time.Time `json:"eventAt"`
}

type Page struct {
	Items    []Log
	Total    int64
	Page     int
	PageSize int
}

type Queryer interface {
	List(ctx context.Context, query Query) (Page, error)
}

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

func (r *MySQLRepository) Write(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	rows := make([]model.UserAccessLog, 0, len(events))
	for _, event := range events {
		var userID *int64
		if event.UserID > 0 {
			id := event.UserID
			userID = &id
		}
		rows = append(rows, model.UserAccessLog{
			EventID:        event.EventID,
			EventType:      event.EventType,
			UserID:         userID,
			Username:       event.Username,
			WorkspaceID:    event.WorkspaceID,
			ProjectID:      event.ProjectID,
			IP:             event.IP,
			RegionCountry:  event.RegionCountry,
			RegionProvince: event.RegionProvince,
			RegionCity:     event.RegionCity,
			DeviceID:       event.DeviceID,
			DeviceType:     event.DeviceType,
			OS:             event.OS,
			Browser:        event.Browser,
			UserAgent:      event.UserAgent,
			Referer:        event.Referer,
			RequestMethod:  event.RequestMethod,
			RequestPath:    event.RequestPath,
			RequestQuery:   event.RequestQuery,
			Route:          event.Route,
			StatusCode:     event.StatusCode,
			DurationMs:     event.DurationMs,
			RequestID:      event.RequestID,
			TraceID:        event.TraceID,
			LoginResult:    event.LoginResult,
			ErrorMessage:   event.ErrorMessage,
			EventAt:        event.EventAt,
		})
	}
	err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(&rows).Error
	if err != nil {
		return fmt.Errorf("write user access log: %w", err)
	}
	return nil
}

func (r *MySQLRepository) List(ctx context.Context, query Query) (Page, error) {
	query = normalizeQuery(query)
	scope := func(db *gorm.DB) *gorm.DB {
		tx := db.Model(&model.UserAccessLog{})
		if query.Keyword != "" {
			like := "%" + query.Keyword + "%"
			tx = tx.Where("(username LIKE ? OR ip LIKE ? OR request_path LIKE ? OR device_id LIKE ? OR project_id LIKE ?)", like, like, like, like, like)
		}
		if query.EventType != "" {
			tx = tx.Where("event_type = ?", query.EventType)
		}
		if query.From != nil {
			tx = tx.Where("event_at >= ?", *query.From)
		}
		if query.To != nil {
			tx = tx.Where("event_at < ?", *query.To)
		}
		return tx
	}

	var total int64
	if err := r.db.WithContext(ctx).Scopes(scope).Count(&total).Error; err != nil {
		return Page{}, fmt.Errorf("count user access log: %w", err)
	}

	var rows []model.UserAccessLog
	err := r.db.WithContext(ctx).Scopes(scope).
		Order("event_at DESC, id DESC").
		Offset((query.Page - 1) * query.PageSize).
		Limit(query.PageSize).
		Find(&rows).Error
	if err != nil {
		return Page{}, fmt.Errorf("list user access log: %w", err)
	}

	items := make([]Log, 0, len(rows))
	for _, row := range rows {
		var userID int64
		if row.UserID != nil {
			userID = *row.UserID
		}
		region := strings.TrimSpace(strings.Join([]string{row.RegionCountry, row.RegionProvince, row.RegionCity}, " "))
		items = append(items, Log{
			ID:           strconv.FormatInt(row.ID, 10),
			EventType:    row.EventType,
			UserID:       userID,
			Username:     row.Username,
			WorkspaceID:  row.WorkspaceID,
			ProjectID:    row.ProjectID,
			IP:           row.IP,
			Region:       region,
			DeviceID:     row.DeviceID,
			DeviceType:   row.DeviceType,
			OS:           row.OS,
			Browser:      row.Browser,
			UserAgent:    row.UserAgent,
			Method:       row.RequestMethod,
			Path:         row.RequestPath,
			Query:        row.RequestQuery,
			Route:        row.Route,
			StatusCode:   row.StatusCode,
			DurationMs:   row.DurationMs,
			LoginResult:  row.LoginResult,
			ErrorMessage: row.ErrorMessage,
			EventAt:      row.EventAt,
		})
	}
	return Page{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

const (
	defaultPageSize = 10
	maxPageSize     = 100
)

func normalizeQuery(query Query) Query {
	query.Keyword = strings.TrimSpace(query.Keyword)
	if query.Page < 1 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = defaultPageSize
	}
	if query.PageSize > maxPageSize {
		query.PageSize = maxPageSize
	}
	return query
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run MySQLRepository
```

Expected: PASS。若 GORM 生成的 count SQL 带括号导致正则不匹配，调整正则或使用 `regexp.QuoteMeta`，不要放宽到无过滤条件。

- [ ] **Step 5: 检查点**

记录变更文件；不创建 commit。

---

## Task 5: 定时 Syncer

**Files:**
- Create: `packages/backend/internal/usertrace/syncer.go`
- Test: `packages/backend/internal/usertrace/syncer_test.go`

**Interfaces:**
- Consumes: `usertrace.Sink`、`usertrace.Event`
- Produces: `usertrace.NewSyncer(dir, plane string, sink Sink, interval time.Duration, logger *slog.Logger) *Syncer`
- Produces: `(*Syncer).Run(ctx context.Context)`
- Produces: `(*Syncer).SyncOnce(ctx context.Context) error`

- [ ] **Step 1: 写失败测试**

```go
package usertrace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeSink struct {
	batches [][]Event
	err     error
}

func (f *fakeSink) Write(_ context.Context, events []Event) error {
	if f.err != nil {
		return f.err
	}
	copied := append([]Event(nil), events...)
	f.batches = append(f.batches, copied)
	return nil
}

func writeTraceFile(t *testing.T, root, day, name string, events []Event) string {
	t.Helper()
	dir := filepath.Join(root, "cp", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSyncerSyncOnce(t *testing.T) {
	root := t.TempDir()
	path := writeTraceFile(t, root, "2026-11-06", "7.log", []Event{
		{EventID: "e1", EventType: EventRequest, UserID: 7},
		{EventID: "e2", EventType: EventLogin, UserID: 7},
	})
	sink := &fakeSink{}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || len(sink.batches[0]) != 2 {
		t.Fatalf("batches = %+v", sink.batches)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("live file still exists: %v", err)
	}
	if pending, _ := filepath.Glob(filepath.Join(root, "cp", "2026-11-06", "*.syncing")); len(pending) != 0 {
		t.Fatalf("pending files = %v", pending)
	}
}

func TestSyncerKeepsFileOnSinkFailure(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "2026-11-06", "7.log", []Event{{EventID: "e1", EventType: EventRequest, UserID: 7}})
	sink := &fakeSink{err: errors.New("db down")}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err == nil {
		t.Fatal("SyncOnce() error = nil, want failure")
	}
	if pending, _ := filepath.Glob(filepath.Join(root, "cp", "2026-11-06", "*.syncing")); len(pending) != 1 {
		t.Fatalf("pending files = %v", pending)
	}
}

func TestSyncerSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cp", "2026-11-06")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "not-json\n" + `{"eventId":"e1","eventType":"request","userId":7}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "7.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sink := &fakeSink{}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || len(sink.batches[0]) != 1 || sink.batches[0][0].EventID != "e1" {
		t.Fatalf("batches = %+v", sink.batches)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run Syncer
```

Expected: `NewSyncer` 未定义。

- [ ] **Step 3: 实现 syncer.go**

```go
package usertrace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const batchSize = 500

type Syncer struct {
	dir      string
	plane    string
	sink     Sink
	interval time.Duration
	logger   *slog.Logger
	now      func() time.Time
}

func NewSyncer(dir, plane string, sink Sink, interval time.Duration, logger *slog.Logger) *Syncer {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Syncer{dir: dir, plane: plane, sink: sink, interval: interval, logger: logger, now: time.Now}
}

func (s *Syncer) Run(ctx context.Context) {
	if s == nil || s.sink == nil {
		return
	}
	s.run(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.run(ctx)
		}
	}
}

func (s *Syncer) run(ctx context.Context) {
	if err := s.SyncOnce(ctx); err != nil && s.logger != nil {
		s.logger.Warn("user trace sync failed", "error", err)
	}
}

func (s *Syncer) SyncOnce(ctx context.Context) error {
	root := filepath.Join(s.dir, s.plane)
	live, err := filepath.Glob(filepath.Join(root, "*", "*.log"))
	if err != nil {
		return err
	}
	for _, path := range live {
		target := strings.TrimSuffix(path, ".log") + fmt.Sprintf(".%d.syncing", s.now().UnixNano())
		if err := os.Rename(path, target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate user trace file: %w", err)
		}
	}

	pending, err := filepath.Glob(filepath.Join(root, "*", "*.syncing"))
	if err != nil {
		return err
	}
	var firstErr error
	for _, path := range pending {
		if err := s.flushFile(ctx, path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Syncer) flushFile(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open user trace file: %w", err)
	}
	defer file.Close()

	var batch []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.sink.Write(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if event.EventID == "" {
			continue
		}
		batch = append(batch, event)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return fmt.Errorf("write user trace batch: %w", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan user trace file: %w", err)
	}
	if err := flush(); err != nil {
		return fmt.Errorf("write user trace batch: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove synced user trace file: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run Syncer
```

Expected: PASS。

- [ ] **Step 5: 检查点**

记录变更文件；不创建 commit。

---

## Task 6: 请求中间件、CORS 与 CP/DP 装配

**Files:**
- Create: `packages/backend/internal/usertrace/middleware.go`
- Test: `packages/backend/internal/usertrace/middleware_test.go`
- Modify: `packages/backend/internal/server/server.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/cmd/dp/main.go`

**Interfaces:**
- Consumes: `Appender`、`CaptureRequest`、`auth.SessionFrom`
- Produces: `usertrace.Middleware(appender Appender, logger *slog.Logger) echo.MiddlewareFunc`

- [ ] **Step 1: 写失败测试**

```go
package usertrace

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
)

type fakeAppender struct {
	events []Event
}

func (f *fakeAppender) Append(event Event) {
	f.events = append(f.events, event)
}

func TestMiddlewareRecordsAuthenticatedRequest(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7, Username: "wang", CurrentWorkspaceID: 3})
			return next(c)
		}
	})
	e.GET("/cp/api/v1/projects/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1", nil)
	req.Header.Set("User-Agent", "Chrome/120.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if len(appender.events) != 1 {
		t.Fatalf("events = %+v", appender.events)
	}
	event := appender.events[0]
	if event.EventType != EventRequest || event.UserID != 7 || event.WorkspaceID != 3 || event.ProjectID != "p1" {
		t.Fatalf("event = %+v", event)
	}
	if event.RequestMethod != http.MethodGet || event.StatusCode != http.StatusOK {
		t.Fatalf("event = %+v", event)
	}
}

func TestMiddlewareSkipsAnonymousAndOps(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.GET("/cp/api/v1/projects", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/cp/api/v1/admin/users", func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{Realm: auth.RealmOps, UserID: 9})
		return c.NoContent(http.StatusOK)
	})

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/users", nil))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if len(appender.events) != 0 {
		t.Fatalf("events = %+v, want none", appender.events)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/usertrace -run Middleware
```

Expected: `Middleware` 未定义。

- [ ] **Step 3: 实现 middleware.go**

```go
package usertrace

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
)

func Middleware(appender Appender, logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appender == nil {
				return next(c)
			}
			if c.Request().Method == http.MethodOptions || c.Request().URL.Path == "/health" || c.Request().URL.Path == "/metrics" {
				return next(c)
			}

			start := time.Now()
			handlerErr := next(c)

			session, ok := auth.SessionFrom(c)
			if !ok || session.Realm != auth.RealmUser {
				return handlerErr
			}
			status := c.Response().Status
			if handlerErr != nil {
				status = http.StatusInternalServerError
				var httpErr *echo.HTTPError
				if errors.As(handlerErr, &httpErr) {
					status = httpErr.Code
				}
			}
			info := CaptureRequest(c)
			appender.Append(Event{
				EventType:     EventRequest,
				UserID:        session.UserID,
				Username:      session.Username,
				WorkspaceID:   session.CurrentWorkspaceID,
				ProjectID:     projectIDFromRoute(c),
				IP:            info.IP,
				DeviceID:      info.DeviceID,
				DeviceType:    info.DeviceType,
				OS:            info.OS,
				Browser:       info.Browser,
				UserAgent:     info.UserAgent,
				Referer:       info.Referer,
				RequestMethod: info.Method,
				RequestPath:   info.Path,
				RequestQuery:  info.Query,
				Route:         info.Route,
				StatusCode:    status,
				DurationMs:    int(time.Since(start).Milliseconds()),
				RequestID:     info.RequestID,
				TraceID:       info.TraceID,
			})
			if logger != nil && status >= http.StatusInternalServerError {
				logger.Debug("user trace recorded failed request", "path", info.Path, "status", status)
			}
			return handlerErr
		}
	}
}

func projectIDFromRoute(c echo.Context) string {
	if strings.Contains(c.Path(), "/projects/:id") {
		return c.Param("id")
	}
	return ""
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/usertrace
```

Expected: PASS。

- [ ] **Step 5: 放行 CORS 头**

在 `packages/backend/internal/server/server.go` 的 `AllowHeaders` 列表中加入 `"X-Device-Id"`：

```go
		AllowHeaders:     []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization, "Cookie", "X-Device-Id"},
```

- [ ] **Step 6: 装配 CP**

在 `packages/backend/cmd/cp/main.go` 引入：

```go
	"vibebiz/internal/usertrace"
```

在创建 `authHandler` 之前，加入：

```go
	traceRepo := usertrace.NewMySQLRepository(db)
	var traceRecorder *usertrace.Recorder
	if cfg.UserTraceEnabled {
		traceRecorder = usertrace.NewRecorder(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), obs.Logger)
		traceCtx, traceCancel := context.WithCancel(ctx)
		defer traceCancel()
		go usertrace.NewSyncer(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), traceRepo, cfg.UserTraceSyncInterval, obs.Logger).Run(traceCtx)
	}
```

把 `authHandler` 保持原样；`Trace` 字段与注入在 Task 7 完成，避免本任务引用尚不存在的字段。

在 `e := server.New(cfg, obs)` 之后、`e.Use(requestCounter.Middleware())` 之前或之后加入：

```go
	e.Use(usertrace.Middleware(traceRecorder, obs.Logger))
```

> `traceRecorder` 为 nil 时中间件直接放行；`Append` 也做 nil 安全。

- [ ] **Step 7: 装配 DP**

在 `packages/backend/cmd/dp/main.go` 引入：

```go
	"vibebiz/internal/usertrace"
```

在 `e := server.New(cfg, obs)` 之前加入：

```go
	traceRepo := usertrace.NewMySQLRepository(db)
	var traceRecorder *usertrace.Recorder
	if cfg.UserTraceEnabled {
		traceRecorder = usertrace.NewRecorder(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), obs.Logger)
		traceCtx, traceCancel := context.WithCancel(ctx)
		defer traceCancel()
		go usertrace.NewSyncer(cfg.UserTraceDir, usertrace.TrimPlane(cfg.Plane), traceRepo, cfg.UserTraceSyncInterval, obs.Logger).Run(traceCtx)
	}
```

在 `router.RegisterData(...)` 之前加入：

```go
	e.Use(usertrace.Middleware(traceRecorder, obs.Logger))
```

- [ ] **Step 8: 编译并运行测试**

Run:

```bash
cd packages/backend && go test ./... && go vet ./...
```

Expected: PASS。

- [ ] **Step 9: 检查点**

记录变更文件；不创建 commit。

---

## Task 7: 登录成功/失败埋点

**Files:**
- Modify: `packages/backend/internal/handler/auth.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Test: `packages/backend/internal/handler/auth_test.go`

**Interfaces:**
- Consumes: `usertrace.Appender`、`usertrace.CaptureRequest`
- Produces: `handler.AuthHandler.Trace usertrace.Appender`
- Produces: 登录事件字段 `EventType=login`、`LoginResult=success|failed`

- [ ] **Step 1: 写失败测试**

在 `packages/backend/internal/handler/auth_test.go` 顶部加入 import：

```go
	"vibebiz/internal/usertrace"
```

加入 fake：

```go
type fakeTraceAppender struct {
	events []usertrace.Event
}

func (f *fakeTraceAppender) Append(event usertrace.Event) {
	f.events = append(f.events, event)
}
```

追加测试：

```go
func TestLoginUserRecordsSuccessTrace(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "wanghg11", Email: "wang@example.com", Status: 1}, []int64{3}, nil)
	trace := &fakeTraceAppender{}
	h.Trace = trace
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/auth/login", `{"identifier":"wanghg11","password":"test-password"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(trace.events) != 1 {
		t.Fatalf("events = %+v", trace.events)
	}
	event := trace.events[0]
	if event.EventType != usertrace.EventLogin || event.LoginResult != "success" {
		t.Fatalf("event = %+v", event)
	}
	if event.UserID != 1 || event.Username != "wanghg11" || event.WorkspaceID != 3 || event.StatusCode != http.StatusOK {
		t.Fatalf("event = %+v", event)
	}
}

func TestLoginUserRecordsFailureTrace(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "wanghg11", Email: "wang@example.com", Status: 1}, []int64{3}, nil)
	trace := &fakeTraceAppender{}
	h.Trace = trace
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/auth/login", `{"identifier":"wanghg11","password":"wrong"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(trace.events) != 1 {
		t.Fatalf("events = %+v", trace.events)
	}
	event := trace.events[0]
	if event.LoginResult != "failed" || event.StatusCode != http.StatusUnauthorized {
		t.Fatalf("event = %+v", event)
	}
	if event.Username != "wanghg11" || event.ErrorMessage != "invalid credentials" {
		t.Fatalf("event = %+v", event)
	}
}

func TestLoginOpsDoesNotRecordUserTrace(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, nil, nil, &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Email: "admin@example.com", RoleCode: "super_admin", Status: 1})
	trace := &fakeTraceAppender{}
	h.Trace = trace
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/admin/auth/login", `{"identifier":"admin","password":"test-password"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(trace.events) != 0 {
		t.Fatalf("events = %+v, want none", trace.events)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/handler -run Login.*Trace
```

Expected: `AuthHandler` 无 `Trace` 字段，编译失败。

- [ ] **Step 3: 重构登录错误分类**

在 `packages/backend/internal/handler/auth.go` 引入：

```go
	"time"

	"vibebiz/internal/usertrace"
```

把 `AuthHandler` 改为：

```go
type AuthHandler struct {
	Service    *auth.Service
	Account    *account.Service
	UserCookie auth.CookieManager
	OpsCookie  auth.CookieManager
	Trace      usertrace.Appender
}
```

新增：

```go
type loginFailure struct {
	status  int
	code    int
	message string
}

func classifyLoginError(err error) loginFailure {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		return loginFailure{http.StatusUnauthorized, 40102, "invalid credentials"}
	case errors.Is(err, auth.ErrAccountDisabled):
		return loginFailure{http.StatusForbidden, 40301, "account disabled"}
	case errors.Is(err, auth.ErrNoWorkspace):
		return loginFailure{http.StatusForbidden, 40302, "no workspace available"}
	default:
		return loginFailure{http.StatusServiceUnavailable, 50301, "session store unavailable"}
	}
}
```

把 `loginError` 改为：

```go
func loginError(c echo.Context, err error) error {
	failure := classifyLoginError(err)
	return response.Fail(c, failure.status, failure.code, failure.message)
}
```

- [ ] **Step 4: 改写 LoginUser**

```go
func (h *AuthHandler) LoginUser(c echo.Context) error {
	start := time.Now()
	req, ok := bindLogin(c)
	if !ok {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	session, err := h.Service.LoginUser(c.Request().Context(), req.Identifier, req.Password, c.RealIP())
	if err != nil {
		failure := classifyLoginError(err)
		h.recordLogin(c, req.Identifier, nil, "failed", failure, time.Since(start))
		return loginError(c, err)
	}
	if err := h.UserCookie.Set(c, auth.CookieIdentity{SID: session.ID, Username: session.Username, Email: session.Email}); err != nil {
		failure := loginFailure{http.StatusInternalServerError, 50005, "set session cookie failed"}
		h.recordLogin(c, req.Identifier, session, "failed", failure, time.Since(start))
		return response.Fail(c, failure.status, failure.code, failure.message)
	}
	h.recordLogin(c, req.Identifier, session, "success", loginFailure{status: http.StatusOK}, time.Since(start))
	return h.meResult(c, session)
}

func (h *AuthHandler) recordLogin(c echo.Context, identifier string, session *auth.Session, result string, failure loginFailure, elapsed time.Duration) {
	if h.Trace == nil {
		return
	}
	info := usertrace.CaptureRequest(c)
	event := usertrace.Event{
		EventType:     usertrace.EventLogin,
		UserID:        sessionUserID(session),
		Username:      identifier,
		WorkspaceID:   sessionWorkspaceID(session),
		IP:            info.IP,
		DeviceID:      info.DeviceID,
		DeviceType:    info.DeviceType,
		OS:            info.OS,
		Browser:       info.Browser,
		UserAgent:     info.UserAgent,
		Referer:       info.Referer,
		RequestMethod: info.Method,
		RequestPath:   info.Path,
		RequestQuery:  info.Query,
		Route:         info.Route,
		StatusCode:    failure.status,
		DurationMs:    int(elapsed.Milliseconds()),
		RequestID:     info.RequestID,
		TraceID:       info.TraceID,
		LoginResult:   result,
		ErrorMessage:  failure.message,
	}
	if session != nil {
		event.Username = session.Username
	}
	h.Trace.Append(event)
}

func sessionUserID(session *auth.Session) int64 {
	if session == nil {
		return 0
	}
	return session.UserID
}

func sessionWorkspaceID(session *auth.Session) int64 {
	if session == nil {
		return 0
	}
	return session.CurrentWorkspaceID
}
```

- [ ] **Step 5: 在 CP 注入 Recorder**

在 `packages/backend/cmd/cp/main.go` 中，保持 `authHandler` 字面量不变，在其后按开关赋值：

```go
	if cfg.UserTraceEnabled {
		authHandler.Trace = traceRecorder
	}
```

`traceRecorder` 由 Task 6 创建；关闭采集时保持接口 nil，`recordLogin` 直接返回，避免 typed-nil 接口仍触发采集。

- [ ] **Step 6: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/handler -run Login && go test ./cmd/cp
```

Expected: PASS。确认原有登录测试不回归。

- [ ] **Step 7: 检查点**

记录变更文件；不创建 commit。

---

## Task 8: Admin 查询接口与路由

**Files:**
- Create: `packages/backend/internal/handler/access_log.go`
- Test: `packages/backend/internal/handler/access_log_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `usertrace.Queryer`、`usertrace.Query`、`usertrace.Page`
- Produces: `handler.NewAccessLogHandler(repo usertrace.Queryer) *AccessLogHandler`
- Produces: `(*AccessLogHandler).List(c echo.Context) error`
- Produces: `GET /cp/api/v1/admin/access-logs`

- [ ] **Step 1: 写失败测试**

创建 `packages/backend/internal/handler/access_log_test.go`：

```go
package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/usertrace"
)

type fakeAccessLogRepo struct {
	query usertrace.Query
	page  usertrace.Page
	err   error
}

func (f *fakeAccessLogRepo) List(_ context.Context, query usertrace.Query) (usertrace.Page, error) {
	f.query = query
	return f.page, f.err
}

func TestAccessLogListParsesFilters(t *testing.T) {
	repo := &fakeAccessLogRepo{page: usertrace.Page{
		Items:    []usertrace.Log{{ID: "1", EventType: "request", Username: "wang", Path: "/cp/api/v1/projects", StatusCode: 200, DurationMs: 12, EventAt: time.Unix(1700000000, 0).UTC()}},
		Total:    1,
		Page:     2,
		PageSize: 20,
	}}
	h := NewAccessLogHandler(repo)
	e := echo.New()
	e.GET("/cp/api/v1/admin/access-logs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/access-logs?page=2&pageSize=20&keyword=wang&eventType=request&from=2026-11-01&to=2026-11-06", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.query.Page != 2 || repo.query.PageSize != 20 || repo.query.Keyword != "wang" || repo.query.EventType != "request" {
		t.Fatalf("query = %+v", repo.query)
	}
	if repo.query.From == nil || repo.query.To == nil {
		t.Fatalf("time range = %+v", repo.query)
	}
	if repo.query.To.Sub(*repo.query.From) != 6*24*time.Hour {
		t.Fatalf("time range = %v - %v", repo.query.From, repo.query.To)
	}

	var env struct {
		Code   int `json:"code"`
		Result struct {
			Logs  []usertrace.Log `json:"logs"`
			Total int64           `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || env.Result.Total != 1 || len(env.Result.Logs) != 1 || env.Result.Logs[0].Path != "/cp/api/v1/projects" {
		t.Fatalf("response = %+v", env)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/backend && go test ./internal/handler -run AccessLog
```

Expected: `NewAccessLogHandler` 未定义。

- [ ] **Step 3: 实现 access_log.go**

```go
package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/response"
	"vibebiz/internal/usertrace"
)

type AccessLogHandler struct {
	Repo usertrace.Queryer
}

func NewAccessLogHandler(repo usertrace.Queryer) *AccessLogHandler {
	return &AccessLogHandler{Repo: repo}
}

func (h *AccessLogHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	query := usertrace.Query{
		Keyword:   c.QueryParam("keyword"),
		EventType: c.QueryParam("eventType"),
		Page:      page,
		PageSize:  pageSize,
		From:      parseAccessLogTime(c.QueryParam("from"), false),
		To:        parseAccessLogTime(c.QueryParam("to"), true),
	}
	result, err := h.Repo.List(c.Request().Context(), query)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "access logs unavailable")
	}
	return response.OK(c, map[string]any{
		"logs": result.Items, "total": result.Total, "page": result.Page, "pageSize": result.PageSize,
	})
}

func parseAccessLogTime(raw string, endOfDay bool) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		value := parsed.UTC()
		return &value
	}
	parsed, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil
	}
	if endOfDay {
		parsed = parsed.Add(24 * time.Hour)
	}
	return &parsed
}
```

- [ ] **Step 4: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./internal/handler -run AccessLog
```

Expected: PASS。

- [ ] **Step 5: 注册路由**

在 `packages/backend/internal/router/control.go` 的 `RegisterControl` 参数列表末尾加入：

```go
	accessLogs *handler.AccessLogHandler
```

在 `adminProtected.GET("/audit", adminHandler.Audit)` 之后加入：

```go
	if accessLogs != nil {
		adminProtected.GET("/access-logs", accessLogs.List)
	}
```

在 `packages/backend/cmd/cp/main.go` 创建：

```go
	accessLogHandler := handler.NewAccessLogHandler(traceRepo)
```

并把 `accessLogHandler` 作为 `RegisterControl` 的最后一个参数传入。

更新两处测试调用，在参数列表末尾补 `nil`：

- `packages/backend/internal/router/helpers_test.go:234`
- `packages/backend/internal/router/span_test.go:45`

- [ ] **Step 6: 运行测试确认通过**

Run:

```bash
cd packages/backend && go test ./... && go vet ./...
```

Expected: PASS。

- [ ] **Step 7: 检查点**

记录变更文件；不创建 commit。

---

## Task 9: web 设备码

**Files:**
- Create: `packages/web/src/utils/device.ts`
- Test: `packages/web/src/utils/device.test.ts`
- Modify: `packages/web/src/api/client.ts`

**Interfaces:**
- Produces: `getDeviceId(): string`
- Produces: `deviceHeader(): Record<string, string>`，为 CP/DP 请求加 `X-Device-Id`

- [ ] **Step 1: 写失败测试**

创建 `packages/web/src/utils/device.test.ts`：

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import { deviceHeader, getDeviceId } from './device'

describe('getDeviceId', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('generates and persists an id', () => {
    const first = getDeviceId()
    expect(first.length).toBeGreaterThan(8)
    expect(localStorage.getItem('vibebiz.deviceId')).toBe(first)
    expect(getDeviceId()).toBe(first)
  })

  it('reuses an existing id', () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    expect(getDeviceId()).toBe('device-123')
  })

  it('exposes the request header', () => {
    localStorage.setItem('vibebiz.deviceId', 'device-123')
    expect(deviceHeader()).toEqual({ 'X-Device-Id': 'device-123' })
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/web && npm test -- --run src/utils/device.test.ts
```

Expected: `getDeviceId` / `deviceHeader` 未定义。

- [ ] **Step 3: 实现 device.ts**

```ts
const DEVICE_KEY = 'vibebiz.deviceId'

function randomId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `dev-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

export function getDeviceId(): string {
  const stored = localStorage.getItem(DEVICE_KEY)
  if (stored) {
    return stored
  }
  const id = randomId()
  localStorage.setItem(DEVICE_KEY, id)
  return id
}

export function deviceHeader(): Record<string, string> {
  return { 'X-Device-Id': getDeviceId() }
}
```

- [ ] **Step 4: 修改 client.ts**

在文件顶部引入：

```ts
import { deviceHeader } from '@/utils/device'
```

并把 axios 类型引入改为：

```ts
import axios, { type AxiosInstance, type AxiosRequestConfig, type InternalAxiosRequestConfig } from 'axios'
```

在 `cpHttp` / `dpHttp` 创建后加入：

```ts
const applyDeviceHeader = (config: InternalAxiosRequestConfig) => {
  config.headers.set('X-Device-Id', deviceHeader()['X-Device-Id'])
  return config
}

cpHttp.interceptors.request.use(applyDeviceHeader)
dpHttp.interceptors.request.use(applyDeviceHeader)
```

> 若 Axios 的 `headers.set` 类型不可用，改用 `config.headers['X-Device-Id'] = deviceHeader()['X-Device-Id']`。以 `npm run build` 通过为准。

- [ ] **Step 5: 运行测试与构建**

Run:

```bash
cd packages/web && npm test -- --run && npm run build
```

Expected: PASS 且构建通过。

- [ ] **Step 6: 检查点**

记录变更文件；不创建 commit。

---

## Task 10: ops-web 访问日志页

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/views/AccessLogsView.vue`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: `GET /cp/api/v1/admin/access-logs`
- Produces: `AccessLog`、`AccessLogQuery` 类型
- Produces: `getAccessLogs(query): Promise<PageResult<AccessLog>>`
- Produces: `/access-logs` 路由与导航

- [ ] **Step 1: 写失败测试**

修改 `packages/ops-web/src/router/navigation.test.ts` 的期望数组：

```ts
    expect(navItems.map((item) => item.path)).toEqual([
      '/',
      '/users',
      '/projects',
      '/sandboxes',
      '/models',
      '/roles',
      '/fde-resources',
      '/topics',
      '/templates',
      '/scheduled-tasks',
      '/quotas',
      '/audit',
      '/access-logs',
      '/notifications',
      '/settings',
    ])
    expect(navItems.map((item) => item.label)).toEqual([
      '概览',
      '用户',
      '项目',
      '沙箱节点',
      '模型管理',
      '角色管理',
      'FDE 资源',
      '类别管理',
      '模板管理',
      '定时任务',
      '配额',
      '审计日志',
      '访问日志',
      '消息通知',
      '系统设置',
    ])
```

- [ ] **Step 2: 运行测试确认失败**

Run:

```bash
cd packages/ops-web && npm test -- --run src/router/navigation.test.ts
```

Expected: 数组不匹配。

- [ ] **Step 3: 加类型**

在 `packages/ops-web/src/types/index.ts` 加入：

```ts
export interface AccessLog {
  id: string
  eventType: 'login' | 'request'
  userId: number
  username: string
  workspaceId: number
  projectId: string
  ip: string
  region: string
  deviceId: string
  deviceType: string
  os: string
  browser: string
  userAgent: string
  method: string
  path: string
  query: string
  route: string
  statusCode: number
  durationMs: number
  loginResult: string
  errorMessage: string
  eventAt: string
}

export interface AccessLogQuery extends PageQuery {
  eventType?: string
  from?: string
  to?: string
}
```

- [ ] **Step 4: 加 API**

在 `packages/ops-web/src/api/admin.ts` 的类型 import 中加入 `AccessLog`、`AccessLogQuery`，并追加：

```ts
export async function getAccessLogs(
  query: AccessLogQuery = {},
): Promise<PageResult<AccessLog>> {
  const result = await request<Record<string, unknown>>({
    method: 'get',
    url: '/access-logs',
    params: {
      page: query.page,
      pageSize: query.pageSize,
      keyword: query.keyword,
      eventType: query.eventType,
      from: query.from,
      to: query.to,
    },
  })
  return toPage<AccessLog>(result, 'logs')
}
```

- [ ] **Step 5: 加页面**

创建 `packages/ops-web/src/views/AccessLogsView.vue`：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'

import { getAccessLogs } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AccessLog, PageQuery, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const eventType = ref('')
const from = ref('')
const to = ref('')
const eventOptions = [
  { label: '全部事件', value: '' },
  { label: '登录', value: 'login' },
  { label: '请求', value: 'request' },
]

async function loader(query: PageQuery) {
  return getAccessLogs({
    ...query,
    eventType: eventType.value || undefined,
    from: from.value || undefined,
    to: to.value || undefined,
  })
}

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(loader)

const columns: TableColumn[] = [
  { key: 'eventAt', label: '时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'eventType', label: '类型', width: '80px' },
  { key: 'username', label: '用户', width: '120px' },
  { key: 'ip', label: 'IP', width: '130px' },
  { key: 'path', label: '请求路径', width: '260px' },
  { key: 'statusCode', label: '状态', width: '70px' },
  { key: 'durationMs', label: '耗时', width: '80px', format: (value) => `${value} ms` },
  { key: 'terminal', label: '终端', width: '160px' },
  { key: 'deviceId', label: '设备码', width: '160px' },
  { key: 'projectId', label: '项目', width: '120px' },
]

const eventLabels: Record<string, string> = { login: '登录', request: '请求' }

function terminal(row: unknown): string {
  const log = row as AccessLog
  return [log.os, log.browser, log.deviceType].filter(Boolean).join(' / ')
}

function eventLabel(row: unknown): string {
  const type = (row as AccessLog).eventType
  return eventLabels[type] ?? type
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>访问日志</h1>
        <p class="page-desc">web 用户登录与接口访问痕迹，先落本地日志再批量入库。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <div class="filters">
      <Select v-model="eventType" :options="eventOptions" option-label="label" option-value="value" placeholder="事件类型" />
      <InputText v-model="from" type="date" />
      <InputText v-model="to" type="date" />
      <Button label="筛选" icon="pi pi-filter" @click="search" />
    </div>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无访问日志"
      @retry="load"
    >
      <template #cell-eventType="{ row }">
        <span class="tag">{{ eventLabel(row) }}</span>
      </template>
      <template #cell-statusCode="{ value }">
        <span :class="{ 'status-fail': Number(value) >= 400 }">{{ value }}</span>
      </template>
      <template #cell-terminal="{ row }">
        {{ terminal(row) }}
      </template>
    </DataTable>

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.status-fail {
  color: #1d4ed8;
  font-weight: 600;
}
</style>
```

- [ ] **Step 6: 加路由与导航**

`packages/ops-web/src/router/routes.ts`：

```ts
import AccessLogsView from '@/views/AccessLogsView.vue'
```

在 `audit` 路由之后加入：

```ts
  { path: 'access-logs', name: 'access-logs', component: AccessLogsView, meta: { title: '访问日志' } },
```

`packages/ops-web/src/router/navigation.ts` 在审计日志之后加入：

```ts
  { name: 'access-logs', label: '访问日志', path: '/access-logs', icon: 'pi pi-eye' },
```

- [ ] **Step 7: 运行测试与构建**

Run:

```bash
cd packages/ops-web && npm test -- --run && npm run build && npm run lint
```

Expected: PASS 且构建、lint 通过。

- [ ] **Step 8: 检查点**

记录变更文件；不创建 commit。

---

## Task 11: 迁移与全量验证

**Files:**
- No new files.

- [ ] **Step 1: 执行数据库迁移**

Run:

```bash
make -C packages/backend db-migrate-up
```

Expected: goose 输出 `OK 00024_user_access_log.sql`。

若本机没有 MySQL / `.env`，记录为未执行，并在最终说明中标注；不要伪造迁移成功。

- [ ] **Step 2: 运行后端全量测试**

Run:

```bash
make -C packages/backend test
make -C packages/backend quality
```

Expected: PASS，gofmt / go vet 无输出。

- [ ] **Step 3: 运行前端测试与构建**

Run:

```bash
make -C packages/web test
make -C packages/web lint
make -C packages/ops-web test
make -C packages/ops-web lint
```

Expected: PASS。

- [ ] **Step 4: 运行根质量门禁**

Run:

```bash
make validate
```

Expected: 全部通过。若已有与本改动无关的失败，记录原始输出，不要忽略。

- [ ] **Step 5: 手工冒烟（可选但推荐）**

启动服务：

```bash
make dev
```

操作：

1. 浏览器访问 `http://vibebiz-dev.com:5173`，登录 `wanghg11`。
2. 打开项目列表与任意项目。
3. 检查本地文件：`data/user-trace/cp/<今天>/<userId>.log` 与 `data/user-trace/dp/<今天>/<userId>.log` 存在且每行是 JSON。
4. 等待同步间隔（或临时把 `USER_TRACE_SYNC_INTERVAL` 设为 `30s`），确认日志文件被消费。
5. 查询数据库：

```sql
SELECT event_type, username, ip, request_method, request_path, status_code, device_id, event_at
FROM user_access_log
ORDER BY id DESC
LIMIT 20;
```

6. 访问 ops-web `http://vibebiz-dev-ops.com:5174/access-logs`，确认能看到登录与请求记录，筛选与分页可用。

- [ ] **Step 6: 检查点**

运行：

```bash
make checkpoint
```

记录完成项、未执行项（如无 MySQL 时的迁移）与手工验证结果。不创建 commit，除非用户明确要求。

---

## 自检

- Spec 覆盖：迁移/模型/配置（Task 1）、事件与 UA（Task 2）、Recorder（Task 3）、MySQL（Task 4）、Syncer（Task 5）、中间件与装配（Task 6）、登录（Task 7）、admin API（Task 8）、web 设备码（Task 9）、ops-web（Task 10）、迁移与验证（Task 11）。
- 无占位符：所有实现步骤包含具体代码或命令。
- 类型一致：`Event`、`Query`、`Page`、`Log`、`Queryer`、`Sink`、`Appender` 在任务间命名一致。
- 边界一致：只记录 `RealmUser`；登录失败入库；ops 登录不入库；区域留空。

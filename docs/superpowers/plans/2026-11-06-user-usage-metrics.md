# 用户按日使用指标与每日额度 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 按用户、按自然日记录项目/Token/模型请求/上传指标，提供 Prometheus 文本端点与 ECharts 趋势图，并在项目创建、聊天、上传链路执行每日额度校验与前端超限弹窗。

**Architecture:** CP 与 DP 共享 `internal/usage` 包，读写 MySQL `user_usage_daily`；sandbox 通过顶层 SSE `usage` 事件上报 Token 与模型请求，DP 记录；CP 项目创建和 DP 资源上传在业务成功后增量累加。`user_quota` 增加四个每日额度字段。CP 暴露 `/metrics/usage` 与 admin usage API，ops-web 用 ECharts 展示，web 用全局 Dialog 提示额度超限。

**Tech Stack:** Go 1.24 + Echo + GORM + goose + prometheus/client_golang；Python 3.13 + FastAPI + claude-agent-sdk；Vue 3 + PrimeVue + ECharts；MySQL 8。

**Spec:** `docs/superpowers/specs/2026-11-06-user-usage-metrics-design.md`

## Global Constraints

- 依赖方向：`web → CP/DP`，`ops-web → CP admin`，`CP/DP → sandbox`；禁止反向依赖。
- CP/DP 不解析 `agent.payload`；只允许识别新的顶层 SSE 事件 `type=usage`。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 每日额度 `0` 表示不限。
- 每日自然日按 `USAGE_TIMEZONE`，默认 `Asia/Shanghai`。
- 超限 HTTP 429，业务码 `42901` 项目、`42902` Token、`42903` 上传数量、`42904` 上传字节。
- 指标记录失败只写日志，不回滚已成功业务。
- `/metrics/usage` 无鉴权，只能暴露给内网/Prometheus。

---

### Task 1: 数据库迁移与模型

**Files:**
- Create: `packages/backend/migrations/00025_user_usage_daily.sql`
- Create: `packages/backend/internal/dao/model/usage.go`
- Modify: `packages/backend/internal/dao/model/workspace.go`
- Modify: `packages/backend/internal/dao/model/table_test.go`
- Modify: `packages/backend/migrations/migrations_test.go`

**Interfaces:**
- Produces: `model.UserUsageDaily`（字段 `UserID`, `UsageDate`, `ProjectsCreated`, `InputTokens`, `OutputTokens`, `ModelRequests`, `UploadCount`, `UploadBytes`）
- Produces: `model.UserQuota` 新增 `ProjectDailyQuota`, `LLMTokenDailyQuota`, `UploadDailyCountQuota`, `UploadDailyBytesQuota`

- [ ] **Step 1: 写迁移文件**

`00025_user_usage_daily.sql` 使用 spec 第 4.1 节的 Up/Down 与历史回填 SQL。Up 必须包含 `-- +goose Up`，Down 必须包含 `-- +goose Down`。

- [ ] **Step 2: 写模型**

`usage.go`：

```go
package model

import "time"

type UserUsageDaily struct {
	ID              int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID          int64     `gorm:"column:user_id" json:"userId"`
	UsageDate       time.Time `gorm:"column:usage_date;type:date" json:"usageDate"`
	ProjectsCreated int64     `gorm:"column:projects_created" json:"projectsCreated"`
	InputTokens     int64     `gorm:"column:input_tokens" json:"inputTokens"`
	OutputTokens    int64     `gorm:"column:output_tokens" json:"outputTokens"`
	ModelRequests   int64     `gorm:"column:model_requests" json:"modelRequests"`
	UploadCount     int64     `gorm:"column:upload_count" json:"uploadCount"`
	UploadBytes     int64     `gorm:"column:upload_bytes" json:"uploadBytes"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (UserUsageDaily) TableName() string { return "user_usage_daily" }
```

`workspace.go` 的 `UserQuota` 增加：

```go
ProjectDailyQuota     int64 `gorm:"column:project_daily_quota" json:"projectDailyQuota"`
LLMTokenDailyQuota    int64 `gorm:"column:llm_token_daily_quota" json:"llmTokenDailyQuota"`
UploadDailyCountQuota int64 `gorm:"column:upload_daily_count_quota" json:"uploadDailyCountQuota"`
UploadDailyBytesQuota int64 `gorm:"column:upload_daily_bytes_quota" json:"uploadDailyBytesQuota"`
```

- [ ] **Step 3: 更新表名测试**

`table_test.go` 增加 `{&UserUsageDaily{}, "user_usage_daily"}`。

- [ ] **Step 4: 更新迁移测试**

`migrations_test.go` 增加：

```go
func TestUserUsageDailyMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00025_user_usage_daily.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `user_usage_daily`", "project_daily_quota",
		"llm_token_daily_quota", "upload_daily_count_quota",
		"upload_daily_bytes_quota", "uk_user_usage_daily_user_date",
		"DROP TABLE IF EXISTS `user_usage_daily`",
		"-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("usage migration missing %q", want)
		}
	}
}
```

并在 `TestInitialSchemaCoversAllBusinessTables` 的表列表加入 `"user_usage_daily"`。

- [ ] **Step 5: 运行测试**

Run: `go test ./internal/dao/model ./migrations`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add packages/backend/migrations/00025_user_usage_daily.sql packages/backend/internal/dao/model/usage.go packages/backend/internal/dao/model/workspace.go packages/backend/internal/dao/model/table_test.go packages/backend/migrations/migrations_test.go
git commit -m "feat: add user usage daily schema"
```

---

### Task 2: `internal/usage` 仓储与服务

**Files:**
- Create: `packages/backend/internal/usage/repository.go`
- Create: `packages/backend/internal/usage/service.go`
- Test: `packages/backend/internal/usage/service_test.go`
- Test: `packages/backend/internal/usage/repository_test.go`

**Interfaces:**
- Consumes: `model.UserUsageDaily`, `model.UserQuota`
- Produces: `usage.Quota`, `usage.Usage`, `usage.Filter`, `usage.DailyRow`, `usage.QuotaExceededError`
- Produces: `usage.Repository` 接口
- Produces: `usage.NewService(repo Repository, loc *time.Location) *Service`
- Produces: `(*Service).CheckProjectCreate/RecordProjectCreate/CheckChat/RecordChatUsage/CheckUpload/RecordUpload/Series/Today/ParseDate`

- [ ] **Step 1: 写服务测试**

`service_test.go` 使用内存 fake：

```go
type fakeRepo struct {
	quota   Quota
	usage   Usage
	deltas  []Usage
	rows    []DailyRow
}

func (f *fakeRepo) Quota(context.Context, int64) (Quota, error) { return f.quota, nil }
func (f *fakeRepo) Usage(context.Context, int64, time.Time) (Usage, error) { return f.usage, nil }
func (f *fakeRepo) Increment(_ context.Context, _ int64, _ time.Time, delta Usage) error {
	f.deltas = append(f.deltas, delta)
	return nil
}
func (f *fakeRepo) Series(context.Context, Filter) ([]DailyRow, error) { return f.rows, nil }
```

测试用例：

- `TestCheckProjectCreateUnlimited`：quota 0，返回 nil。
- `TestCheckProjectCreateExceeded`：used 3 / limit 3，返回 `*QuotaExceededError`，`QuotaType=="project"`。
- `TestCheckChatExceeded`：input 60 + output 40 / limit 100。
- `TestCheckUploadCountExceeded`：used 4，add 1，limit 5。
- `TestCheckUploadBytesExceeded`：used 90，add 20，limit 100。
- `TestRecordMethodsIncrement`：调用三个 Record 方法，断言 deltas 为 `{ProjectsCreated:1}`、`{InputTokens:10,OutputTokens:5,ModelRequests:2}`、`{UploadCount:1,UploadBytes:100}`。
- `TestTodayUsesConfiguredLocation`：`Now` 返回 `2026-11-05T17:00:00Z`，loc=`Asia/Shanghai`，`Today()` 为 `2026-11-06`。

- [ ] **Step 2: 运行测试确认失败**

Run: `go test ./internal/usage`
Expected: FAIL（`undefined: Service` 等）

- [ ] **Step 3: 实现服务**

`service.go`：

```go
package usage

import (
	"context"
	"fmt"
	"time"
)

type Quota struct {
	ProjectDailyQuota     int64
	LLMTokenDailyQuota    int64
	UploadDailyCountQuota int64
	UploadDailyBytesQuota int64
}

type Usage struct {
	ProjectsCreated int64
	InputTokens     int64
	OutputTokens    int64
	ModelRequests   int64
	UploadCount     int64
	UploadBytes     int64
}

type Filter struct {
	From   time.Time
	To     time.Time
	UserID int64
}

type DailyRow struct {
	UserID          int64  `json:"userId"`
	Date            string `json:"date"`
	ProjectsCreated int64  `json:"projectsCreated"`
	InputTokens     int64  `json:"inputTokens"`
	OutputTokens    int64  `json:"outputTokens"`
	ModelRequests   int64  `json:"modelRequests"`
	UploadCount     int64  `json:"uploadCount"`
	UploadBytes     int64  `json:"uploadBytes"`
}

type QuotaExceededError struct {
	QuotaType string
	Used      int64
	Limit     int64
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("%s quota exceeded: %d/%d", e.QuotaType, e.Used, e.Limit)
}

type Service struct {
	Repo Repository
	Now  func() time.Time
	Loc  *time.Location
}

func NewService(repo Repository, loc *time.Location) *Service {
	if loc == nil {
		loc = time.UTC
	}
	return &Service{Repo: repo, Now: time.Now, Loc: loc}
}

func (s *Service) Today() time.Time {
	now := s.Now().In(s.Loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.Loc)
}

func (s *Service) ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, s.Loc)
}

func (s *Service) CheckProjectCreate(ctx context.Context, userID int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil { return err }
	if quota.ProjectDailyQuota <= 0 { return nil }
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil { return err }
	if used.ProjectsCreated >= quota.ProjectDailyQuota {
		return &QuotaExceededError{QuotaType: "project", Used: used.ProjectsCreated, Limit: quota.ProjectDailyQuota}
	}
	return nil
}

func (s *Service) CheckChat(ctx context.Context, userID int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil { return err }
	if quota.LLMTokenDailyQuota <= 0 { return nil }
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil { return err }
	total := used.InputTokens + used.OutputTokens
	if total >= quota.LLMTokenDailyQuota {
		return &QuotaExceededError{QuotaType: "token", Used: total, Limit: quota.LLMTokenDailyQuota}
	}
	return nil
}

func (s *Service) CheckUpload(ctx context.Context, userID, addCount, addBytes int64) error {
	quota, err := s.Repo.Quota(ctx, userID)
	if err != nil { return err }
	used, err := s.Repo.Usage(ctx, userID, s.Today())
	if err != nil { return err }
	if quota.UploadDailyCountQuota > 0 && used.UploadCount+addCount > quota.UploadDailyCountQuota {
		return &QuotaExceededError{QuotaType: "upload_count", Used: used.UploadCount, Limit: quota.UploadDailyCountQuota}
	}
	if quota.UploadDailyBytesQuota > 0 && used.UploadBytes+addBytes > quota.UploadDailyBytesQuota {
		return &QuotaExceededError{QuotaType: "upload_bytes", Used: used.UploadBytes, Limit: quota.UploadDailyBytesQuota}
	}
	return nil
}

func (s *Service) RecordProjectCreate(ctx context.Context, userID int64) error {
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{ProjectsCreated: 1})
}

func (s *Service) RecordChatUsage(ctx context.Context, userID, inputTokens, outputTokens, modelRequests int64) error {
	if inputTokens == 0 && outputTokens == 0 && modelRequests == 0 { return nil }
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{
		InputTokens: inputTokens, OutputTokens: outputTokens, ModelRequests: modelRequests,
	})
}

func (s *Service) RecordUpload(ctx context.Context, userID, count, bytes int64) error {
	if count == 0 && bytes == 0 { return nil }
	return s.Repo.Increment(ctx, userID, s.Today(), Usage{UploadCount: count, UploadBytes: bytes})
}

func (s *Service) Series(ctx context.Context, filter Filter) ([]DailyRow, error) {
	return s.Repo.Series(ctx, filter)
}
```

- [ ] **Step 4: 实现仓储**

`repository.go`：

```go
package usage

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type Repository interface {
	Quota(ctx context.Context, userID int64) (Quota, error)
	Usage(ctx context.Context, userID int64, date time.Time) (Usage, error)
	Increment(ctx context.Context, userID int64, date time.Time, delta Usage) error
	Series(ctx context.Context, filter Filter) ([]DailyRow, error)
}

type MySQLRepository struct{ db *gorm.DB }

func NewMySQLRepository(db *gorm.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (r *MySQLRepository) Quota(ctx context.Context, userID int64) (Quota, error) {
	var row model.UserQuota
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) { return Quota{}, nil }
	if err != nil { return Quota{}, err }
	return Quota{
		ProjectDailyQuota: row.ProjectDailyQuota,
		LLMTokenDailyQuota: row.LLMTokenDailyQuota,
		UploadDailyCountQuota: row.UploadDailyCountQuota,
		UploadDailyBytesQuota: row.UploadDailyBytesQuota,
	}, nil
}

func (r *MySQLRepository) Usage(ctx context.Context, userID int64, date time.Time) (Usage, error) {
	var row model.UserUsageDaily
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND usage_date = ?", userID, date.Format("2006-01-02")).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) { return Usage{}, nil }
	if err != nil { return Usage{}, err }
	return Usage{
		ProjectsCreated: row.ProjectsCreated, InputTokens: row.InputTokens,
		OutputTokens: row.OutputTokens, ModelRequests: row.ModelRequests,
		UploadCount: row.UploadCount, UploadBytes: row.UploadBytes,
	}, nil
}

func (r *MySQLRepository) Increment(ctx context.Context, userID int64, date time.Time, delta Usage) error {
	row := &model.UserUsageDaily{
		UserID: userID, UsageDate: date,
		ProjectsCreated: delta.ProjectsCreated, InputTokens: delta.InputTokens,
		OutputTokens: delta.OutputTokens, ModelRequests: delta.ModelRequests,
		UploadCount: delta.UploadCount, UploadBytes: delta.UploadBytes,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}, {Name: "usage_date"}},
		DoUpdates: clause.Assignments(map[string]any{
			"projects_created": gorm.Expr("projects_created + ?", delta.ProjectsCreated),
			"input_tokens":     gorm.Expr("input_tokens + ?", delta.InputTokens),
			"output_tokens":    gorm.Expr("output_tokens + ?", delta.OutputTokens),
			"model_requests":   gorm.Expr("model_requests + ?", delta.ModelRequests),
			"upload_count":     gorm.Expr("upload_count + ?", delta.UploadCount),
			"upload_bytes":     gorm.Expr("upload_bytes + ?", delta.UploadBytes),
		}),
	}).Create(row).Error
}

func (r *MySQLRepository) Series(ctx context.Context, filter Filter) ([]DailyRow, error) {
	query := r.db.WithContext(ctx).Model(&model.UserUsageDaily{}).
		Select("user_id, DATE_FORMAT(usage_date, '%Y-%m-%d') AS date, projects_created, input_tokens, output_tokens, model_requests, upload_count, upload_bytes").
		Where("usage_date >= ? AND usage_date <= ?", filter.From.Format("2006-01-02"), filter.To.Format("2006-01-02"))
	if filter.UserID > 0 {
		query = query.Where("user_id = ?", filter.UserID)
	}
	var rows []DailyRow
	err := query.Group("user_id, usage_date").
		Order("usage_date ASC, user_id ASC").Scan(&rows).Error
	return rows, err
}
```

- [ ] **Step 5: 仓储测试**

`repository_test.go` 复用 `sqlmock` 模式，覆盖：

- `Quota` 返回四个每日额度。
- `Usage` 无记录返回零值。
- `Increment` 生成 `INSERT INTO user_usage_daily ... ON DUPLICATE KEY UPDATE`。
- `Series` 全平台与按用户过滤的 SQL 参数。

- [ ] **Step 6: 运行测试**

Run: `go test ./internal/usage`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add packages/backend/internal/usage
git commit -m "feat: add usage repository and quota service"
```

---

### Task 3: CP 项目创建额度与 admin usage API

**Files:**
- Modify: `packages/backend/internal/response/response.go`
- Modify: `packages/backend/internal/handler/project.go`
- Modify: `packages/backend/internal/handler/project_test.go`
- Modify: `packages/backend/internal/admin/repository.go`
- Modify: `packages/backend/internal/admin/mysql.go`
- Modify: `packages/backend/internal/admin/memory.go`
- Modify: `packages/backend/internal/admin/mysql_test.go`
- Modify: `packages/backend/internal/handler/admin.go`
- Create: `packages/backend/internal/handler/usage_admin_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Produces: `response.FailWithResult(c echo.Context, status, code int, message string, result any) error`
- Produces: `handler.UsageGuard`（`CheckProjectCreate`, `RecordProjectCreate`）
- Produces: `handler.UsageReader`（`Series(ctx, usage.Filter) ([]usage.DailyRow, error)`）
- Produces: `admin.Quota` 新增四个每日额度字段
- Produces: `admin.QuotaUpdateInput` 改为指针字段，支持部分更新

- [ ] **Step 1: 写响应 helper 与项目测试**

`response.go`：

```go
func FailWithResult(c echo.Context, status, code int, message string, result any) error {
	return c.JSON(status, Envelope{Code: code, Message: message, Result: result})
}
```

`project_test.go` 增加 fake：

```go
type fakeUsageGuard struct {
	checkErr error
	recorded bool
}

func (f *fakeUsageGuard) CheckProjectCreate(context.Context, int64) error { return f.checkErr }
func (f *fakeUsageGuard) RecordProjectCreate(context.Context, int64) error {
	f.recorded = true
	return nil
}
```

测试：

- `TestCreateProjectRejectsDailyQuota`：`checkErr = &usage.QuotaExceededError{QuotaType:"project",Used:3,Limit:3}`，断言 429、code 42901、result 中 `quotaType=project`。
- `TestCreateProjectRecordsUsage`：创建成功，`recorded == true`。

- [ ] **Step 2: 实现项目额度**

`project.go` 增加：

```go
type UsageGuard interface {
	CheckProjectCreate(ctx context.Context, userID int64) error
	RecordProjectCreate(ctx context.Context, userID int64) error
}
```

`ProjectHandler` 增加 `Usage UsageGuard` 字段。

在 `Create` 中，工作区校验通过后、生成 UUID 前：

```go
if h.Usage != nil {
	if err := h.Usage.CheckProjectCreate(ctx, session.UserID); err != nil {
		var exceeded *usage.QuotaExceededError
		if errors.As(err, &exceeded) {
			return response.FailWithResult(c, http.StatusTooManyRequests, quotaCode(exceeded.QuotaType), "daily quota exceeded",
				map[string]any{"quotaType": exceeded.QuotaType, "used": exceeded.Used, "limit": exceeded.Limit})
		}
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "quota check failed")
	}
}
```

`ctx` 需要在之前定义；把 `ctx := c.Request().Context()` 提前到该检查之前。

在 `Create` 最后、`return response.Created(c, project)` 前：

```go
if h.Usage != nil {
	if err := h.Usage.RecordProjectCreate(ctx, session.UserID); err != nil && h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelWarn, "project.create", "project_id", id, "status", "usage_record_error", "error", err.Error())
	}
}
```

在 handler 内新增：

```go
func quotaCode(quotaType string) int {
	switch quotaType {
	case "project":
		return 42901
	case "token":
		return 42902
	case "upload_count":
		return 42903
	case "upload_bytes":
		return 42904
	default:
		return 42900
	}
}
```

- [ ] **Step 3: admin 类型与仓储**

`admin/repository.go`：

```go
type Quota struct {
	UserID                string `json:"userId"`
	Plan                  string `json:"plan"`
	LLMTokensQuota        int64  `json:"llmTokensQuota"`
	LLMTokensUsed         int64  `json:"llmTokensUsed"`
	SandboxMinutesQuota   int64  `json:"sandboxMinutesQuota"`
	SandboxMinutesUsed    int64  `json:"sandboxMinutesUsed"`
	StorageQuotaMB        int64  `json:"storageQuotaMb"`
	StorageUsedMB         int64  `json:"storageUsedMb"`
	ProjectDailyQuota     int64  `json:"projectDailyQuota"`
	LLMTokenDailyQuota    int64  `json:"llmTokenDailyQuota"`
	UploadDailyCountQuota int64  `json:"uploadDailyCountQuota"`
	UploadDailyBytesQuota int64  `json:"uploadDailyBytesQuota"`
}

type QuotaUpdateInput struct {
	LLMTokensQuota        *int64
	SandboxMinutesQuota   *int64
	StorageQuotaMB        *int64
	ProjectDailyQuota     *int64
	LLMTokenDailyQuota    *int64
	UploadDailyCountQuota *int64
	UploadDailyBytesQuota *int64
}
```

`admin/mysql.go`：

- `Quotas` 的 SELECT 增加四个字段并写入 `Quota`。
- `UpdateQuota` 构造 `updates := map[string]any{}`，每个非 nil 指针写入；为空直接返回 nil；`RowsAffected == 0` 仍走 `ensureQuotaExists`。

`admin/memory.go` 的 `UpdateQuota` 按非 nil 指针更新内存配额。

- [ ] **Step 4: admin handler usage API 与部分更新**

`admin.go` 增加：

```go
type UsageReader interface {
	Series(ctx context.Context, filter usage.Filter) ([]usage.DailyRow, error)
}

func (h *AdminHandler) UsageDaily(c echo.Context) error {
	from, to, err := parseUsageRange(c, h.Now)
	if err != nil { return response.Fail(c, http.StatusBadRequest, 40001, "invalid date range") }
	userID, _ := strconv.ParseInt(c.QueryParam("userId"), 10, 64)
	rows, err := h.Usage.Series(c.Request().Context(), usage.Filter{From: from, To: to, UserID: userID})
	if err != nil { return response.Fail(c, http.StatusInternalServerError, 50001, "read usage failed") }
	totals := usage.Usage{}
	for _, row := range rows { totals.ProjectsCreated += row.ProjectsCreated; /* ... */ }
	return response.OK(c, map[string]any{
		"from": from.Format("2006-01-02"), "to": to.Format("2006-01-02"),
		"userId": userID, "series": rows, "totals": totals,
	})
}
```

`parseUsageRange`：

- `from`/`to` 缺省今天。
- `time.ParseInLocation("2006-01-02", value, loc)`。
- `from > to` 或跨度 > 90 天返回错误。

`UpdateQuota` 请求改为全部指针，至少一个非 nil；把指针直接传给 `admin.QuotaUpdateInput`。

- [ ] **Step 5: 路由与 CP 装配**

`router/control.go` 的 `RegisterControl` 增加参数 `usageService *usage.Service`，并：

```go
projectHandler.Usage = usageService
adminHandler.Usage = usageService
...
adminProtected.GET("/usage/daily", adminHandler.UsageDaily)
```

`cmd/cp/main.go`：

```go
usageLoc, err := time.LoadLocation(cfg.UsageTimezone)
if err != nil {
	obs.Logger.Warn("invalid usage timezone, falling back to UTC", "value", cfg.UsageTimezone)
	usageLoc = time.UTC
}
usageService := usage.NewService(usage.NewMySQLRepository(db), usageLoc)
```

传给 `RegisterControl`。

`config.go` 增加 `UsageTimezone string`，`Load` 中：

```go
UsageTimezone: getString("USAGE_TIMEZONE", "Asia/Shanghai"),
```

`packages/backend/.env.example` 增加 `USAGE_TIMEZONE=Asia/Shanghai`。

- [ ] **Step 6: 更新 admin 测试**

`admin/mysql_test.go` 的 `UpdateQuota` 测试改用指针：

```go
input := QuotaUpdateInput{
	LLMTokensQuota: int64Ptr(500000),
	SandboxMinutesQuota: int64Ptr(600),
	StorageQuotaMB: int64Ptr(1024),
}
```

新增 helper `func int64Ptr(v int64) *int64 { return &v }`。

- [ ] **Step 7: 测试**

Run: `go test ./internal/handler ./internal/admin ./internal/router ./cmd/cp`
Expected: PASS

- [ ] **Step 8: 提交**

```bash
git add packages/backend/internal/response packages/backend/internal/handler/project.go packages/backend/internal/handler/project_test.go packages/backend/internal/admin packages/backend/internal/handler/admin.go packages/backend/internal/handler/usage_admin_test.go packages/backend/internal/router/control.go packages/backend/cmd/cp/main.go packages/backend/internal/config/config.go packages/backend/.env.example
git commit -m "feat: enforce daily project quota and expose usage admin api"
```

---

### Task 4: Prometheus `/metrics/usage`

**Files:**
- Create: `packages/backend/internal/handler/usage_metrics.go`
- Test: `packages/backend/internal/handler/usage_metrics_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Consumes: `usage.Service.Series`, `Today`, `ParseDate`
- Produces: `handler.NewUsageMetricsHandler(source UsageMetricsSource) *UsageMetricsHandler`
- Produces: `GET /metrics/usage`

- [ ] **Step 1: 写 handler 测试**

fake source 返回一行数据；httptest 请求 `/metrics/usage?from=2026-11-06&to=2026-11-06`，断言 body 包含：

```
vibebiz_user_daily_projects_created{date="2026-11-06",user_id="1"} 3
vibebiz_user_daily_input_tokens{date="2026-11-06",user_id="1"} 1234
```

并测试非法日期返回 400。

- [ ] **Step 2: 实现 handler**

`usage_metrics.go`：

```go
type UsageMetricsSource interface {
	Series(ctx context.Context, filter usage.Filter) ([]usage.DailyRow, error)
	Today() time.Time
	ParseDate(value string) (time.Time, error)
}

type UsageMetricsHandler struct {
	Source UsageMetricsSource
}

func NewUsageMetricsHandler(source UsageMetricsSource) *UsageMetricsHandler {
	return &UsageMetricsHandler{Source: source}
}

func (h *UsageMetricsHandler) Serve(c echo.Context) error {
	from := h.Source.Today()
	to := from
	var err error
	if raw := c.QueryParam("from"); raw != "" { from, err = h.Source.ParseDate(raw); if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid from") } }
	if raw := c.QueryParam("to"); raw != "" { to, err = h.Source.ParseDate(raw); if err != nil { return echo.NewHTTPError(http.StatusBadRequest, "invalid to") } }
	if to.Before(from) || to.Sub(from) > 31*24*time.Hour { return echo.NewHTTPError(http.StatusBadRequest, "invalid range") }
	userID, _ := strconv.ParseInt(c.QueryParam("user_id"), 10, 64)
	rows, err := h.Source.Series(c.Request().Context(), usage.Filter{From: from, To: to, UserID: userID})
	if err != nil { return echo.NewHTTPError(http.StatusInternalServerError, "read usage failed") }
	renderUsageMetrics(c, rows)
	return nil
}
```

`renderUsageMetrics` 创建 registry 与六个 GaugeVec，labels `user_id`, `date`，Set 后 `promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(c.Response(), c.Request())`。

- [ ] **Step 3: 装配 CP 路由**

`cmd/cp/main.go` 在 `server.New` 后：

```go
e.GET("/metrics/usage", handler.NewUsageMetricsHandler(usageService).Serve)
```

- [ ] **Step 4: 测试**

Run: `go test ./internal/handler -run UsageMetrics`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/handler/usage_metrics.go packages/backend/internal/handler/usage_metrics_test.go packages/backend/cmd/cp/main.go
git commit -m "feat: expose prometheus usage metrics endpoint"
```

---

### Task 5: DP 聊天额度与 usage 事件记录

**Files:**
- Modify: `packages/backend/internal/handler/chat.go`
- Modify: `packages/backend/internal/handler/chat_test.go`
- Modify: `packages/backend/internal/router/data.go`
- Modify: `packages/backend/cmd/dp/main.go`
- Modify: `packages/backend/internal/router/sandbox_route_test.go`
- Modify: `packages/backend/internal/router/helpers_test.go`

**Interfaces:**
- Produces: `handler.ChatUsageRecorder`（`CheckChat`, `RecordChatUsage`）
- Produces: `ChatHandler.Usage` 字段
- Consumes: `usage.Service`

- [ ] **Step 1: 写测试**

`chat_test.go` 增加 fake：

```go
type fakeChatUsage struct {
	checkErr error
	recorded bool
	input, output, requests int64
}

func (f *fakeChatUsage) CheckChat(context.Context, int64) error { return f.checkErr }
func (f *fakeChatUsage) RecordChatUsage(_ context.Context, _ int64, input, output, requests int64) error {
	f.recorded = true
	f.input, f.output, f.requests = input, output, requests
	return nil
}
```

测试：

- `TestChatRejectsDailyTokenQuota`：`checkErr=&usage.QuotaExceededError{QuotaType:"token",Used:100,Limit:100}`，断言 429 / 42902。
- `TestChatRecordsUsageEvent`：sandbox 返回 `data: {"type":"usage","inputTokens":10,"outputTokens":5,"modelRequests":2}\n\n` 与 `[DONE]`，断言 recorder 收到 10/5/2。
- `TestChatIgnoresAgentPayloadUsage`：只有 `agent` 事件里带 usage 字段时不记录。

- [ ] **Step 2: 实现 ChatHandler**

增加接口与字段：

```go
type ChatUsageRecorder interface {
	CheckChat(ctx context.Context, userID int64) error
	RecordChatUsage(ctx context.Context, userID, inputTokens, outputTokens, modelRequests int64) error
}

type ChatHandler struct {
	Sandbox ChatClient
	Roles   ProjectRoleResolver
	Obs     *observability.Provider
	Storage StorageSyncer
	Usage   ChatUsageRecorder
}
```

在 `Chat` 中获取 session，并在 `ctx` 得到后：

```go
session, ok := auth.SessionFrom(c)
if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
if h.Usage != nil {
	if err := h.Usage.CheckChat(ctx, session.UserID); err != nil {
		var exceeded *usage.QuotaExceededError
		if errors.As(err, &exceeded) {
			return response.FailWithResult(c, http.StatusTooManyRequests, 42902, "daily token quota exceeded",
				map[string]any{"quotaType": exceeded.QuotaType, "used": exceeded.Used, "limit": exceeded.Limit})
		}
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "quota check failed")
	}
}
```

在读取循环中，写响应前：

```go
if strings.HasPrefix(line, "data:") {
	if event, ok := parseUsageEvent(line); ok && h.Usage != nil {
		if err := h.Usage.RecordChatUsage(ctx, session.UserID, event.InputTokens, event.OutputTokens, event.ModelRequests); err != nil && h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelWarn, "chat.usage", "project_id", projectID, "status", "record_error", "error", err.Error())
		}
	}
	if h.Obs != nil { /* 现有 SSE 指标与日志 */ }
}
```

`parseUsageEvent`：

```go
type usageEvent struct {
	Type          string `json:"type"`
	InputTokens   int64  `json:"inputTokens"`
	OutputTokens  int64  `json:"outputTokens"`
	ModelRequests int64  `json:"modelRequests"`
}

func parseUsageEvent(line string) (usageEvent, bool) {
	payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	if payload == "" || payload == "[DONE]" { return usageEvent{}, false }
	var event usageEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil || event.Type != "usage" {
		return usageEvent{}, false
	}
	return event, true
}
```

- [ ] **Step 3: 路由与 DP 装配**

`RegisterData` 增加参数 `chatUsage handler.ChatUsageRecorder`，构造后 `chat.Usage = chatUsage`。

`cmd/dp/main.go` 构造 `usageService`（同 CP 的时区逻辑），传给 `RegisterData`。

更新两处 router 测试调用，末尾多传一个 `nil`。

- [ ] **Step 4: 测试**

Run: `go test ./internal/handler ./internal/router ./cmd/dp`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/handler/chat.go packages/backend/internal/handler/chat_test.go packages/backend/internal/router/data.go packages/backend/cmd/dp/main.go packages/backend/internal/router/sandbox_route_test.go packages/backend/internal/router/helpers_test.go
git commit -m "feat: record chat token usage and enforce daily token quota"
```

---

### Task 6: 资源上传额度与记录

**Files:**
- Modify: `packages/backend/internal/resource/service.go`
- Modify: `packages/backend/internal/resource/service_test.go`
- Modify: `packages/backend/internal/handler/resource.go`
- Modify: `packages/backend/internal/handler/resource_test.go`（如不存在则新建）
- Modify: `packages/backend/cmd/dp/main.go`

**Interfaces:**
- Produces: `resource.UsageGuard`（`CheckUpload`, `RecordUpload`）
- Produces: `Service.Usage` 与 `Service.Logger` 字段
- Consumes: `usage.Service`

- [ ] **Step 1: 写资源测试**

`service_test.go` 增加：

```go
type fakeUsage struct {
	checkErr error
	recorded bool
	count, bytes int64
}

func (f *fakeUsage) CheckUpload(context.Context, int64, int64, int64) error { return f.checkErr }
func (f *fakeUsage) RecordUpload(_ context.Context, _ int64, count, bytes int64) error {
	f.recorded = true; f.count = count; f.bytes = bytes; return nil
}
```

测试：

- `TestPresignRejectsDailyUploadQuota`：Usage 返回 `*usage.QuotaExceededError{QuotaType:"upload_count"}`，Presign 返回该错误。
- `TestAdoptRecordsUpload`：adopt 成功后 `recorded` 为 true，count=1，bytes=22。
- `TestAdoptReadyDoesNotRecordAgain`：先 adopt，再 adopt 同一资源，`recorded` 仍只 1 次（或记录值不变）。

- [ ] **Step 2: 实现资源服务**

`resource/service.go` 增加：

```go
type UsageGuard interface {
	CheckUpload(ctx context.Context, userID int64, addCount, addBytes int64) error
	RecordUpload(ctx context.Context, userID int64, count, bytes int64) error
}

type Service struct {
	Repo    Repository
	S3      S3
	Sandbox Sandbox
	Usage   UsageGuard
	Logger  *slog.Logger
	Expiry  time.Duration
	Now     func() time.Time
	NewID   func() string
}
```

`Presign` 在文件大小校验后、S3 presign 前：

```go
if s.Usage != nil {
	if err := s.Usage.CheckUpload(ctx, userID, 1, input.Size); err != nil {
		return PresignResult{}, err
	}
}
```

`Adopt` 在 ready 早退分支之后、S3 Head 前：

```go
if s.Usage != nil {
	if err := s.Usage.CheckUpload(ctx, userID, 1, item.SizeBytes); err != nil {
		return Item{}, err
	}
}
```

在最终 `return toItem(*item), nil` 前：

```go
if s.Usage != nil {
	if err := s.Usage.RecordUpload(ctx, userID, 1, item.SizeBytes); err != nil {
		logger := s.Logger
		if logger == nil { logger = slog.Default() }
		logger.Warn("resource.usage_record_error", "project_id", projectID, "resource_id", item.ResourceID, "error", err)
	}
}
```

- [ ] **Step 3: handler 错误映射**

`resourceError` 增加：

```go
var exceeded *usage.QuotaExceededError
if errors.As(err, &exceeded) {
	return response.FailWithResult(c, http.StatusTooManyRequests, quotaCode(exceeded.QuotaType), "daily quota exceeded",
		map[string]any{"quotaType": exceeded.QuotaType, "used": exceeded.Used, "limit": exceeded.Limit})
}
```

放在 `switch` 前。

- [ ] **Step 4: DP 装配**

`cmd/dp/main.go` 的 `resourceService` 构造后：

```go
resourceService.Usage = usageService
resourceService.Logger = obs.Logger
```

- [ ] **Step 5: 测试**

Run: `go test ./internal/resource ./internal/handler ./cmd/dp`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add packages/backend/internal/resource packages/backend/internal/handler/resource.go packages/backend/internal/handler/resource_test.go packages/backend/cmd/dp/main.go
git commit -m "feat: enforce daily upload quota and record usage"
```

---

### Task 7: sandbox usage 采集与 SSE 事件

**Files:**
- Modify: `packages/sandbox/app/services/claude_agent.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Test: `packages/sandbox/tests/test_claude_agent.py`
- Test: `packages/sandbox/tests/test_chat_turns.py`（新建）
- Test: `packages/sandbox/tests/test_api.py`

**Interfaces:**
- Produces: 顶层 SSE `{"type":"usage","inputTokens":...,"outputTokens":...,"modelRequests":...,"model":"...","turnId":"..."}`
- Produces: `TurnCollector.usage() -> dict`

- [ ] **Step 1: 写测试**

`test_claude_agent.py`：

```python
def test_serialize_assistant_includes_usage():
    message = AssistantMessage(
        content=[TextBlock(text="hi")],
        model="deepseek-chat",
        usage={"input_tokens": 12, "output_tokens": 3},
    )
    event = serialize_sdk_message(message)
    assert event["payload"]["usage"] == {"input_tokens": 12, "output_tokens": 3}


def test_serialize_result_includes_usage_and_model_usage():
    message = ResultMessage(
        subtype="success", duration_ms=1, duration_api_ms=1, is_error=False,
        num_turns=2, session_id="s", usage={"input_tokens": 10, "output_tokens": 4},
        model_usage={"m": {"inputTokens": 10, "outputTokens": 4}},
    )
    event = serialize_sdk_message(message)
    assert event["payload"]["usage"]["input_tokens"] == 10
    assert event["payload"]["modelUsage"]["m"]["inputTokens"] == 10
```

`test_chat_turns.py`：

```python
def test_collector_sums_assistant_usage():
    collector = TurnCollector()
    collector.consume({"type": "agent", "payload": {"type": "assistant", "message": {"content": [{"type": "text", "text": "a"}]}, "usage": {"input_tokens": 10, "output_tokens": 2}}})
    collector.consume({"type": "agent", "payload": {"type": "assistant", "message": {"content": [{"type": "text", "text": "b"}]}, "usage": {"inputTokens": 5, "outputTokens": 3}}})
    assert collector.usage() == {"inputTokens": 15, "outputTokens": 5, "modelRequests": 2}
```

`test_api.py` 断言 chat SSE 响应包含 `"type": "usage"` 且出现在 `[DONE]` 之前。

- [ ] **Step 2: 修改 claude_agent 序列化**

`AssistantMessage` 分支增加 `"usage": message.usage`。
`ResultMessage` 分支增加：

```python
"usage": message.usage,
"modelUsage": message.model_usage,
```

- [ ] **Step 3: 修改 TurnCollector**

增加字段：

```python
self.input_tokens = 0
self.output_tokens = 0
self.model_requests = 0
```

`_consume_assistant` 中：

```python
self.model_requests += 1
self._consume_usage(payload.get("usage"))
```

`_consume_result` 中：

```python
if self.input_tokens == 0 and self.output_tokens == 0:
    self._consume_usage(payload.get("usage"))
    self._consume_model_usage(payload.get("modelUsage") or payload.get("model_usage"))
```

新增：

```python
def _consume_usage(self, usage: Any) -> None:
    if not isinstance(usage, dict): return
    self.input_tokens += int(usage.get("input_tokens", usage.get("inputTokens", 0)) or 0)
    self.output_tokens += int(usage.get("output_tokens", usage.get("outputTokens", 0)) or 0)

def _consume_model_usage(self, model_usage: Any) -> None:
    if not isinstance(model_usage, dict): return
    for item in model_usage.values():
        if not isinstance(item, dict): continue
        self.input_tokens += int(item.get("inputTokens", item.get("input_tokens", 0)) or 0)
        self.output_tokens += int(item.get("outputTokens", item.get("output_tokens", 0)) or 0)

def usage(self) -> dict[str, int]:
    return {"inputTokens": self.input_tokens, "outputTokens": self.output_tokens, "modelRequests": self.model_requests}
```

- [ ] **Step 4: 广播 usage 事件**

`ChatTurnManager._run` 的 `finally` 中，在 `self._broadcast(turn, None)` 前：

```python
usage = collector.usage()
if usage["modelRequests"] > 0 or usage["inputTokens"] > 0 or usage["outputTokens"] > 0:
    self._broadcast(turn, {
        "type": "usage",
        **usage,
        "model": self.model,
        "turnId": turn.id,
    })
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/sandbox && .venv/bin/pytest tests/test_claude_agent.py tests/test_chat_turns.py tests/test_api.py -q`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add packages/sandbox/app/services/claude_agent.py packages/sandbox/app/services/chat_turns.py packages/sandbox/tests/test_claude_agent.py packages/sandbox/tests/test_chat_turns.py packages/sandbox/tests/test_api.py
git commit -m "feat: emit agent usage events from sandbox"
```

---

### Task 8: web 额度超限全局弹窗

**Files:**
- Modify: `packages/web/src/api/client.ts`
- Modify: `packages/web/src/api/client.test.ts`
- Modify: `packages/web/src/api/chat.ts`
- Modify: `packages/web/src/App.vue`
- Create: `packages/web/src/components/QuotaExceededDialog.vue`
- Create: `packages/web/src/components/QuotaExceededDialog.test.ts`

**Interfaces:**
- Produces: `QuotaExceededInfo`, `onQuotaExceeded(handler)`
- Produces: `QuotaExceededDialog.vue`

- [ ] **Step 1: 写 client 测试**

新增：

```ts
it('calls the quota handler on a 429 quota response', async () => {
  const handler = vi.fn()
  const dispose = onQuotaExceeded(handler)
  const client = axios.create({
    baseURL: 'http://test.local/cp/api/v1',
    adapter: async (config) => {
      throw Object.assign(new Error('quota'), {
        isAxiosError: true,
        config,
        response: {
          data: { code: 42901, message: 'daily project quota exceeded', result: { quotaType: 'project', used: 3, limit: 3 } },
          status: 429, statusText: 'Too Many Requests', headers: {}, config,
        },
      })
    },
  })
  await expect(request(client, { method: 'post', url: '/projects' })).rejects.toBeInstanceOf(ApiError)
  expect(handler).toHaveBeenCalledWith(expect.objectContaining({ code: 42901, quotaType: 'project', used: 3, limit: 3 }))
  dispose()
})
```

- [ ] **Step 2: 实现 client**

`client.ts`：

```ts
export type QuotaType = 'project' | 'token' | 'upload_count' | 'upload_bytes' | 'unknown'

export interface QuotaExceededInfo {
  message: string
  code: number
  quotaType: QuotaType
  used?: number
  limit?: number
}

const QUOTA_CODES = new Set([42901, 42902, 42903, 42904])
let quotaExceededHandler: ((info: QuotaExceededInfo) => void) | null = null

export function onQuotaExceeded(handler: (info: QuotaExceededInfo) => void): () => void {
  quotaExceededHandler = handler
  return () => { if (quotaExceededHandler === handler) quotaExceededHandler = null }
}

function notifyQuotaExceeded(message: string, status: number, code: number, result?: unknown): void {
  if (status !== 429 || !QUOTA_CODES.has(code)) return
  const detail = (result ?? {}) as { quotaType?: QuotaType; used?: number; limit?: number }
  quotaExceededHandler?.({
    message,
    code,
    quotaType: detail.quotaType ?? 'unknown',
    used: detail.used,
    limit: detail.limit,
  })
}
```

`request` 的 axios 错误分支在抛出 `ApiError` 前调用 `notifyQuotaExceeded(...)`。

`chat.ts` 的 `streamChat` 非 2xx：

```ts
if (!response.ok || !response.body) {
  const text = await response.text().catch(() => '')
  if (response.status === 429) {
    try {
      const envelope = JSON.parse(text) as { code?: number; message?: string; result?: unknown }
      notifyQuotaExceeded(envelope.message ?? 'quota exceeded', 429, envelope.code ?? -1, envelope.result)
    } catch {
      // ignore malformed body
    }
  }
  throw new Error(text || `chat failed (${response.status})`)
}
```

`notifyQuotaExceeded` 从 `client.ts` 导出。

- [ ] **Step 3: 弹窗组件**

`QuotaExceededDialog.vue`：

```vue
<script setup lang="ts">
import { computed } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'

import type { QuotaExceededInfo } from '@/api/client'

const props = defineProps<{ visible: boolean; info: QuotaExceededInfo | null }>()
const emit = defineEmits<{ (event: 'update:visible', value: boolean): void }>()

const title = '额度已用完'
const description = computed(() => {
  switch (props.info?.quotaType) {
    case 'project': return '今日创建项目数已超出限额。'
    case 'token': return '今日 Token 使用量已超出限额。'
    case 'upload_count': return '今日上传文件数已超出限额。'
    case 'upload_bytes': return '今日上传文件大小已超出限额。'
    default: return '今日使用额度已超出限额。'
  }
})
const usage = computed(() => {
  if (props.info?.used === undefined || props.info?.limit === undefined) return ''
  return `当前：${props.info.used} / ${props.info.limit}`
})
</script>

<template>
  <Dialog :visible="props.visible" :header="title" modal :style="{ width: '420px' }"
    @update:visible="emit('update:visible', $event)">
    <div class="quota-dialog">
      <p class="quota-dialog__desc">{{ description }}</p>
      <p v-if="usage" class="quota-dialog__usage">{{ usage }}</p>
      <p class="quota-dialog__hint">请联系管理员调整配额后再试。</p>
    </div>
    <template #footer>
      <Button label="我知道了" @click="emit('update:visible', false)" />
    </template>
  </Dialog>
</template>

<style scoped>
.quota-dialog { display: flex; flex-direction: column; gap: 10px; }
.quota-dialog__desc { margin: 0; color: var(--text); font-size: 14px; }
.quota-dialog__usage { margin: 0; color: var(--muted); font-size: 13px; }
.quota-dialog__hint { margin: 0; color: var(--muted); font-size: 13px; }
</style>
```

- [ ] **Step 4: App 注册**

`App.vue`：

```vue
<script setup lang="ts">
import { onBeforeUnmount, ref } from 'vue'
import Toast from 'primevue/toast'

import { onQuotaExceeded, type QuotaExceededInfo } from '@/api/client'
import QuotaExceededDialog from '@/components/QuotaExceededDialog.vue'

const quotaVisible = ref(false)
const quotaInfo = ref<QuotaExceededInfo | null>(null)
const disposeQuota = onQuotaExceeded((info) => {
  quotaInfo.value = info
  quotaVisible.value = true
})
onBeforeUnmount(disposeQuota)
</script>

<template>
  <router-view />
  <Toast />
  <QuotaExceededDialog v-model:visible="quotaVisible" :info="quotaInfo" />
</template>
```

- [ ] **Step 5: 测试**

Run: `cd packages/web && npm test`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add packages/web/src/api/client.ts packages/web/src/api/client.test.ts packages/web/src/api/chat.ts packages/web/src/App.vue packages/web/src/components/QuotaExceededDialog.vue packages/web/src/components/QuotaExceededDialog.test.ts
git commit -m "feat: show quota exceeded dialog in web"
```

---

### Task 9: ops-web ECharts 概览与配额页

**Files:**
- Modify: `packages/ops-web/package.json`
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/utils/usage.ts`
- Create: `packages/ops-web/src/utils/usage.test.ts`
- Create: `packages/ops-web/src/components/UsageTrendChart.vue`
- Modify: `packages/ops-web/src/views/OverviewView.vue`
- Modify: `packages/ops-web/src/views/QuotasView.vue`

**Interfaces:**
- Consumes: `GET /admin/usage/daily`, `GET /admin/users`, `GET/PUT /admin/quotas`
- Produces: `getUsageDaily`, `AdminUsageDaily`, `UsageDailyPoint`
- Produces: `UsageTrendChart.vue` props `{ option: EChartsOption; height?: string }`

- [ ] **Step 1: 安装 echarts**

```bash
cd packages/ops-web && npm install echarts
```

`package.json` dependencies 增加 `"echarts": "^5.5.1"`。

- [ ] **Step 2: 写纯函数测试**

`usage.test.ts`：

```ts
import { describe, expect, it } from 'vitest'
import { dateRange, fillUsageSeries } from './usage'

describe('dateRange', () => {
  it('returns inclusive dates', () => {
    expect(dateRange(new Date('2026-11-04T00:00:00Z'), new Date('2026-11-06T00:00:00Z')))
      .toEqual(['2026-11-04', '2026-11-05', '2026-11-06'])
  })
})

describe('fillUsageSeries', () => {
  it('fills missing dates with zeros', () => {
    const rows = [{ userId: 1, date: '2026-11-05', projectsCreated: 2, inputTokens: 10, outputTokens: 5, modelRequests: 1, uploadCount: 1, uploadBytes: 100 }]
    const filled = fillUsageSeries(rows, ['2026-11-04', '2026-11-05'])
    expect(filled.map((item) => item.date)).toEqual(['2026-11-04', '2026-11-05'])
    expect(filled[0].projectsCreated).toBe(0)
    expect(filled[1].projectsCreated).toBe(2)
  })
})
```

- [ ] **Step 3: 实现工具与类型**

`types/index.ts`：

```ts
export interface UsageDailyPoint {
  date: string
  projectsCreated: number
  inputTokens: number
  outputTokens: number
  modelRequests: number
  uploadCount: number
  uploadBytes: number
}

export interface UsageDailyResult {
  from: string
  to: string
  userId: number
  series: UsageDailyPoint[]
  totals: Omit<UsageDailyPoint, 'date'>
}
```

`api/admin.ts`：

```ts
export async function getUsageDaily(query: { from: string; to: string; userId?: number }): Promise<UsageDailyResult> {
  return request<UsageDailyResult>({
    method: 'get',
    url: '/usage/daily',
    params: { from: query.from, to: query.to, userId: query.userId || undefined },
  })
}
```

`utils/usage.ts` 提供 `dateRange(start, end)`、`fillUsageSeries(rows, dates)`、`buildUsageOption` 四个 option builder（项目、Token、模型请求、上传）。

`UsageTrendChart.vue`：

```vue
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'

const props = withDefaults(defineProps<{ option: EChartsOption; height?: string }>(), { height: '280px' })
const container = ref<HTMLDivElement | null>(null)
let chart: echarts.ECharts | null = null
let observer: ResizeObserver | null = null

onMounted(() => {
  if (!container.value) return
  chart = echarts.init(container.value)
  chart.setOption(props.option)
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(container.value)
})

watch(() => props.option, (option) => chart?.setOption(option, true), { deep: true })

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="container" class="usage-chart" :style="{ height: props.height }" />
</template>

<style scoped>
.usage-chart { width: 100%; }
</style>
```

- [ ] **Step 4: 概览页**

`OverviewView.vue` 增加：

- `rangeDays = ref(30)`、`selectedUserId = ref<number | null>(null)`、`users = ref<AdminUser[]>([])`、`usage = ref<UsageDailyResult | null>(null)`、`usageError = ref('')`。
- `load()` 中并行请求 `getOverview`、`getProjects`、`getUsageDaily`、`getUsers({ page: 1, pageSize: 100 })`。
- `computed` 生成 dates 与四个 option，分别传给四个 `UsageTrendChart`。
- 顶部筛选：`Button` 7/30/90 天，`Dropdown` 用户；变化时重新加载。

- [ ] **Step 5: 配额页**

`QuotasView.vue`：

- `AdminQuota` 类型新增四个每日额度。
- 列增加每日项目数、每日 Token、每日上传数量、每日上传字节。
- 编辑表单 `QuotaUpdateInput` 增加四个字段；`openEdit` 与 `submit` 传递。
- 文案说明 `0 表示不限`。

- [ ] **Step 6: 测试与构建**

Run: `cd packages/ops-web && npm test && npm run build`
Expected: PASS

- [ ] **Step 7: 提交**

```bash
git add packages/ops-web/package.json packages/ops-web/package-lock.json packages/ops-web/src/types/index.ts packages/ops-web/src/api/admin.ts packages/ops-web/src/utils/usage.ts packages/ops-web/src/utils/usage.test.ts packages/ops-web/src/components/UsageTrendChart.vue packages/ops-web/src/views/OverviewView.vue packages/ops-web/src/views/QuotasView.vue
git commit -m "feat: add usage charts and daily quotas to ops-web"
```

---

### Task 10: 契约文档与全量验证

**Files:**
- Modify: `.ai/rules/architecture.md`
- Modify: `.ai/CURRENT_FOCUS`
- Optionally modify: `README.md`

**Interfaces:**
- Documents: `/metrics/usage`、SSE `usage` 事件、`user_usage_daily`、每日额度、429 错误码

- [ ] **Step 1: 更新架构文档**

在 `.ai/rules/architecture.md`：

- Section 9 可观测性：增加 `/metrics/usage` 指标名。
- Section 4 SSE 契约：增加顶层 `usage` 事件示例。
- Section 3 CP/DP API：增加 `GET /metrics/usage` 与 `GET /admin/usage/daily`。
- Section 5 Sandbox 工作区规则或数据库说明：增加 `user_usage_daily` 与每日额度字段。
- 错误码表补充 42901-42904。

- [ ] **Step 2: 运行全量测试**

```bash
make test
make validate
```

Expected: 全部 PASS。

- [ ] **Step 3: 保存 checkpoint**

```bash
make checkpoint
```

- [ ] **Step 4: 提交**

```bash
git add .ai/rules/architecture.md .ai/CURRENT_FOCUS README.md
git commit -m "docs: document usage metrics contracts"
```

---

## Self-Review

- Spec 第 1 节日标 1-6 分别由 Task 1/2/3/4/5/6/7/8/9 覆盖。
- 每日额度四个字段、`user_usage_daily`、历史回填由 Task 1 覆盖。
- `/metrics/usage` 由 Task 4 覆盖。
- ops-web 时间/用户筛选与四图表由 Task 9 覆盖。
- 额度超限弹窗由 Task 8 覆盖。
- 所有接口签名、字段名、错误码在任务间保持一致。

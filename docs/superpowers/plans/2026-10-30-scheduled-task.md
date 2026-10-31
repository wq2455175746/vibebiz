# 定时任务模块 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现用户定时任务：AI/手动创建、固定项目、数据库到期扫描调度、sandbox Agent 执行、站内通知，以及 web/ops-web 管理界面。

**Architecture:** 复用现有 `scheduled_task` 表。所有 CP 副本注册一个 `@every 30s` 的 asynq dispatcher，用 `asynq.Unique` 去重，扫描 `next_run_at` 到期任务并原子抢占，再入队 `scheduled_task:run`。runner 从固定项目读取最近 5 条聊天记录，组合任务提示词和 `scheduled` 主题系统提示词，调用 sandbox Agent chat。sandbox 新增自然语言任务解析接口，并让 chat 支持不落历史记录的 `systemPrompt`/`context`。

**Tech Stack:** Go 1.24、Echo、GORM/MySQL、asynq、Redis、robfig/cron、Python FastAPI、httpx、Vue 3、Pinia、Vitest、PrimeVue。

**Spec:** `docs/superpowers/specs/2026-10-30-scheduled-task-design.md`

## Global Constraints

- 复用现有 `scheduled_task` 表；不新增 `scheduled_task_run` 表。
- `scheduled_task.task_type` 固定为 `agent`。
- 每个任务创建一个固定项目，创建任务时立即创建；后续触发不新建项目。
- 任务创建成功后立即入队执行一次。
- 所有任务项目主题指向 `project_topic.topic_code='scheduled'`。
- 调度不使用 per-task asynq Scheduler entry，不做 leader 选举。
- dispatcher 为 `@every 30s` + `asynq.Unique(25s)`；运行任务加 `asynq.Unique(50s)` 和 Redis 执行锁。
- 执行上下文固定为：任务提示词 + topic 系统提示词 + 固定项目最近 5 条 message。
- 用户可编辑、暂停、恢复、停止、删除自己的任务；ops 只能查看、暂停、恢复、停止，不能编辑或删除。
- 删除任务时用户可选择是否删除项目；ops 不能删除。
- 颜色仅使用黑 / 白 / 灰 / 蓝。
- 不写无意义注释；公共函数需要说明时写一行。
- 不提交 git，除非用户明确要求。每个任务结束只做测试和 `make checkpoint`。

---

### Task 1: 主题种子与配置

**Files:**
- Create: `packages/backend/migrations/00020_scheduled_task_topic.sql`
- Modify: `packages/backend/internal/config/config.go`
- Test: `packages/backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: 现有 goose migration 自动加载 `migrations/*.sql`。
- Produces: `Config.ScheduledDispatchInterval time.Duration`、`Config.ScheduledRunTimeout time.Duration`。

- [ ] **Step 1: 写迁移文件**

```sql
-- +goose Up

INSERT INTO `project_topic`
  (`topic_code`, `topic_name`, `description`, `prompt_template`, `is_default`, `sort_order`, `status`)
VALUES
  ('scheduled', '定时任务', '用户定时任务专用主题',
   '你是定时任务执行助手。你会收到任务描述和最近对话上下文。请先检查固定项目的当前文件与状态，再直接执行任务，不要向用户提问。完成后用简洁中文说明本次执行结果、修改的文件和需要用户注意的风险。',
   0, 90, 1)
ON DUPLICATE KEY UPDATE
  `topic_name` = VALUES(`topic_name`),
  `description` = VALUES(`description`),
  `prompt_template` = VALUES(`prompt_template`),
  `status` = VALUES(`status`);

-- +goose Down

DELETE FROM `project_topic` WHERE `topic_code` = 'scheduled';
```

- [ ] **Step 2: 写配置失败测试**

```go
func TestScheduledTaskDefaults(t *testing.T) {
	t.Setenv("SCHEDULED_DISPATCH_INTERVAL", "")
	t.Setenv("SCHEDULED_RUN_TIMEOUT", "")
	cfg := Load("cp")
	if cfg.ScheduledDispatchInterval != 30*time.Second {
		t.Fatalf("dispatch interval = %s", cfg.ScheduledDispatchInterval)
	}
	if cfg.ScheduledRunTimeout != 15*time.Minute {
		t.Fatalf("run timeout = %s", cfg.ScheduledRunTimeout)
	}
}
```

- [ ] **Step 3: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/config -run TestScheduledTaskDefaults -v
```

Expected: FAIL，`cfg.ScheduledDispatchInterval` 字段不存在。

- [ ] **Step 4: 添加配置字段**

在 `Config` 结构加入：

```go
ScheduledDispatchInterval time.Duration
ScheduledRunTimeout       time.Duration
```

在 `Load` 返回结构加入：

```go
ScheduledDispatchInterval: getDuration("SCHEDULED_DISPATCH_INTERVAL", 30*time.Second),
ScheduledRunTimeout:       getDuration("SCHEDULED_RUN_TIMEOUT", 15*time.Minute),
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/config -v
```

- [ ] **Step 6: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 2: 调度表达式与领域类型

**Files:**
- Create: `packages/backend/internal/scheduledtask/service.go`
- Test: `packages/backend/internal/scheduledtask/service_test.go`

**Interfaces:**
- Consumes: `github.com/robfig/cron/v3`。
- Produces:
  - `NormalizeSchedule(input ScheduleInput) (Schedule, error)`
  - `NextRun(schedule Schedule, now time.Time) (time.Time, error)`
  - `Task`、`Draft`、`CreateInput`、`UpdateInput`、`ListFilter`、`Page`
  - 常量 `TopicCodeScheduled`、`ScheduleDaily`、`ScheduleWeekly`、`ScheduleCron`、`SourceAI`、`SourceManual`
  - 状态常量 `StatusPending/Running/Success/Failed/Stopped`

- [ ] **Step 1: 写失败测试**

```go
package scheduledtask

import (
	"testing"
	"time"
)

func TestNormalizeScheduleDaily(t *testing.T) {
	got, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleDaily, Time: "09:05", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpr != "5 9 * * *" || got.Type != ScheduleDaily {
		t.Fatalf("schedule = %+v", got)
	}
}

func TestNormalizeScheduleWeekly(t *testing.T) {
	got, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleWeekly, Time: "18:30", Weekdays: []int{1, 3, 5}, Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.CronExpr != "30 18 * * 1,3,5" {
		t.Fatalf("cron = %q", got.CronExpr)
	}
}

func TestNormalizeScheduleRejectsBadInput(t *testing.T) {
	cases := []ScheduleInput{
		{Type: ScheduleDaily, Time: "25:00", Timezone: "Asia/Shanghai"},
		{Type: ScheduleDaily, Time: "09:00", Timezone: "Bad/Zone"},
		{Type: ScheduleCron, CronExpression: "* * *", Timezone: "Asia/Shanghai"},
		{Type: ScheduleWeekly, Time: "09:00", Weekdays: nil, Timezone: "Asia/Shanghai"},
	}
	for _, input := range cases {
		if _, err := NormalizeSchedule(input); err == nil {
			t.Fatalf("expected error for %+v", input)
		}
	}
}

func TestNextRunUsesTimezone(t *testing.T) {
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: ScheduleDaily, Time: "09:00", Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	next, err := NextRun(schedule, now)
	if err != nil {
		t.Fatal(err)
	}
	if next.Format(time.RFC3339) != "2026-10-30T01:00:00Z" {
		t.Fatalf("next = %s", next.Format(time.RFC3339))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run 'TestNormalize|TestNextRun' -v
```

Expected: FAIL，包不存在。

- [ ] **Step 3: 写领域类型与调度实现**

```go
package scheduledtask

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	TopicCodeScheduled = "scheduled"
	TaskTypeAgent      = "agent"
	ScheduleDaily      = "daily"
	ScheduleWeekly     = "weekly"
	ScheduleCron       = "cron"
	SourceAI           = "ai"
	SourceManual       = "manual"
)

const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
	StatusStopped = "stopped"
)

var (
	ErrInvalid   = errors.New("invalid scheduled task")
	ErrNotFound  = errors.New("scheduled task not found")
	ErrForbidden = errors.New("scheduled task forbidden")
	ErrBusy      = errors.New("scheduled task project busy")
)

type ScheduleInput struct {
	Type           string
	Time           string
	Weekdays       []int
	CronExpression string
	Timezone       string
}

type Schedule struct {
	Type       string
	Time       string
	Weekdays   []int
	CronExpr   string
	Timezone   string
	Location   *time.Location
}

type Draft struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ScheduleType   string `json:"scheduleType"`
	Time           string `json:"time"`
	Weekdays       []int  `json:"weekdays"`
	CronExpression string `json:"cronExpression"`
	Timezone       string `json:"timezone"`
}

type CreateInput struct {
	Name           string
	Description    string
	ScheduleType   string
	Time           string
	Weekdays       []int
	CronExpression string
	Timezone       string
	Source         string
}

type UpdateInput struct {
	Name           string
	Description    string
	ScheduleType   string
	Time           string
	Weekdays       []int
	CronExpression string
	Timezone       string
}

type ListFilter struct {
	Keyword  string
	Enabled  *bool
	Page     int
	PageSize int
}

type Page struct {
	Items    []Task `json:"items"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type Task struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Description     string  `json:"description"`
	ScheduleType    string  `json:"scheduleType"`
	Time            string  `json:"time"`
	Weekdays        []int   `json:"weekdays"`
	CronExpression  string  `json:"cronExpression"`
	Timezone        string  `json:"timezone"`
	Source          string  `json:"source"`
	Enabled         bool    `json:"enabled"`
	ProjectID       string  `json:"projectId"`
	NextRunAt       *string `json:"nextRunAt,omitempty"`
	LastRunAt       *string `json:"lastRunAt,omitempty"`
	LastStatus      string  `json:"lastStatus"`
	LastError       string  `json:"lastError"`
	RunCount        int64   `json:"runCount"`
	UserID          int64   `json:"userId,omitempty"`
	Username        string  `json:"username,omitempty"`
	WorkspaceID     int64   `json:"workspaceId,omitempty"`
}

func NormalizeSchedule(input ScheduleInput) (Schedule, error) {
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = "Asia/Shanghai"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return Schedule{}, ErrInvalid
	}
	switch input.Type {
	case ScheduleDaily:
		hour, minute, err := parseClock(input.Time)
		if err != nil {
			return Schedule{}, err
		}
		return Schedule{
			Type: ScheduleDaily, Time: input.Time, Timezone: timezone, Location: location,
			CronExpr: fmt.Sprintf("%d %d * * *", minute, hour),
		}, nil
	case ScheduleWeekly:
		hour, minute, err := parseClock(input.Time)
		if err != nil {
			return Schedule{}, err
		}
		days, err := normalizeWeekdays(input.Weekdays)
		if err != nil {
			return Schedule{}, err
		}
		return Schedule{
			Type: ScheduleWeekly, Time: input.Time, Weekdays: days, Timezone: timezone, Location: location,
			CronExpr: fmt.Sprintf("%d %d * * %s", minute, hour, joinInts(days)),
		}, nil
	case ScheduleCron:
		expr := strings.TrimSpace(input.CronExpression)
		if expr == "" {
			return Schedule{}, ErrInvalid
		}
		if _, err := cron.ParseStandard("CRON_TZ=" + timezone + " " + expr); err != nil {
			return Schedule{}, ErrInvalid
		}
		return Schedule{Type: ScheduleCron, CronExpr: expr, Timezone: timezone, Location: location}, nil
	default:
		return Schedule{}, ErrInvalid
	}
}

func NextRun(schedule Schedule, now time.Time) (time.Time, error) {
	parser, err := cron.ParseStandard("CRON_TZ=" + schedule.Timezone + " " + schedule.CronExpr)
	if err != nil {
		return time.Time{}, ErrInvalid
	}
	next := parser.Next(now)
	if next.IsZero() {
		return time.Time{}, ErrInvalid
	}
	return next.UTC(), nil
}

func parseClock(value string) (int, int, error) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, 0, ErrInvalid
	}
	var hour, minute int
	if _, err := fmt.Sscanf(value, "%02d:%02d", &hour, &minute); err != nil {
		return 0, 0, ErrInvalid
	}
	if hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, 0, ErrInvalid
	}
	return hour, minute, nil
}

func normalizeWeekdays(values []int) ([]int, error) {
	if len(values) == 0 {
		return nil, ErrInvalid
	}
	seen := map[int]bool{}
	out := make([]int, 0, len(values))
	for _, value := range values {
		if value < 0 || value > 6 || seen[value] {
			return nil, ErrInvalid
		}
		seen[value] = true
		out = append(out, value)
	}
	return out, nil
}

func joinInts(values []int) string {
	parts := make([]string, len(values))
	for index, value := range values {
		parts[index] = fmt.Sprintf("%d", value)
	}
	return strings.Join(parts, ",")
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 3: 服务层 CRUD 与固定项目

**Files:**
- Modify: `packages/backend/internal/scheduledtask/service.go`
- Test: `packages/backend/internal/scheduledtask/service_test.go`

**Interfaces:**
- Consumes: `store.ProjectStore`、`auth.ProjectMetaStore`、`daomodel.ScheduledTask`。
- Produces:
  - `Repository` interface
  - `ProjectInitializer` interface
  - `MetaStore` interface
  - `StopChatClient` interface
  - `Service` 与 `NewService`
  - `Service.Create/List/Get/Update/Pause/Resume/Stop/Delete/AdminList`

- [ ] **Step 1: 写失败测试**

```go
type fakeRepo struct {
	items  []daomodel.ScheduledTask
	topics []daomodel.ProjectTopic
	nextID int64
}

func (f *fakeRepo) Create(_ context.Context, item *daomodel.ScheduledTask) error {
	f.nextID++
	item.ID = f.nextID
	f.items = append(f.items, *item)
	return nil
}
func (f *fakeRepo) Get(_ context.Context, id int64) (*daomodel.ScheduledTask, error) {
	for index := range f.items {
		if f.items[index].ID == id {
			return &f.items[index], nil
		}
	}
	return nil, ErrNotFound
}
func (f *fakeRepo) ListByUser(context.Context, int64, []int64) ([]daomodel.ScheduledTask, error) {
	return f.items, nil
}
func (f *fakeRepo) List(context.Context, ListFilter) ([]daomodel.ScheduledTask, int64, error) {
	return f.items, int64(len(f.items)), nil
}
func (f *fakeRepo) Update(_ context.Context, id int64, fields map[string]any) error {
	for index := range f.items {
		if f.items[index].ID == id {
			if value, ok := fields["name"].(string); ok { f.items[index].Name = value }
			if value, ok := fields["prompt"].(string); ok { f.items[index].Prompt = value }
			return nil
		}
	}
	return ErrNotFound
}
func (f *fakeRepo) Delete(_ context.Context, id int64) error {
	for index := range f.items {
		if f.items[index].ID == id {
			f.items = append(f.items[:index], f.items[index+1:]...)
			return nil
		}
	}
	return ErrNotFound
}
func (f *fakeRepo) Due(context.Context, time.Time, int) ([]daomodel.ScheduledTask, error) { return nil, nil }
func (f *fakeRepo) Claim(context.Context, int64, time.Time, time.Time, time.Time) (bool, error) { return true, nil }
func (f *fakeRepo) TopicByCode(_ context.Context, code string) (*daomodel.ProjectTopic, error) {
	for index := range f.topics {
		if f.topics[index].TopicCode == code {
			return &f.topics[index], nil
		}
	}
	return nil, ErrInvalid
}
func (f *fakeRepo) UserNames(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{}, nil
}

type fakeInitializer struct{ id int64; err error }
func (f *fakeInitializer) InitProject(context.Context, string) (int64, error) { return f.id, f.err }

type fakeMetas struct{ set map[string]auth.ProjectMeta }
func (f *fakeMetas) Set(_ context.Context, id string, meta auth.ProjectMeta) error {
	if f.set == nil { f.set = map[string]auth.ProjectMeta{} }
	f.set[id] = meta
	return nil
}
func (f *fakeMetas) Delete(_ context.Context, id string) error { delete(f.set, id); return nil }

func newServiceFixture(t *testing.T) (*Service, *fakeRepo, *fakeInitializer, *store.MemoryProjectStore, *fakeMetas) {
	t.Helper()
	repo := &fakeRepo{topics: []daomodel.ProjectTopic{{Base: daomodel.Base{ID: 9}, TopicCode: TopicCodeScheduled, TopicName: "定时任务"}}}
	projects := store.NewMemoryProjectStore()
	initializer := &fakeInitializer{id: 77}
	metas := &fakeMetas{}
	service := NewService(repo, projects, initializer, metas, nil, nil)
	service.Now = func() time.Time { return time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC) }
	service.NewID = func() string { return "task-project-1" }
	return service, repo, initializer, projects, metas
}

func TestCreateScheduledTaskCreatesProject(t *testing.T) {
	service, repo, _, projects, metas := newServiceFixture(t)
	view, err := service.Create(context.Background(), CreateInput{
		Name: "每日检查", Description: "检查依赖", ScheduleType: ScheduleDaily,
		Time: "09:00", Timezone: "Asia/Shanghai", Source: SourceManual,
	}, 7, 5)
	if err != nil {
		t.Fatal(err)
	}
	if view.ProjectID != "task-project-1" || view.Name != "每日检查" {
		t.Fatalf("view = %+v", view)
	}
	project, err := projects.Get(context.Background(), "task-project-1")
	if err != nil {
		t.Fatal(err)
	}
	if project.TopicID != 9 || project.OwnerUserID != 7 || project.WorkspaceID != 5 {
		t.Fatalf("project = %+v", project)
	}
	if len(repo.items) != 1 || repo.items[0].ProjectID == nil || *repo.items[0].ProjectID != project.InternalID {
		t.Fatalf("task = %+v", repo.items)
	}
	if metas.set["task-project-1"].OwnerUserID != 7 {
		t.Fatalf("meta = %+v", metas.set)
	}
}

func TestUpdateScheduleRecomputesNextRun(t *testing.T) {
	service, _, _, _, _ := newServiceFixture(t)
	view, err := service.Create(context.Background(), CreateInput{
		Name: "任务", Description: "描述", ScheduleType: ScheduleDaily,
		Time: "09:00", Timezone: "Asia/Shanghai", Source: SourceManual,
	}, 7, 5)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.Update(context.Background(), view.ID, 7, UpdateInput{
		Name: "新名字", Description: "新描述", ScheduleType: ScheduleWeekly,
		Time: "18:00", Weekdays: []int{1}, Timezone: "Asia/Shanghai",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "新名字" || updated.CronExpression != "0 18 * * 1" {
		t.Fatalf("updated = %+v", updated)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run 'TestCreateScheduledTask|TestUpdateSchedule' -v
```

Expected: FAIL，`Service`/`NewService` 不存在。

- [ ] **Step 3: 实现 Repository 接口与 Service**

在 `service.go` 追加：

```go
type Repository interface {
	Create(ctx context.Context, item *daomodel.ScheduledTask) error
	Get(ctx context.Context, id int64) (*daomodel.ScheduledTask, error)
	ListByUser(ctx context.Context, userID int64, workspaceIDs []int64) ([]daomodel.ScheduledTask, error)
	List(ctx context.Context, filter ListFilter) ([]daomodel.ScheduledTask, int64, error)
	Update(ctx context.Context, id int64, fields map[string]any) error
	Delete(ctx context.Context, id int64) error
	Due(ctx context.Context, now time.Time, limit int) ([]daomodel.ScheduledTask, error)
	Claim(ctx context.Context, id int64, oldNext, next, now time.Time) (bool, error)
	TopicByCode(ctx context.Context, code string) (*daomodel.ProjectTopic, error)
	UserNames(ctx context.Context, ids []int64) (map[int64]string, error)
}

type ProjectInitializer interface {
	InitProject(ctx context.Context, id string) (int64, error)
}

type MetaStore interface {
	Set(ctx context.Context, id string, meta auth.ProjectMeta) error
	Delete(ctx context.Context, id string) error
}

type StopChatClient interface {
	StopChat(ctx context.Context, id string) error
}

type Flags interface {
	Acquire(ctx context.Context, taskID int64, ttl time.Duration) (bool, error)
	Release(ctx context.Context, taskID int64)
	MarkStop(ctx context.Context, taskID int64, ttl time.Duration) error
	Stopped(ctx context.Context, taskID int64) bool
	ClearStop(ctx context.Context, taskID int64)
}

type Service struct {
	Repo        Repository
	Projects    store.ProjectStore
	Initializer ProjectInitializer
	Metas       MetaStore
	Sandbox     StopChatClient
	Flags       Flags
	Now         func() time.Time
	NewID       func() string
}

func NewService(repo Repository, projects store.ProjectStore, initializer ProjectInitializer, metas MetaStore, sandbox StopChatClient, flags Flags) *Service {
	return &Service{
		Repo: repo, Projects: projects, Initializer: initializer, Metas: metas,
		Sandbox: sandbox, Flags: flags, Now: time.Now, NewID: func() string { return uuid.NewString() },
	}
}

type extraFields struct {
	Source          string `json:"source"`
	ScheduleType    string `json:"scheduleType"`
	Time            string `json:"time"`
	Weekdays        []int  `json:"weekdays,omitempty"`
	ProjectPublicID string `json:"projectPublicId"`
}

func (s *Service) Create(ctx context.Context, input CreateInput, userID, workspaceID int64) (Task, error) {
	if userID <= 0 || workspaceID <= 0 {
		return Task{}, ErrInvalid
	}
	description := strings.TrimSpace(input.Description)
	if description == "" || len([]rune(description)) > 8000 {
		return Task{}, ErrInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		runes := []rune(description)
		if len(runes) > 24 { runes = runes[:24] }
		name = string(runes)
	}
	if len([]rune(name)) > 128 {
		return Task{}, ErrInvalid
	}
	source := strings.TrimSpace(input.Source)
	if source != SourceAI && source != SourceManual {
		source = SourceManual
	}
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: input.ScheduleType, Time: input.Time, Weekdays: input.Weekdays,
		CronExpression: input.CronExpression, Timezone: input.Timezone,
	})
	if err != nil {
		return Task{}, err
	}
	topic, err := s.Repo.TopicByCode(ctx, TopicCodeScheduled)
	if err != nil || topic.Status != 1 {
		return Task{}, ErrInvalid
	}
	now := s.Now().UTC()
	next, err := NextRun(schedule, now)
	if err != nil {
		return Task{}, err
	}
	publicID := s.NewID()
	sandboxID, err := s.Initializer.InitProject(ctx, publicID)
	if err != nil {
		return Task{}, fmt.Errorf("init scheduled project: %w", err)
	}
	project := &appmodel.Project{
		ID: publicID, Name: name, Prompt: description,
		State: appmodel.ProjectStateReady, ReviewStatus: appmodel.ReviewStatusPending,
		WorkspaceID: workspaceID, TopicID: topic.ID, OwnerUserID: userID,
		SandboxID: &sandboxID, AgentRoleIDs: []int64{}, CreatedAt: now, UpdatedAt: now,
	}
	cleanup := func() {
		_ = s.Projects.Delete(ctx, publicID)
		if s.Metas != nil { _ = s.Metas.Delete(ctx, publicID) }
	}
	if err := s.Projects.Create(ctx, project); err != nil {
		cleanup()
		return Task{}, fmt.Errorf("create scheduled project: %w", err)
	}
	createdProject, err := s.Projects.Get(ctx, publicID)
	if err != nil {
		cleanup()
		return Task{}, err
	}
	if s.Metas != nil {
		if err := s.Metas.Set(ctx, publicID, auth.ProjectMeta{WorkspaceID: workspaceID, OwnerUserID: userID}); err != nil {
			cleanup()
			return Task{}, err
		}
	}
	extra, _ := json.Marshal(extraFields{
		Source: source, ScheduleType: schedule.Type, Time: schedule.Time,
		Weekdays: schedule.Weekdays, ProjectPublicID: publicID,
	})
	workspace := workspaceID
	item := &daomodel.ScheduledTask{
		UserID: userID, WorkspaceID: &workspace, ProjectID: &createdProject.InternalID,
		Name: name, TaskType: TaskTypeAgent, CronExpr: schedule.CronExpr,
		Timezone: schedule.Timezone, Prompt: description, Enabled: 1,
		NextRunAt: &next, LastStatus: StatusPending, ExtraP: extra,
	}
	if err := s.Repo.Create(ctx, item); err != nil {
		cleanup()
		return Task{}, err
	}
	return s.view(item), nil
}
```

同文件追加 `List/Get/Update/Pause/Resume/Delete` 与辅助函数。核心逻辑：

```go
func (s *Service) List(ctx context.Context, userID int64, workspaceIDs []int64) ([]Task, error) {
	items, err := s.Repo.ListByUser(ctx, userID, workspaceIDs)
	if err != nil { return nil, err }
	out := make([]Task, 0, len(items))
	for index := range items { out = append(out, s.view(&items[index])) }
	return out, nil
}

func (s *Service) Get(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return Task{}, err }
	if item.UserID != userID { return Task{}, ErrForbidden }
	return s.view(item), nil
}

func (s *Service) Update(ctx context.Context, id, userID int64, input UpdateInput) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return Task{}, err }
	if item.UserID != userID { return Task{}, ErrForbidden }
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	if name == "" || len([]rune(name)) > 128 || description == "" || len([]rune(description)) > 8000 {
		return Task{}, ErrInvalid
	}
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: input.ScheduleType, Time: input.Time, Weekdays: input.Weekdays,
		CronExpression: input.CronExpression, Timezone: input.Timezone,
	})
	if err != nil { return Task{}, err }
	next, err := NextRun(schedule, s.Now().UTC())
	if err != nil { return Task{}, err }
	extra, _ := json.Marshal(extraFields{
		Source: sourceOf(item), ScheduleType: schedule.Type, Time: schedule.Time,
		Weekdays: schedule.Weekdays, ProjectPublicID: projectIDOf(item),
	})
	fields := map[string]any{
		"name": name, "prompt": description, "cron_expr": schedule.CronExpr,
		"timezone": schedule.Timezone, "next_run_at": next, "extra_p": extra,
	}
	if item.Enabled == 1 { fields["last_status"] = StatusPending }
	if err := s.Repo.Update(ctx, id, fields); err != nil { return Task{}, err }
	item.Name, item.Prompt, item.CronExpr, item.Timezone = name, description, schedule.CronExpr, schedule.Timezone
	item.NextRunAt, item.ExtraP = &next, extra
	return s.view(item), nil
}

func (s *Service) Pause(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return Task{}, err }
	if userID > 0 && item.UserID != userID { return Task{}, ErrForbidden }
	if err := s.Repo.Update(ctx, id, map[string]any{"enabled": 0, "next_run_at": nil}); err != nil { return Task{}, err }
	item.Enabled = 0
	item.NextRunAt = nil
	return s.view(item), nil
}

func (s *Service) Resume(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return Task{}, err }
	if userID > 0 && item.UserID != userID { return Task{}, ErrForbidden }
	schedule, err := NormalizeSchedule(ScheduleInput{Type: inferScheduleType(item), Time: timeOf(item), Weekdays: weekdaysOf(item), CronExpression: item.CronExpr, Timezone: item.Timezone})
	if err != nil { return Task{}, err }
	next, err := NextRun(schedule, s.Now().UTC())
	if err != nil { return Task{}, err }
	if err := s.Repo.Update(ctx, id, map[string]any{"enabled": 1, "next_run_at": next}); err != nil { return Task{}, err }
	item.Enabled = 1
	item.NextRunAt = &next
	return s.view(item), nil
}
```

`Delete` 逻辑：

```go
func (s *Service) Stop(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return Task{}, err }
	if userID > 0 && item.UserID != userID { return Task{}, ErrForbidden }
	if item.LastStatus != StatusRunning {
		return s.view(item), nil
	}
	if s.Flags != nil {
		if err := s.Flags.MarkStop(ctx, id, 10*time.Minute); err != nil { return Task{}, err }
	}
	if s.Sandbox != nil {
		if err := s.Sandbox.StopChat(ctx, projectIDOf(item)); err != nil { return Task{}, err }
	}
	if err := s.Repo.Update(ctx, id, map[string]any{"last_status": StatusStopped}); err != nil { return Task{}, err }
	item.LastStatus = StatusStopped
	return s.view(item), nil
}

func (s *Service) AdminList(ctx context.Context, filter ListFilter) (Page, error) {
	if filter.Page < 1 { filter.Page = 1 }
	if filter.PageSize <= 0 { filter.PageSize = 20 }
	items, total, err := s.Repo.List(ctx, filter)
	if err != nil { return Page{}, err }
	ids := make([]int64, 0, len(items))
	for index := range items { ids = append(ids, items[index].UserID) }
	names, _ := s.Repo.UserNames(ctx, ids)
	out := make([]Task, 0, len(items))
	for index := range items {
		view := s.view(&items[index])
		view.Username = names[items[index].UserID]
		out = append(out, view)
	}
	return Page{Items: out, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *Service) Delete(ctx context.Context, id, userID int64, deleteProject bool) error {
	item, err := s.Repo.Get(ctx, id)
	if err != nil { return err }
	if item.UserID != userID { return ErrForbidden }
	if item.LastStatus == StatusRunning { return ErrInvalid }
	if deleteProject {
		projectID := projectIDOf(item)
		if projectID == "" { return ErrInvalid }
		if err := s.Projects.Delete(ctx, projectID); err != nil { return err }
		if s.Metas != nil { _ = s.Metas.Delete(ctx, projectID) }
	}
	return s.Repo.Delete(ctx, id)
}
```

辅助函数：

```go
func (s *Service) view(item *daomodel.ScheduledTask) Task {
	extra := extraOf(item)
	enabled := item.Enabled == 1
	view := Task{
		ID: item.ID, Name: item.Name, Description: item.Prompt,
		ScheduleType: extra.ScheduleType, Time: extra.Time, Weekdays: extra.Weekdays,
		CronExpression: item.CronExpr, Timezone: item.Timezone,
		Source: extra.Source, Enabled: enabled, ProjectID: extra.ProjectPublicID,
		LastStatus: item.LastStatus, LastError: item.LastError, RunCount: item.RunCount,
	}
	if item.WorkspaceID != nil { view.WorkspaceID = *item.WorkspaceID }
	if item.NextRunAt != nil {
		value := item.NextRunAt.UTC().Format(time.RFC3339)
		view.NextRunAt = &value
	}
	if item.LastRunAt != nil {
		value := item.LastRunAt.UTC().Format(time.RFC3339)
		view.LastRunAt = &value
	}
	return view
}

func extraOf(item *daomodel.ScheduledTask) extraFields {
	var extra extraFields
	if len(item.ExtraP) > 0 {
		_ = json.Unmarshal(item.ExtraP, &extra)
	}
	if extra.Source == "" { extra.Source = SourceManual }
	if extra.ScheduleType == "" { extra.ScheduleType = inferScheduleType(item) }
	return extra
}

func projectIDOf(item *daomodel.ScheduledTask) string { return extraOf(item).ProjectPublicID }
func sourceOf(item *daomodel.ScheduledTask) string { return extraOf(item).Source }
func timeOf(item *daomodel.ScheduledTask) string { return extraOf(item).Time }
func weekdaysOf(item *daomodel.ScheduledTask) []int { return extraOf(item).Weekdays }

func inferScheduleType(item *daomodel.ScheduledTask) string {
	fields := strings.Fields(item.CronExpr)
	if len(fields) != 5 { return ScheduleCron }
	if fields[2] == "*" && fields[3] == "*" && fields[4] == "*" { return ScheduleDaily }
	if fields[2] == "*" && fields[3] == "*" { return ScheduleWeekly }
	return ScheduleCron
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 4: MySQL Repository

**Files:**
- Create: `packages/backend/internal/scheduledtask/mysql.go`
- Test: `packages/backend/internal/scheduledtask/mysql_test.go`

**Interfaces:**
- Consumes: `*gorm.DB`、`daomodel.ScheduledTask`。
- Produces: `MySQLRepository` 实现 Task 3 的 `Repository`。

- [ ] **Step 1: 写失败测试**

使用现有 `sqlmock` 模式：

```go
func TestMySQLDueAndClaim(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT .* FROM `scheduled_task` .*enabled = \\? AND deleted_at IS NULL AND next_run_at <= \\?").
		WithArgs(1, now, 100).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "workspace_id", "project_id", "name", "task_type", "cron_expr",
			"timezone", "prompt", "enabled", "next_run_at", "last_run_at", "last_status",
			"last_error", "run_count", "max_runs", "created_at", "updated_at", "deleted_at", "extra_p",
		}).AddRow(3, 7, 5, 11, "任务", "agent", "0 9 * * *", "Asia/Shanghai", "检查", 1,
			now.Add(-time.Minute), nil, "pending", "", 2, 0, now, now, nil, []byte(`{}`)))
	items, err := repo.Due(context.Background(), now, 100)
	if err != nil || len(items) != 1 || items[0].ID != 3 {
		t.Fatalf("items = %+v err = %v", items, err)
	}
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE `scheduled_task` SET .*next_run_at.*run_count.*").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	ok, err := repo.Claim(context.Background(), 3, now.Add(-time.Minute), now.Add(time.Hour), now)
	if err != nil || !ok {
		t.Fatalf("ok = %v err = %v", ok, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil { t.Fatal(err) }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestMySQLDueAndClaim -v
```

Expected: FAIL，`NewMySQLRepository` 不存在。

- [ ] **Step 3: 实现 Repository**

```go
type MySQLRepository struct{ db *gorm.DB }

func NewMySQLRepository(db *gorm.DB) *MySQLRepository { return &MySQLRepository{db: db} }

func (r *MySQLRepository) Create(ctx context.Context, item *daomodel.ScheduledTask) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create scheduled task: %w", err)
	}
	return nil
}

func (r *MySQLRepository) Get(ctx context.Context, id int64) (*daomodel.ScheduledTask, error) {
	var item daomodel.ScheduledTask
	if err := r.db.WithContext(ctx).Where("id = ?", id).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrNotFound }
		return nil, fmt.Errorf("get scheduled task: %w", err)
	}
	return &item, nil
}

func (r *MySQLRepository) ListByUser(ctx context.Context, userID int64, workspaceIDs []int64) ([]daomodel.ScheduledTask, error) {
	var items []daomodel.ScheduledTask
	query := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if len(workspaceIDs) > 0 { query = query.Where("workspace_id IN ?", workspaceIDs) }
	if err := query.Order("created_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list user scheduled tasks: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) List(ctx context.Context, filter ListFilter) ([]daomodel.ScheduledTask, int64, error) {
	if filter.Page < 1 { filter.Page = 1 }
	if filter.PageSize <= 0 { filter.PageSize = 20 }
	if filter.PageSize > 100 { filter.PageSize = 100 }
	query := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{})
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR prompt LIKE ?", like, like)
	}
	if filter.Enabled != nil {
		value := 0
		if *filter.Enabled { value = 1 }
		query = query.Where("enabled = ?", value)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil { return nil, 0, err }
	var items []daomodel.ScheduledTask
	if err := query.Order("created_at DESC, id DESC").Offset((filter.Page-1)*filter.PageSize).Limit(filter.PageSize).Find(&items).Error; err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *MySQLRepository) Update(ctx context.Context, id int64, fields map[string]any) error {
	fields["updated_at"] = time.Now().UTC()
	result := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{}).Where("id = ?", id).Updates(fields)
	if result.Error != nil { return fmt.Errorf("update scheduled task: %w", result.Error) }
	if result.RowsAffected == 0 { return ErrNotFound }
	return nil
}

func (r *MySQLRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&daomodel.ScheduledTask{}, id)
	if result.Error != nil { return fmt.Errorf("delete scheduled task: %w", result.Error) }
	if result.RowsAffected == 0 { return ErrNotFound }
	return nil
}

func (r *MySQLRepository) Due(ctx context.Context, now time.Time, limit int) ([]daomodel.ScheduledTask, error) {
	var items []daomodel.ScheduledTask
	if err := r.db.WithContext(ctx).
		Where("enabled = ? AND deleted_at IS NULL AND next_run_at <= ?", 1, now).
		Order("next_run_at ASC, id ASC").Limit(limit).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list due scheduled tasks: %w", err)
	}
	return items, nil
}

func (r *MySQLRepository) Claim(ctx context.Context, id int64, oldNext, next, now time.Time) (bool, error) {
	result := r.db.WithContext(ctx).Model(&daomodel.ScheduledTask{}).
		Where("id = ? AND enabled = ? AND deleted_at IS NULL AND next_run_at = ?", id, 1, oldNext).
		Updates(map[string]any{
			"next_run_at": next, "last_run_at": now,
			"run_count": gorm.Expr("run_count + 1"), "updated_at": now,
		})
	if result.Error != nil { return false, fmt.Errorf("claim scheduled task: %w", result.Error) }
	return result.RowsAffected == 1, nil
}

func (r *MySQLRepository) UserNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 { return out, nil }
	var rows []struct {
		ID       int64  `gorm:"column:id"`
		Username string `gorm:"column:username"`
	}
	if err := r.db.WithContext(ctx).Table("user").Select("id, username").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load scheduled task usernames: %w", err)
	}
	for _, row := range rows { out[row.ID] = row.Username }
	return out, nil
}

func (r *MySQLRepository) TopicByCode(ctx context.Context, code string) (*daomodel.ProjectTopic, error) {
	var topic daomodel.ProjectTopic
	if err := r.db.WithContext(ctx).Where("topic_code = ?", code).Take(&topic).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrInvalid }
		return nil, fmt.Errorf("get scheduled topic: %w", err)
	}
	return &topic, nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 5: Redis 执行锁与停止标记

**Files:**
- Create: `packages/backend/internal/scheduledtask/flags.go`
- Test: `packages/backend/internal/scheduledtask/flags_test.go`

**Interfaces:**
- Consumes: `*redis.Client`、miniredis。
- Produces:
  - `NewRedisFlags(client *redis.Client, prefix string) *RedisFlags`，实现 Task 3 的 `Flags`。

- [ ] **Step 1: 写失败测试**

```go
func TestRedisFlags(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	flags := NewRedisFlags(client, "vibebiz")
	ctx := context.Background()

	ok, err := flags.Acquire(ctx, 9, time.Minute)
	if err != nil || !ok { t.Fatalf("acquire ok=%v err=%v", ok, err) }
	if second, _ := flags.Acquire(ctx, 9, time.Minute); second {
		t.Fatal("second acquire should fail")
	}
	flags.Release(ctx, 9)
	if ok, _ := flags.Acquire(ctx, 9, time.Minute); !ok {
		t.Fatal("acquire after release should succeed")
	}

	if err := flags.MarkStop(ctx, 9, time.Minute); err != nil { t.Fatal(err) }
	if !flags.Stopped(ctx, 9) { t.Fatal("stop flag should be set") }
	flags.ClearStop(ctx, 9)
	if flags.Stopped(ctx, 9) { t.Fatal("stop flag should be cleared") }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestRedisFlags -v
```

Expected: FAIL，`NewRedisFlags` 不存在。

- [ ] **Step 3: 实现 Flags**

```go
type RedisFlags struct {
	client *redis.Client
	prefix string
}

func NewRedisFlags(client *redis.Client, prefix string) *RedisFlags {
	return &RedisFlags{client: client, prefix: strings.TrimSuffix(prefix, ":")}
}

func (f *RedisFlags) lockKey(taskID int64) string { return fmt.Sprintf("%s:scheduled:run:%d", f.prefix, taskID) }
func (f *RedisFlags) stopKey(taskID int64) string { return fmt.Sprintf("%s:scheduled:stop:%d", f.prefix, taskID) }

func (f *RedisFlags) Acquire(ctx context.Context, taskID int64, ttl time.Duration) (bool, error) {
	return f.client.SetNX(ctx, f.lockKey(taskID), "1", ttl).Result()
}
func (f *RedisFlags) Release(ctx context.Context, taskID int64) {
	_ = f.client.Del(ctx, f.lockKey(taskID)).Err()
}
func (f *RedisFlags) MarkStop(ctx context.Context, taskID int64, ttl time.Duration) error {
	return f.client.Set(ctx, f.stopKey(taskID), "1", ttl).Err()
}
func (f *RedisFlags) Stopped(ctx context.Context, taskID int64) bool {
	value, err := f.client.Exists(ctx, f.stopKey(taskID)).Result()
	return err == nil && value == 1
}
func (f *RedisFlags) ClearStop(ctx context.Context, taskID int64) {
	_ = f.client.Del(ctx, f.stopKey(taskID)).Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestRedisFlags -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 6: Dispatcher

**Files:**
- Create: `packages/backend/internal/scheduledtask/dispatcher.go`
- Test: `packages/backend/internal/scheduledtask/dispatcher_test.go`

**Interfaces:**
- Consumes: `Repository`、`asynq.Client`。
- Produces:
  - 常量 `TaskTypeDispatch = "scheduled_task:dispatch"`
  - 常量 `TaskTypeRun = "scheduled_task:run"`
  - `Enqueuer` interface：`EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)`
  - `Dispatcher` 与 `NewDispatcher(repo, client, logger)`

- [ ] **Step 1: 写失败测试**

```go
type fakeDueRepo struct {
	*fakeRepo
	due []daomodel.ScheduledTask
	claimed map[int64]bool
}

func (f *fakeDueRepo) Due(context.Context, time.Time, int) ([]daomodel.ScheduledTask, error) { return f.due, nil }
func (f *fakeDueRepo) Claim(_ context.Context, id int64, _, _ , _ time.Time) (bool, error) {
	f.claimed[id] = true
	return true, nil
}

type fakeEnqueuer struct{ tasks []*asynq.Task }

func (f *fakeEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "x"}, nil
}

func TestDispatcherClaimsAndEnqueues(t *testing.T) {
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	item := daomodel.ScheduledTask{
		Base: daomodel.Base{ID: 3}, CronExpr: "0 9 * * *", Timezone: "Asia/Shanghai",
		Enabled: 1, NextRunAt: ptrTime(now.Add(-time.Minute)),
		ExtraP: []byte(`{"scheduleType":"daily","time":"09:00","projectPublicId":"p1"}`),
	}
	repo := &fakeDueRepo{fakeRepo: &fakeRepo{}, due: []daomodel.ScheduledTask{item}, claimed: map[int64]bool{}}
	enqueuer := &fakeEnqueuer{}
	dispatcher := NewDispatcher(repo, enqueuer, slog.Default())
	dispatcher.Now = func() time.Time { return now }
	if err := dispatcher.HandleTask(context.Background(), asynq.NewTask(TaskTypeDispatch, nil)); err != nil {
		t.Fatal(err)
	}
	if !repo.claimed[3] || len(enqueuer.tasks) != 1 {
		t.Fatalf("claimed=%v tasks=%d", repo.claimed, len(enqueuer.tasks))
	}
	if enqueuer.tasks[0].Type() != TaskTypeRun {
		t.Fatalf("type = %s", enqueuer.tasks[0].Type())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestDispatcher -v
```

Expected: FAIL，`NewDispatcher` 不存在。

- [ ] **Step 3: 实现 Dispatcher**

```go
const (
	TaskTypeDispatch = "scheduled_task:dispatch"
	TaskTypeRun      = "scheduled_task:run"
)

type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type Dispatcher struct {
	Repo     Repository
	Client   Enqueuer
	Logger   *slog.Logger
	Now      func() time.Time
}

func NewDispatcher(repo Repository, client Enqueuer, logger *slog.Logger) *Dispatcher {
	if logger == nil { logger = slog.Default() }
	return &Dispatcher{Repo: repo, Client: client, Logger: logger, Now: time.Now}
}

func (d *Dispatcher) HandleTask(ctx context.Context, _ *asynq.Task) error {
	now := d.Now().UTC()
	items, err := d.Repo.Due(ctx, now, 100)
	if err != nil { return err }
	for index := range items {
		item := items[index]
		oldNext := *item.NextRunAt
		schedule, err := NormalizeSchedule(ScheduleInput{
			Type: inferScheduleType(&item), Time: timeOf(&item), Weekdays: weekdaysOf(&item),
			CronExpression: item.CronExpr, Timezone: item.Timezone,
		})
		if err != nil {
			_ = d.Repo.Update(ctx, item.ID, map[string]any{"enabled": 0, "last_error": "invalid schedule: " + err.Error()})
			continue
		}
		next, err := NextRun(schedule, now)
		if err != nil {
			_ = d.Repo.Update(ctx, item.ID, map[string]any{"enabled": 0, "last_error": "invalid next run: " + err.Error()})
			continue
		}
		claimed, err := d.Repo.Claim(ctx, item.ID, oldNext, next, now)
		if err != nil || !claimed { continue }
		payload, _ := json.Marshal(map[string]int64{"taskId": item.ID})
		task := asynq.NewTask(TaskTypeRun, payload)
		if _, err := d.Client.EnqueueContext(ctx, task,
			asynq.Unique(50*time.Second),
			asynq.MaxRetry(2),
			asynq.Timeout(15*time.Minute),
		); err != nil {
			d.Logger.Error("scheduled task enqueue failed", "task_id", item.ID, "error", err)
			_ = d.Repo.Update(ctx, item.ID, map[string]any{"last_status": StatusFailed, "last_error": err.Error()})
		}
	}
	return nil
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestDispatcher -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 7: Runner

**Files:**
- Create: `packages/backend/internal/scheduledtask/runner.go`
- Test: `packages/backend/internal/scheduledtask/runner_test.go`

**Interfaces:**
- Consumes: `Repository`、`Flags`、sandbox `ReadChatHistory`/`Chat`、`notification.Service`。
- Produces: `Runner` 与 `NewRunner`。

- [ ] **Step 1: 写失败测试**

```go
type fakeHistory struct {
	tail client.ChatHistoryTail
	err  error
}

func (f *fakeHistory) ReadChatHistory(context.Context, string, int) (client.ChatHistoryTail, error) {
	return f.tail, f.err
}

type fakeChat struct {
	payload []byte
	body    string
	err     error
}

func (f *fakeChat) Chat(_ context.Context, _ string, payload []byte) (io.ReadCloser, error) {
	f.payload = payload
	if f.err != nil { return nil, f.err }
	return io.NopCloser(strings.NewReader(f.body)), nil
}

type fakeFlags struct{ stopped bool; acquired bool }
func (f *fakeFlags) Acquire(context.Context, int64, time.Duration) (bool, error) { return f.acquired, nil }
func (f *fakeFlags) Release(context.Context, int64) {}
func (f *fakeFlags) MarkStop(context.Context, int64, time.Duration) error { return nil }
func (f *fakeFlags) Stopped(context.Context, int64) bool { return f.stopped }
func (f *fakeFlags) ClearStop(context.Context, int64) {}

type fakeNotifier struct{ created int; published int }
func (f *fakeNotifier) Create(context.Context, notification.CreateInput, notification.Creator) (notification.View, error) {
	f.created++
	return notification.View{ID: 11}, nil
}
func (f *fakeNotifier) Publish(context.Context, int64) (notification.View, error) {
	f.published++
	return notification.View{ID: 11}, nil
}

func TestRunnerBuildsPromptAndNotifies(t *testing.T) {
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	repo := &fakeRepo{
		items: []daomodel.ScheduledTask{{
			Base: daomodel.Base{ID: 3}, UserID: 7, ProjectID: ptrInt64(11),
			Name: "检查", Prompt: "检查依赖", Enabled: 1, CronExpr: "0 9 * * *",
			Timezone: "Asia/Shanghai", LastStatus: StatusRunning,
			ExtraP: []byte(`{"projectPublicId":"p1"}`),
		}},
		topics: []daomodel.ProjectTopic{{
			Base: daomodel.Base{ID: 9}, TopicCode: TopicCodeScheduled,
			PromptTemplate: "系统提示",
		}},
	}
	history := &fakeHistory{tail: client.ChatHistoryTail{Content: `{"type":"session"}` + "\n" + `{"type":"message","message":{"role":"user","content":"你好"}}` + "\n" + `{"type":"message","message":{"role":"assistant","content":"收到"}}` + "\n"}}
	chat := &fakeChat{body: "data: {\"type\":\"agent\",\"payload\":{\"type\":\"result\"}}\n\ndata: [DONE]\n\n"}
	flags := &fakeFlags{acquired: true}
	notifier := &fakeNotifier{}
	runner := NewRunner(repo, chat, history, flags, notifier, "http://vibebiz-dev.com:5173", slog.Default())
	runner.Now = func() time.Time { return now }
	out, err := runner.Execute(context.Background(), 3)
	if err != nil { t.Fatal(err) }
	if out.Status != StatusSuccess { t.Fatalf("status = %s", out.Status) }
	var payload map[string]string
	if err := json.Unmarshal(chat.payload, &payload); err != nil { t.Fatal(err) }
	if payload["message"] != "检查依赖" || !strings.Contains(payload["context"], "用户：你好") {
		t.Fatalf("payload = %+v", payload)
	}
	if notifier.created != 1 || notifier.published != 1 {
		t.Fatalf("notifications = %+v", notifier)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestRunner -v
```

Expected: FAIL，`NewRunner` 不存在。

- [ ] **Step 3: 实现 Runner**

```go
type HistoryClient interface {
	ReadChatHistory(ctx context.Context, id string, limit int) (client.ChatHistoryTail, error)
}
type ChatClient interface {
	Chat(ctx context.Context, id string, payload []byte) (io.ReadCloser, error)
}
type Notifier interface {
	Create(ctx context.Context, input notification.CreateInput, creator notification.Creator) (notification.View, error)
	Publish(ctx context.Context, id int64) (notification.View, error)
}

type Runner struct {
	Repo     Repository
	Chat     ChatClient
	History  HistoryClient
	Flags    Flags
	Notifier Notifier
	WebBase  string
	Logger   *slog.Logger
	Now      func() time.Time
}

func NewRunner(repo Repository, chat ChatClient, history HistoryClient, flags Flags, notifier Notifier, webBase string, logger *slog.Logger) *Runner {
	if logger == nil { logger = slog.Default() }
	return &Runner{Repo: repo, Chat: chat, History: history, Flags: flags, Notifier: notifier, WebBase: strings.TrimRight(webBase, "/"), Logger: logger, Now: time.Now}
}

type ExecutionResult struct {
	Status string
	Error  string
}

func (r *Runner) HandleTask(ctx context.Context, task *asynq.Task) error {
	var payload struct{ TaskID int64 `json:"taskId"` }
	if err := json.Unmarshal(task.Payload(), &payload); err != nil { return err }
	_, err := r.Execute(ctx, payload.TaskID)
	return err
}

func (r *Runner) Execute(ctx context.Context, taskID int64) (ExecutionResult, error) {
	item, err := r.Repo.Get(ctx, taskID)
	if err != nil { return ExecutionResult{}, err }
	if item.Enabled != 1 { return ExecutionResult{Status: StatusStopped}, nil }
	acquired, err := r.Flags.Acquire(ctx, taskID, 15*time.Minute)
	if err != nil { return ExecutionResult{}, err }
	if !acquired { return ExecutionResult{Status: StatusRunning}, nil }
	defer r.Flags.Release(ctx, taskID)

	_ = r.Repo.Update(ctx, taskID, map[string]any{"last_status": StatusRunning})
	topic, err := r.Repo.TopicByCode(ctx, TopicCodeScheduled)
	if err != nil { return r.fail(ctx, item, err) }
	projectID := projectIDOf(item)
	history, err := r.History.ReadChatHistory(ctx, projectID, 5)
	if err != nil { return r.fail(ctx, item, err) }
	payload, _ := json.Marshal(map[string]string{
		"message": item.Prompt,
		"systemPrompt": topic.PromptTemplate,
		"context": formatHistory(history.Content),
	})
	stream, err := r.Chat.Chat(ctx, projectID, payload)
	if err != nil { return r.fail(ctx, item, err) }
	defer stream.Close()
	if err := consumeSSE(stream); err != nil { return r.fail(ctx, item, err) }

	status := StatusSuccess
	if r.Flags.Stopped(ctx, taskID) {
		status = StatusStopped
		r.Flags.ClearStop(ctx, taskID)
	}
	_ = r.Repo.Update(ctx, taskID, map[string]any{"last_status": status, "last_error": ""})
	r.notify(ctx, item, status, "")
	return ExecutionResult{Status: status}, nil
}

func (r *Runner) fail(ctx context.Context, item *daomodel.ScheduledTask, cause error) (ExecutionResult, error) {
	status := StatusFailed
	if r.Flags.Stopped(ctx, item.ID) {
		status = StatusStopped
		r.Flags.ClearStop(ctx, item.ID)
	}
	fields := map[string]any{"last_status": status, "last_error": cause.Error()}
	var apiErr *client.APIError
	if errors.As(cause, &apiErr) && apiErr.Status == http.StatusNotFound {
		fields["enabled"] = 0
		fields["next_run_at"] = nil
	}
	_ = r.Repo.Update(ctx, item.ID, fields)
	r.notify(ctx, item, status, cause.Error())
	return ExecutionResult{Status: status, Error: cause.Error()}, cause
}

func (r *Runner) notify(ctx context.Context, item *daomodel.ScheduledTask, status, detail string) {
	if r.Notifier == nil { return }
	title := "定时任务执行成功"
	if status == StatusFailed { title = "定时任务执行失败" }
	if status == StatusStopped { title = "定时任务已停止" }
	content := "任务「" + item.Name + "」" + status
	if detail != "" { content += "：" + detail }
	link := ""
	if projectID := projectIDOf(item); projectID != "" && r.WebBase != "" {
		link = r.WebBase + "/editor/" + url.PathEscape(projectID)
	}
	view, err := r.Notifier.Create(ctx, notification.CreateInput{
		Title: title, Content: content, Type: model.NotificationInbox,
		Link: link, RecipientUserID: item.UserID, ExpireAt: r.Now().Add(7 * 24 * time.Hour),
	}, notification.Creator{ID: 0, Name: "system"})
	if err != nil { return }
	_, _ = r.Notifier.Publish(ctx, view.ID)
}
```

```go
func formatHistory(content string) string {
	lines := strings.Split(content, "\n")
	parts := make([]string, 0, 5)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" { continue }
		var entry struct {
			Type    string `json:"type"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
		}
		if json.Unmarshal([]byte(line), &entry) != nil || entry.Type != "message" { continue }
		role := "用户"
		if entry.Message.Role == "assistant" { role = "助手" }
		text := strings.TrimSpace(entry.Message.Content)
		if len([]rune(text)) > 1500 {
			runes := []rune(text)
			text = string(runes[:1500]) + "..."
		}
		parts = append(parts, role+"："+text)
	}
	if len(parts) > 5 { parts = parts[len(parts)-5:] }
	out := strings.Join(parts, "\n")
	if len([]rune(out)) > 6000 {
		runes := []rune(out)
		out = string(runes[:6000]) + "..."
	}
	return out
}

func consumeSSE(stream io.Reader) error {
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadString('\n')
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(trimmed, "data:"))
			if payload == "[DONE]" { return nil }
			var event struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}
			if json.Unmarshal([]byte(payload), &event) == nil && event.Type == "error" {
				if event.Message == "" { event.Message = "agent failed" }
				return errors.New(event.Message)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) { return nil }
			return err
		}
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/scheduledtask -run TestRunner -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 8: Sandbox 任务解析接口

**Files:**
- Create: `packages/sandbox/app/services/task_parser.py`
- Create: `packages/sandbox/app/routers/tasks.py`
- Modify: `packages/sandbox/app/main.py`
- Test: `packages/sandbox/tests/test_tasks.py`

**Interfaces:**
- Consumes: `Settings.llm_api_key`、`Settings.llm_base_url`、`Settings.llm_model`、`httpx.AsyncClient`。
- Produces: `POST /internal/tasks/parse`。

- [ ] **Step 1: 写失败测试**

```python
from pathlib import Path

import httpx
from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path, monkeypatch) -> TestClient:
    settings = Settings(workspaces_dir=tmp_path, llm_api_key="key", llm_base_url="https://llm.test", llm_model="model")
    return TestClient(create_app(settings))


def test_parse_task_returns_draft(tmp_path: Path, monkeypatch):
    client = make_client(tmp_path, monkeypatch)

    def fake_post(self, url, headers=None, json=None):
        assert url == "https://llm.test/chat/completions"
        return httpx.Response(200, json={"choices": [{"message": {"content": '{"name":"检查依赖","description":"检查更新","scheduleType":"daily","time":"09:00","weekdays":[],"cronExpression":"0 9 * * *","timezone":"Asia/Shanghai"}'}}]})

    monkeypatch.setattr(httpx.AsyncClient, "post", fake_post)
    response = client.post("/internal/tasks/parse", json={"prompt": "每天 9 点检查依赖"})
    assert response.status_code == 200
    body = response.json()
    assert body["code"] == 0
    assert body["result"]["scheduleType"] == "daily"
    assert body["result"]["cronExpression"] == "0 9 * * *"


def test_parse_task_requires_llm_key(tmp_path: Path):
    settings = Settings(workspaces_dir=tmp_path, llm_api_key="")
    client = TestClient(create_app(settings))
    response = client.post("/internal/tasks/parse", json={"prompt": "每天 9 点检查依赖"})
    assert response.status_code == 503
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/sandbox && pytest tests/test_tasks.py -v
```

Expected: FAIL，404 或路由不存在。

- [ ] **Step 3: 实现解析服务和路由**

`app/services/task_parser.py`：

```python
import json
from typing import Any

import httpx

from app.config import Settings
from app.errors import WorkspaceError
from app.observability import get_logger

logger = get_logger()


class TaskParseUnavailable(WorkspaceError):
    status = 503
    code = 50301


class TaskParseFailed(WorkspaceError):
    status = 502
    code = 50201


SYSTEM_PROMPT = """你是定时任务解析器。请把用户需求解析成 JSON，只输出 JSON，不要输出解释。
字段：
name: 任务名称，最多 128 字
description: 实际执行时发给 Agent 的任务描述
scheduleType: daily、weekly 或 cron
time: 每日/每周时间，HH:MM
weekdays: 每周的星期数组，0=周日，1=周一，最多 7 个
cronExpression: 5 段 cron；daily/weekly 也要给出等价表达式
timezone: IANA 时区，默认 Asia/Shanghai
"""


async def parse_task(settings: Settings, prompt: str) -> dict[str, Any]:
    if not settings.llm_api_key:
        raise TaskParseUnavailable("LLM_API_KEY is not configured")
    body = {
        "model": settings.llm_model,
        "messages": [
            {"role": "system", "content": SYSTEM_PROMPT},
            {"role": "user", "content": prompt},
        ],
        "temperature": 0.1,
    }
    try:
        async with httpx.AsyncClient(timeout=30) as client:
            response = await client.post(
                settings.llm_base_url.rstrip("/") + "/chat/completions",
                headers={"Authorization": f"Bearer {settings.llm_api_key}"},
                json=body,
            )
        response.raise_for_status()
    except Exception as exc:
        logger.error("task.parse.llm_error", extra={"status": "error", "error_message": str(exc)})
        raise TaskParseFailed("task parse failed") from exc
    content = response.json()["choices"][0]["message"]["content"]
    if not isinstance(content, str):
        raise TaskParseFailed("empty parse result")
    raw = content.strip()
    if raw.startswith("```"):
        raw = raw.strip("`")
        if raw.startswith("json"):
            raw = raw[4:]
    start, end = raw.find("{"), raw.rfind("}")
    if start < 0 or end <= start:
        raise TaskParseFailed("invalid parse result")
    try:
        data = json.loads(raw[start:end + 1])
    except json.JSONDecodeError as exc:
        raise TaskParseFailed("invalid parse result") from exc
    return normalize_draft(data)


def normalize_draft(data: dict[str, Any]) -> dict[str, Any]:
    schedule_type = str(data.get("scheduleType") or "daily")
    if schedule_type not in {"daily", "weekly", "cron"}:
        raise TaskParseFailed("invalid schedule type")
    weekdays = data.get("weekdays") or []
    if not isinstance(weekdays, list):
        weekdays = []
    weekdays = [int(item) for item in weekdays if isinstance(item, int) and 0 <= item <= 6]
    return {
        "name": str(data.get("name") or "").strip()[:128],
        "description": str(data.get("description") or "").strip()[:8000],
        "scheduleType": schedule_type,
        "time": str(data.get("time") or "09:00"),
        "weekdays": weekdays,
        "cronExpression": str(data.get("cronExpression") or ""),
        "timezone": str(data.get("timezone") or "Asia/Shanghai"),
    }
```

`app/routers/tasks.py`：

```python
from fastapi import APIRouter, Request
from pydantic import BaseModel

from app.response import envelope
from app.services.task_parser import parse_task

router = APIRouter(prefix="/internal/tasks", tags=["tasks"])


class ParseRequest(BaseModel):
    prompt: str = ""


@router.post("/parse")
async def parse(body: ParseRequest, request: Request) -> dict:
    prompt = body.prompt.strip()
    if not prompt:
        from app.errors import WorkspaceError
        raise WorkspaceError("prompt is required")
    result = await parse_task(request.app.state.settings, prompt)
    return envelope(result)
```

`app/main.py`：

```python
from app.routers import chat, files, logs, preview, projects, resources, skills, tasks
...
app.include_router(tasks.router)
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/sandbox && pytest tests/test_tasks.py -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 9: Sandbox chat 支持 systemPrompt/context

**Files:**
- Modify: `packages/sandbox/app/routers/chat.py`
- Modify: `packages/sandbox/app/services/chat_turns.py`
- Modify: `packages/sandbox/app/services/agent.py`
- Modify: `packages/sandbox/app/services/claude_agent.py`
- Test: `packages/sandbox/tests/test_chat_context.py`

**Interfaces:**
- Consumes: 现有 `ChatRequest`、`ChatTurnManager.start`、`Agent.run`。
- Produces: `ChatRequest.systemPrompt`、`ChatRequest.context`；`Agent.run(..., system_prompt="", context="")`。

- [ ] **Step 1: 写失败测试**

```python
from pathlib import Path

from fastapi.testclient import TestClient

from app.config import Settings
from app.main import create_app


def make_client(tmp_path: Path) -> TestClient:
    return TestClient(create_app(Settings(workspaces_dir=tmp_path, agent_mode="stub")))


def test_context_and_system_prompt_are_not_persisted(tmp_path: Path):
    client = make_client(tmp_path)
    assert client.post("/internal/projects/p1/init").status_code == 200
    with client.stream("POST", "/internal/projects/p1/chat", json={
        "message": "真正任务",
        "systemPrompt": "系统要求",
        "context": "用户：旧消息\n助手：旧回复",
    }) as response:
        list(response.iter_lines())
    history = client.get("/internal/projects/p1/chat/history").json()["result"]["content"]
    assert "真正任务" in history
    assert "系统要求" not in history
    assert "旧消息" not in history
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/sandbox && pytest tests/test_chat_context.py -v
```

Expected: FAIL，历史里出现系统提示或旧上下文，或字段被忽略。

- [ ] **Step 3: 修改 chat 请求和 Agent 签名**

`routers/chat.py`：

```python
class ChatRequest(BaseModel):
    message: str = ""
    sessionId: str | None = None
    references: list[ChatReference] = []
    agentRoles: list[AgentRoleInput] = []
    teamMode: bool = False
    systemPrompt: str = ""
    context: str = ""
```

`manager.start` 调用追加：

```python
system_prompt=body.systemPrompt,
context=body.context,
```

`services/chat_turns.py`：

```python
@dataclass
class Turn:
    ...
    system_prompt: str = ""
    context: str = ""


async def start(..., team_mode: bool = False, system_prompt: str = "", context: str = "") -> Turn:
    ...
    turn = Turn(..., system_prompt=system_prompt, context=context)
```

`_run` 调用：

```python
async for event in self.agent.run(
    turn.project_id,
    build_prompt(turn.message, turn.references),
    workspace,
    session_id,
    turn.agent_roles,
    turn.team_mode,
    system_prompt=turn.system_prompt,
    context=turn.context,
):
```

`services/agent.py` 和 `claude_agent.py` 的 `run`/`_run` 增加参数：

```python
system_prompt: str = ""
context: str = ""
```

`StubAgent` 中组合：

```python
prompt = message.strip()
if context:
    prompt = f"最近对话上下文：\n{context}\n\n当前任务：\n{prompt}"
if system_prompt:
    prompt = f"{system_prompt}\n\n{prompt}"
```

`ClaudeAgent.build_options` 增加 `system_prompt` 参数，并把 `role_prompt` 和它合并：

```python
parts = [part for part in [role_prompt, system_prompt] if part]
if parts:
    system_prompt_value = {
        "type": "preset",
        "preset": "claude_code",
        "append": "\n\n".join(parts),
    }
```

`ClaudeAgent._run` 把 `context` 加到消息前：

```python
prompt = message
if context:
    prompt = f"最近对话上下文：\n{context}\n\n当前任务：\n{prompt}"
```

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/sandbox && pytest tests/test_chat_context.py tests/test_api.py tests/test_chat_history.py -v
```

- [ ] **Step 5: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 10: CP 调 sandbox 解析任务

**Files:**
- Modify: `packages/backend/internal/client/sandbox.go`
- Modify: `packages/backend/internal/client/router.go`（仅在需要时）
- Create: `packages/backend/internal/scheduledtask/parser.go`
- Test: `packages/backend/internal/client/sandbox_test.go`
- Test: `packages/backend/internal/scheduledtask/parser_test.go`

**Interfaces:**
- Consumes: `client.Sandbox`、`sandboxregistry.Selector`。
- Produces:
  - `client.TaskDraft`
  - `(*client.Sandbox).ParseTask(ctx, prompt) (TaskDraft, error)`
  - `scheduledtask.NewSandboxParser(selector, obs)`
  - `scheduledtask.DraftParser` interface

- [ ] **Step 1: 写 client 测试**

```go
func TestSandboxParseTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/tasks/parse" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "ok",
			"result": map[string]any{
				"name": "检查", "description": "检查依赖", "scheduleType": "daily",
				"time": "09:00", "weekdays": []int{}, "cronExpression": "0 9 * * *",
				"timezone": "Asia/Shanghai",
			},
		})
	}))
	defer server.Close()
	sandbox := NewSandbox(server.URL, nil)
	draft, err := sandbox.ParseTask(context.Background(), "每天 9 点检查依赖")
	if err != nil { t.Fatal(err) }
	if draft.ScheduleType != "daily" || draft.CronExpression != "0 9 * * *" {
		t.Fatalf("draft = %+v", draft)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/client -run TestSandboxParseTask -v
```

Expected: FAIL，`ParseTask` 不存在。

- [ ] **Step 3: 实现 client.ParseTask**

```go
type TaskDraft struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ScheduleType   string `json:"scheduleType"`
	Time           string `json:"time"`
	Weekdays       []int  `json:"weekdays"`
	CronExpression string `json:"cronExpression"`
	Timezone       string `json:"timezone"`
}

func (s *Sandbox) ParseTask(ctx context.Context, prompt string) (TaskDraft, error) {
	raw, err := s.doJSON(ctx, "parse_task", "", http.MethodPost, "/internal/tasks/parse", map[string]string{"prompt": prompt})
	if err != nil { return TaskDraft{}, err }
	var draft TaskDraft
	if err := json.Unmarshal(raw, &draft); err != nil { return TaskDraft{}, err }
	return draft, nil
}
```

`doJSON` 的 `projectID` 传空字符串不会影响 span 属性；如果 observability 要求非空，传 `"scheduled-parse"`。

- [ ] **Step 4: 写 parser 测试**

```go
type fakeSelector struct{ instance sandboxregistry.Instance; err error }
func (f *fakeSelector) Select(context.Context) (sandboxregistry.Instance, error) { return f.instance, f.err }

func TestSandboxParserRejectsEmptyPrompt(t *testing.T) {
	parser := NewSandboxParser(&fakeSelector{}, nil)
	if _, err := parser.Parse(context.Background(), "  "); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 5: 实现 parser adapter**

```go
type DraftParser interface {
	Parse(ctx context.Context, prompt string) (Draft, error)
}

type SandboxSelector interface {
	Select(ctx context.Context) (sandboxregistry.Instance, error)
}

type SandboxParser struct {
	Selector SandboxSelector
	Obs      *observability.Provider
}

func NewSandboxParser(selector SandboxSelector, obs *observability.Provider) *SandboxParser {
	return &SandboxParser{Selector: selector, Obs: obs}
}

func (p *SandboxParser) Parse(ctx context.Context, prompt string) (Draft, error) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Draft{}, ErrInvalid
	}
	instance, err := p.Selector.Select(ctx)
	if err != nil {
		return Draft{}, err
	}
	sandbox := client.NewSandbox(instance.BaseURL(), p.Obs)
	raw, err := sandbox.ParseTask(ctx, prompt)
	if err != nil {
		return Draft{}, err
	}
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: raw.ScheduleType, Time: raw.Time, Weekdays: raw.Weekdays,
		CronExpression: raw.CronExpression, Timezone: raw.Timezone,
	})
	if err != nil {
		return Draft{}, err
	}
	return Draft{
		Name: raw.Name, Description: raw.Description, ScheduleType: schedule.Type,
		Time: schedule.Time, Weekdays: schedule.Weekdays,
		CronExpression: schedule.CronExpr, Timezone: schedule.Timezone,
	}, nil
}
```

- [ ] **Step 6: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/client ./internal/scheduledtask -v
```

- [ ] **Step 7: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 11: CP Handler 与路由

**Files:**
- Create: `packages/backend/internal/handler/scheduled_task.go`
- Create: `packages/backend/internal/handler/scheduled_task_test.go`
- Create: `packages/backend/internal/router/scheduled_task.go`

**Interfaces:**
- Consumes: `*scheduledtask.Service`、`scheduledtask.DraftParser`、`auth.Session`。
- Produces:
  - `handler.NewScheduledTaskHandler(service, parser)`
  - `router.RegisterScheduledTasks(e, handler, authn)`
  - 用户接口和 admin 接口。

- [ ] **Step 1: 写 handler 测试**

```go
func TestScheduledTaskRoutesRequireIsolation(t *testing.T) {
	e := echo.New()
	authn := &fakeAuthenticator{}
	group := e.Group("/cp/api/v1")
	RegisterScheduledTasksForTest(group, &ScheduledTaskHandler{}, authn)
	routes := e.Routes()
	assertRouteExists(t, routes, "POST", "/cp/api/v1/scheduled-tasks/parse")
	assertRouteExists(t, routes, "GET", "/cp/api/v1/scheduled-tasks")
	assertRouteExists(t, routes, "POST", "/cp/api/v1/admin/scheduled-tasks/:id/stop")
}
```

实际测试使用 `echo.New().Routes()` 断言 9 条用户路由和 4 条 admin 路由；不要依赖真实 service。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/handler -run TestScheduledTaskRoutes -v
```

Expected: FAIL，`RegisterScheduledTasks` 不存在。

- [ ] **Step 3: 实现 handler**

```go
type ScheduledTaskHandler struct {
	Service *scheduledtask.Service
	Parser  scheduledtask.DraftParser
}

func NewScheduledTaskHandler(service *scheduledtask.Service, parser scheduledtask.DraftParser) *ScheduledTaskHandler {
	return &ScheduledTaskHandler{Service: service, Parser: parser}
}

type scheduledTaskInput struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	ScheduleType   string `json:"scheduleType"`
	Time           string `json:"time"`
	Weekdays       []int  `json:"weekdays"`
	CronExpression string `json:"cronExpression"`
	Timezone       string `json:"timezone"`
	Source         string `json:"source"`
	DeleteProject  bool   `json:"deleteProject"`
}

func (h *ScheduledTaskHandler) Parse(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	var req struct{ Prompt string `json:"prompt"` }
	if err := c.Bind(&req); err != nil { return response.Fail(c, http.StatusBadRequest, 40001, "invalid body") }
	if h.Parser == nil { return response.Fail(c, http.StatusServiceUnavailable, 50301, "task parser unavailable") }
	draft, err := h.Parser.Parse(c.Request().Context(), req.Prompt)
	if err != nil { return scheduledTaskError(c, err) }
	_ = session
	return response.OK(c, draft)
}
```

`Create/List/Get/Update/Pause/Resume/Stop/Delete` 从 `auth.SessionFrom(c)` 取 `UserID` 和 `CurrentWorkspaceID`，调用 service。创建前校验 `CurrentWorkspaceID > 0` 且存在于 `WorkspaceIDs`，否则返回 403。`List` 返回 `{"items": tasks}`；`Delete` 返回 `{"deleted": true}`。`Stop` 调用 `Service.Stop`。`AdminList` 调用 `Service.AdminList`，`AdminPause/Resume/Stop` 传 `userID=0` 跳过所有权检查。

- [ ] **Step 4: 实现路由**

```go
func RegisterScheduledTasks(e *echo.Echo, handler *handler.ScheduledTaskHandler, authn *auth.Authenticator) {
	api := e.Group("/cp/api/v1")
	user := api.Group("/scheduled-tasks", authn.RequireUser)
	user.POST("/parse", handler.Parse)
	user.POST("", handler.Create)
	user.GET("", handler.List)
	user.GET("/:id", handler.Get)
	user.PUT("/:id", handler.Update)
	user.POST("/:id/pause", handler.Pause)
	user.POST("/:id/resume", handler.Resume)
	user.POST("/:id/stop", handler.Stop)
	user.DELETE("/:id", handler.Delete)

	admin := api.Group("/admin/scheduled-tasks", authn.RequireOps)
	admin.GET("", handler.AdminList)
	admin.POST("/:id/pause", handler.AdminPause)
	admin.POST("/:id/resume", handler.AdminResume)
	admin.POST("/:id/stop", handler.AdminStop)
}
```

- [ ] **Step 5: 运行测试确认通过**

```bash
cd packages/backend && go test ./internal/handler ./internal/router -v
```

- [ ] **Step 6: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 12: CP 装配与 Dispatcher 注册

**Files:**
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/internal/router/control.go`（只调用 `RegisterScheduledTasks`，不改大签名）
- Test: `packages/backend/internal/router/control_test.go`、`packages/backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: Task 1–11 的 `scheduledtask`、`handler`、`router`。
- Produces: CP 启动时注册 dispatcher/runner、周期 dispatcher、路由。

- [ ] **Step 1: 写路由装配测试**

在 `control_test.go` 增加：

```go
func TestScheduledTaskRoutesRegistered(t *testing.T) {
	e := echo.New()
	authn := auth.NewAuthenticator(nil, auth.CookieManager{}, auth.CookieManager{})
	RegisterScheduledTasks(e, handler.NewScheduledTaskHandler(nil, nil), authn)
	found := false
	for _, route := range e.Routes() {
		if route.Method == http.MethodGet && route.Path == "/cp/api/v1/scheduled-tasks" {
			found = true
		}
	}
	if !found { t.Fatal("scheduled task route not registered") }
}
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/backend && go test ./internal/router -run TestScheduledTaskRoutesRegistered -v
```

Expected: FAIL，`RegisterScheduledTasks` 未在 CP 装配中使用。

- [ ] **Step 3: 修改 `cmd/cp/main.go`**

在 `asynqRedis` 创建后、`e := server.New(cfg, obs)` 之前创建依赖；在 `router.RegisterControl(...)` 之后注册路由：

```go
scheduledRepo := scheduledtask.NewMySQLRepository(db)
scheduledFlags := scheduledtask.NewRedisFlags(redisClient, redisPrefix)
scheduledService := scheduledtask.NewService(
	scheduledRepo, projectStore, provisioner, metas, sandboxRouter, scheduledFlags,
)
scheduledParser := scheduledtask.NewSandboxParser(selector, obs)
webBase := ""
if len(cfg.CorsOrigins) > 0 { webBase = cfg.CorsOrigins[0] }
scheduledRunner := scheduledtask.NewRunner(
	scheduledRepo, sandboxRouter, sandboxRouter, scheduledFlags,
	notificationService, webBase, obs.Logger,
)
scheduledDispatcher := scheduledtask.NewDispatcher(scheduledRepo, asynqClient, obs.Logger)
scheduledHandler := handler.NewScheduledTaskHandler(scheduledService, scheduledParser)
router.RegisterScheduledTasks(e, scheduledHandler, authn)
```

需要先创建 `asynqClient`：

```go
asynqClient := asynq.NewClient(asynqRedis)
defer asynqClient.Close()
```

在 `asynqMux` 注册：

```go
asynqMux.HandleFunc(scheduledtask.TaskTypeDispatch, scheduledDispatcher.HandleTask)
asynqMux.HandleFunc(scheduledtask.TaskTypeRun, scheduledRunner.HandleTask)
```

在 sandbox health 注册后追加：

```go
if _, err := asynqScheduler.Register(
	"@every "+cfg.ScheduledDispatchInterval.String(),
	asynq.NewTask(scheduledtask.TaskTypeDispatch, nil),
	asynq.Unique(cfg.ScheduledDispatchInterval-5*time.Second),
	asynq.MaxRetry(0),
); err != nil {
	obs.Logger.Error("scheduled task dispatcher register failed", "error", err)
	os.Exit(1)
}
```

注意 `cfg.ScheduledDispatchInterval` 必须大于 5 秒，配置测试覆盖默认 30 秒。

- [ ] **Step 4: 运行测试确认通过**

```bash
cd packages/backend && go test ./cmd/cp ./internal/router ./internal/config -v
```

- [ ] **Step 5: 启动 CP 做冒烟**

```bash
make dev-cp
```

Expected: 日志出现 `scheduled task dispatcher register` 或 CP 正常监听 8080，无 panic。

- [ ] **Step 6: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 13: Web API、类型和定时任务页

**Files:**
- Create: `packages/web/src/api/scheduledTasks.ts`
- Create: `packages/web/src/api/scheduledTasks.test.ts`
- Modify: `packages/web/src/types/index.ts`
- Create: `packages/web/src/views/ScheduledTasksView.vue`
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Test: `packages/web/src/api/scheduledTasks.test.ts`

**Interfaces:**
- Consumes: `cpHttp`、`request`。
- Produces:
  - `parseScheduledTask(prompt)`
  - `createScheduledTask(input)`
  - `listScheduledTasks()`
  - `updateScheduledTask(id, input)`
  - `pauseScheduledTask(id)` / `resumeScheduledTask(id)` / `stopScheduledTask(id)`
  - `deleteScheduledTask(id, deleteProject)`
  - 类型 `ScheduledTask`、`ScheduledTaskDraft`

- [ ] **Step 1: 写 API 测试**

```ts
import { describe, expect, it, vi } from 'vitest'
import { cpHttp } from './client'
import { deleteScheduledTask, parseScheduledTask } from './scheduledTasks'

describe('scheduledTasks api', () => {
  it('unwraps parse draft', async () => {
    vi.spyOn(cpHttp, 'request').mockResolvedValue({
      data: { code: 0, message: 'ok', result: { name: '检查', scheduleType: 'daily' } },
    } as never)
    await expect(parseScheduledTask('每天检查')).resolves.toMatchObject({ name: '检查' })
  })

  it('passes deleteProject flag', async () => {
    const spy = vi.spyOn(cpHttp, 'request').mockResolvedValue({
      data: { code: 0, message: 'ok', result: { id: 1 } },
    } as never)
    await deleteScheduledTask(1, true)
    expect(spy).toHaveBeenCalledWith(expect.objectContaining({
      method: 'delete',
      url: '/scheduled-tasks/1',
      params: { deleteProject: true },
    }))
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/web && npx vitest run src/api/scheduledTasks.test.ts
```

Expected: FAIL，模块不存在。

- [ ] **Step 3: 实现 API 与类型**

`api/scheduledTasks.ts`：

```ts
import { cpHttp, request } from './client'
import type { ScheduledTask, ScheduledTaskDraft, ScheduledTaskInput } from '@/types'

export function parseScheduledTask(prompt: string): Promise<ScheduledTaskDraft> {
  return request<ScheduledTaskDraft>(cpHttp, {
    method: 'post',
    url: '/scheduled-tasks/parse',
    data: { prompt },
    timeout: 45_000,
  })
}

export function createScheduledTask(input: ScheduledTaskInput): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, {
    method: 'post',
    url: '/scheduled-tasks',
    data: input,
  })
}

export async function listScheduledTasks(): Promise<ScheduledTask[]> {
  const result = await request<{ items: ScheduledTask[] }>(cpHttp, {
    method: 'get',
    url: '/scheduled-tasks',
  })
  return result.items ?? []
}

export function updateScheduledTask(id: number, input: ScheduledTaskInput): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, {
    method: 'put',
    url: `/scheduled-tasks/${id}`,
    data: input,
  })
}

export function pauseScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/pause` })
}

export function resumeScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/resume` })
}

export function stopScheduledTask(id: number): Promise<ScheduledTask> {
  return request<ScheduledTask>(cpHttp, { method: 'post', url: `/scheduled-tasks/${id}/stop` })
}

export async function deleteScheduledTask(id: number, deleteProject: boolean): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/scheduled-tasks/${id}`,
    params: { deleteProject },
  })
}
```

`types/index.ts` 追加：

```ts
export interface ScheduledTaskDraft {
  name: string
  description: string
  scheduleType: 'daily' | 'weekly' | 'cron'
  time: string
  weekdays: number[]
  cronExpression: string
  timezone: string
}

export interface ScheduledTaskInput extends ScheduledTaskDraft {
  source?: 'ai' | 'manual'
}

export interface ScheduledTask {
  id: number
  name: string
  description: string
  scheduleType: 'daily' | 'weekly' | 'cron'
  time: string
  weekdays: number[]
  cronExpression: string
  timezone: string
  source: 'ai' | 'manual'
  enabled: boolean
  projectId: string
  nextRunAt?: string
  lastRunAt?: string
  lastStatus: string
  lastError: string
  runCount: number
}
```

- [ ] **Step 4: 实现页面**

`ScheduledTasksView.vue` 使用现有 `home.css` 的布局和配色，结构：

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createScheduledTask, deleteScheduledTask, listScheduledTasks,
  parseScheduledTask, pauseScheduledTask, resumeScheduledTask,
  stopScheduledTask, updateScheduledTask,
} from '@/api/scheduledTasks'
import type { ScheduledTask, ScheduledTaskDraft, ScheduledTaskInput } from '@/types'

const tasks = ref<ScheduledTask[]>([])
const loading = ref(false)
const error = ref('')
const notice = ref('')
const dialogOpen = ref(false)
const mode = ref<'ai' | 'manual'>('ai')
const prompt = ref('')
const draft = ref<ScheduledTaskDraft | null>(null)
const editing = ref<ScheduledTask | null>(null)
const deleteTarget = ref<ScheduledTask | null>(null)
const deleteProject = ref(false)
const form = ref<ScheduledTaskInput>({
  name: '', description: '', scheduleType: 'daily', time: '09:00',
  weekdays: [], cronExpression: '0 9 * * *', timezone: 'Asia/Shanghai',
})
const timezones = ['Asia/Shanghai', 'Asia/Tokyo', 'UTC', 'America/Los_Angeles']
const weekdayOptions = [
  { label: '周日', value: 0 }, { label: '周一', value: 1 }, { label: '周二', value: 2 },
  { label: '周三', value: 3 }, { label: '周四', value: 4 }, { label: '周五', value: 5 },
  { label: '周六', value: 6 },
]

async function load() {
  loading.value = true
  error.value = ''
  try { tasks.value = await listScheduledTasks() }
  catch (err) { error.value = err instanceof Error ? err.message : '加载失败' }
  finally { loading.value = false }
}

function openEdit(task: ScheduledTask) {
  mode.value = 'manual'
  editing.value = task
  draft.value = null
  form.value = {
    name: task.name, description: task.description, scheduleType: task.scheduleType,
    time: task.time || '09:00', weekdays: [...task.weekdays],
    cronExpression: task.cronExpression, timezone: task.timezone,
  }
  dialogOpen.value = true
}

function openCreate(nextMode: 'ai' | 'manual') {
  mode.value = nextMode
  editing.value = null
  draft.value = null
  prompt.value = ''
  form.value = { name: '', description: '', scheduleType: 'daily', time: '09:00', weekdays: [], cronExpression: '0 9 * * *', timezone: 'Asia/Shanghai' }
  dialogOpen.value = true
}

async function generateDraft() {
  error.value = ''
  try {
    draft.value = await parseScheduledTask(prompt.value)
    form.value = { ...draft.value, source: 'ai' }
  } catch (err) { error.value = err instanceof Error ? err.message : '解析失败，请改用手动设置' }
}

async function save() {
  error.value = ''
  try {
    if (editing.value) { await updateScheduledTask(editing.value.id, form.value) }
    else { await createScheduledTask(form.value) }
    dialogOpen.value = false
    notice.value = '任务已保存'
    await load()
  } catch (err) { error.value = err instanceof Error ? err.message : '保存失败' }
}

async function toggle(task: ScheduledTask) {
  if (task.enabled) { await pauseScheduledTask(task.id) } else { await resumeScheduledTask(task.id) }
  await load()
}

async function stop(task: ScheduledTask) { await stopScheduledTask(task.id); await load() }

function requestDelete(task: ScheduledTask) { deleteTarget.value = task; deleteProject.value = false }
async function confirmDelete() {
  if (!deleteTarget.value) return
  await deleteScheduledTask(deleteTarget.value.id, deleteProject.value)
  deleteTarget.value = null
  await load()
}

onMounted(load)
</script>
```

模板：

```vue
<template>
  <main class="projects-page">
    <header class="projects-head">
      <div>
        <h1>定时任务</h1>
        <p>让 Vibebiz 安排任务、设置提醒，或监控更新。</p>
      </div>
      <div class="task-create">
        <Button label="使用 Vibebiz 创建" @click="openCreate('ai')" />
        <Button label="手动设置" text @click="openCreate('manual')" />
      </div>
    </header>

    <p v-if="error" class="error-text">{{ error }}</p>
    <p v-if="notice" class="notice-text">{{ notice }}</p>
    <p v-if="loading">加载中...</p>

    <div v-else-if="tasks.length === 0" class="scheduled-empty">
      <p>还没有定时任务。</p>
      <div class="task-create">
        <Button label="使用 Vibebiz 创建" @click="openCreate('ai')" />
        <Button label="手动设置" text @click="openCreate('manual')" />
      </div>
    </div>

    <div v-else class="project-grid">
      <article v-for="task in tasks" :key="task.id" class="project-card">
        <div class="project-info">
          <strong>{{ task.name }}</strong>
          <span>{{ task.cronExpression }} · {{ task.timezone }}</span>
          <span>{{ task.enabled ? '启用' : '暂停' }} · {{ task.lastStatus || '未执行' }}</span>
          <span v-if="task.lastError" class="error-text">{{ task.lastError }}</span>
        </div>
        <div class="task-actions">
          <Button label="编辑" text @click="openEdit(task)" />
          <Button :label="task.enabled ? '暂停' : '恢复'" text @click="toggle(task)" />
          <Button v-if="task.lastStatus === 'running'" label="停止" text @click="stop(task)" />
          <Button label="删除" text @click="requestDelete(task)" />
          <a v-if="task.projectId" :href="`/editor/${task.projectId}`">打开项目</a>
        </div>
      </article>
    </div>

    <Dialog v-model:visible="dialogOpen" header="定时任务" modal>
      <div v-if="mode === 'ai' && !draft" class="task-ai">
        <Textarea v-model="prompt" rows="5" placeholder="例如：每天早上 9 点检查项目依赖有没有更新" />
        <Button label="生成任务草稿" @click="generateDraft" />
      </div>
      <div v-else class="task-form">
        <label>任务名称<InputText v-model="form.name" /></label>
        <label>任务描述<Textarea v-model="form.description" rows="4" /></label>
        <label>频率
          <Select v-model="form.scheduleType" :options="['daily', 'weekly', 'cron']" />
        </label>
        <label v-if="form.scheduleType !== 'cron'">时间
          <InputText v-model="form.time" placeholder="09:00" />
        </label>
        <label v-if="form.scheduleType === 'weekly'">星期
          <Select
            v-model="form.weekdays"
            :options="weekdayOptions"
            option-label="label"
            option-value="value"
            multiple
          />
        </label>
        <label v-if="form.scheduleType === 'cron'">Cron
          <InputText v-model="form.cronExpression" placeholder="0 9 * * *" />
        </label>
        <label>时区<Select v-model="form.timezone" :options="timezones" /></label>
      </div>
      <template #footer>
        <Button label="取消" text @click="dialogOpen = false" />
        <Button label="保存" @click="save" />
      </template>
    </Dialog>

    <Dialog
      :visible="Boolean(deleteTarget)"
      header="删除定时任务"
      modal
      @update:visible="deleteTarget = null"
    >
      <p>确定删除「{{ deleteTarget?.name }}」？</p>
      <label><Checkbox v-model="deleteProject" binary /> 同时删除绑定项目</label>
      <template #footer>
        <Button label="取消" text @click="deleteTarget = null" />
        <Button label="删除" @click="confirmDelete" />
      </template>
    </Dialog>
  </main>
</template>
```

在 `router/index.ts` 增加：

```ts
{ path: '/scheduled-tasks', name: 'scheduled-tasks', component: ScheduledTasksView },
```

在 `HomeSidebar.vue` 把：

```ts
{ label: '定时任务', icon: 'pi pi-clock', route: '', visible: true },
```

改为：

```ts
{ label: '定时任务', icon: 'pi pi-clock', route: 'scheduled-tasks', visible: true },
```

- [ ] **Step 5: 运行测试与类型检查**

```bash
cd packages/web && npx vitest run src/api/scheduledTasks.test.ts && npm run lint && npx vue-tsc --noEmit
```

- [ ] **Step 6: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 14: Ops-web 定时任务管理

**Files:**
- Modify: `packages/ops-web/src/api/admin.ts`
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/routes.ts`
- Create: `packages/ops-web/src/views/ScheduledTasksView.vue`
- Test: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: `request`。
- Produces:
  - `getAdminScheduledTasks(query)`
  - `pauseAdminScheduledTask(id)` / `resumeAdminScheduledTask(id)` / `stopAdminScheduledTask(id)`
  - `AdminScheduledTask` 类型

- [ ] **Step 1: 更新导航测试**

```ts
it('exposes the scheduled task module', () => {
  expect(navItems.map((item) => item.path)).toContain('/scheduled-tasks')
  expect(navItems.map((item) => item.name)).toContain('scheduled-tasks')
})
```

同时更新原来的“exposes the twelve admin modules”数组，加入 `/scheduled-tasks`。

- [ ] **Step 2: 运行测试确认失败**

```bash
cd packages/ops-web && npx vitest run src/router/navigation.test.ts
```

Expected: FAIL，缺少导航项。

- [ ] **Step 3: 实现 API 与类型**

`types/index.ts`：

```ts
export interface AdminScheduledTask {
  id: number
  userId: number
  username: string
  name: string
  projectId: string
  cronExpression: string
  timezone: string
  enabled: boolean
  lastStatus: string
  lastError: string
  lastRunAt?: string
  nextRunAt?: string
  runCount: number
}
```

`api/admin.ts`：

```ts
export async function getAdminScheduledTasks(query: PageQuery = {}): Promise<PageResult<AdminScheduledTask>> {
  const result = await request<Record<string, unknown>>({
    method: 'get',
    url: '/scheduled-tasks',
    params: pageParams(query),
  })
  return toPage<AdminScheduledTask>(result, 'items')
}

export function pauseAdminScheduledTask(id: number): Promise<AdminScheduledTask> {
  return request<AdminScheduledTask>({ method: 'post', url: `/scheduled-tasks/${id}/pause` })
}

export function resumeAdminScheduledTask(id: number): Promise<AdminScheduledTask> {
  return request<AdminScheduledTask>({ method: 'post', url: `/scheduled-tasks/${id}/resume` })
}

export function stopAdminScheduledTask(id: number): Promise<AdminScheduledTask> {
  return request<AdminScheduledTask>({ method: 'post', url: `/scheduled-tasks/${id}/stop` })
}
```

- [ ] **Step 4: 实现 ops 页面**

`ScheduledTasksView.vue` 复用 `useAdminPage` 和 `DataTable`：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'

import {
  getAdminScheduledTasks, pauseAdminScheduledTask,
  resumeAdminScheduledTask, stopAdminScheduledTask,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminScheduledTask, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getAdminScheduledTasks)

const actionError = ref('')
const columns: TableColumn[] = [
  { key: 'username', label: '用户', width: '120px' },
  { key: 'name', label: '任务名称' },
  { key: 'projectId', label: '项目', width: '180px' },
  { key: 'cronExpression', label: 'Cron', width: '140px' },
  { key: 'timezone', label: '时区', width: '150px' },
  { key: 'enabled', label: '状态', width: '90px', format: (value) => Number(value) ? '启用' : '暂停' },
  { key: 'lastStatus', label: '上次状态', width: '100px' },
  { key: 'lastRunAt', label: '上次执行', width: '170px' },
  { key: 'runCount', label: '次数', width: '80px' },
  { key: 'actions', label: '操作', width: '180px' },
]

async function toggle(row: unknown) {
  const task = row as AdminScheduledTask
  try {
    if (task.enabled) await pauseAdminScheduledTask(task.id)
    else await resumeAdminScheduledTask(task.id)
    await load()
  } catch (err) { actionError.value = err instanceof Error ? err.message : '操作失败' }
}

async function stop(row: unknown) {
  const task = row as AdminScheduledTask
  try { await stopAdminScheduledTask(task.id); await load() }
  catch (err) { actionError.value = err instanceof Error ? err.message : '停止失败' }
}
</script>
```

模板：

```vue
<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>定时任务</h1>
        <p class="page-desc">查看用户定时任务，可暂停、恢复或停止执行。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无定时任务"
      @retry="load"
    >
      <template #cell-enabled="{ row }">
        <span class="tag">{{ (row as AdminScheduledTask).enabled ? '启用' : '暂停' }}</span>
      </template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminScheduledTask).enabled ? '暂停' : '恢复'"
            size="small"
            text
            @click="toggle(row)"
          />
          <Button
            v-if="(row as AdminScheduledTask).lastStatus === 'running'"
            label="停止执行"
            size="small"
            text
            severity="danger"
            @click="stop(row)"
          />
        </div>
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
```

在 `navigation.ts` 加：

```ts
{ name: 'scheduled-tasks', label: '定时任务', path: '/scheduled-tasks', icon: 'pi pi-clock' },
```

在 `routes.ts` 加：

```ts
{ path: 'scheduled-tasks', name: 'scheduled-tasks', component: ScheduledTasksView, meta: { title: '定时任务' } },
```

- [ ] **Step 5: 运行测试与类型检查**

```bash
cd packages/ops-web && npx vitest run && npm run lint && npx vue-tsc --noEmit
```

- [ ] **Step 6: Checkpoint**

```bash
make validate
make checkpoint
```

---

### Task 15: 端到端联调与质量门禁

**Files:**
- Modify: `packages/backend/internal/scheduledtask/*_test.go`（仅修 bug）
- Modify: `packages/sandbox/tests/*.py`（仅修 bug）
- Modify: `packages/web/src/**/*.test.ts`（仅修 bug）
- Modify: `packages/ops-web/src/**/*.test.ts`（仅修 bug）

**Interfaces:**
- Consumes: Task 1–14。
- Produces: 可运行、可验证的定时任务模块。

- [ ] **Step 1: 启动依赖**

```bash
make dev
```

- [ ] **Step 2: 迁移数据库**

```bash
cd packages/backend
set -a; source .env; set +a
go run ./cmd/migrate up
go run ./cmd/migrate version
```

Expected: version 至少 20。

- [ ] **Step 3: 手动验证 AI 创建**

1. 打开 web，进入“定时任务”。
2. 点击“创建 -> 使用 Vibebiz 创建”。
3. 输入“每天早上 9 点检查项目依赖并汇报更新”。
4. 确认草稿出现，频率为每日，时间 09:00，时区 Asia/Shanghai。
5. 点击创建。
6. 检查 `scheduled_task` 新增一行，`enabled=1`，`project_id` 非空，`cron_expr='0 9 * * *'`。
7. 检查对应项目出现在“我的项目”。

- [ ] **Step 4: 手动验证手动创建与编辑**

1. 点击“手动设置”，填写任务描述和每周计划。
2. 创建后编辑名称和频率。
3. 确认 `next_run_at` 随编辑变化。
4. 暂停任务，确认 `enabled=0`、`next_run_at=NULL`。
5. 恢复任务，确认 `enabled=1`、`next_run_at` 有值。

- [ ] **Step 5: 手动验证触发执行**

把 `next_run_at` 改成当前时间前一分钟：

```sql
UPDATE scheduled_task SET next_run_at = NOW() - INTERVAL 1 MINUTE WHERE id = ?;
```

等待一个 dispatcher tick，确认：

- `last_status` 先变为 `running`，再变为 `success/failed`
- `last_run_at`、`run_count` 更新
- 固定项目聊天历史新增任务消息
- web 消息通知出现成功/失败通知

- [ ] **Step 6: 手动验证停止**

1. 把任务 cron 设置为每分钟，等待执行。
2. 在 web 或 ops 点击“停止执行”。
3. 确认 sandbox 当前 turn 被停止，`last_status=stopped`。

- [ ] **Step 7: 手动验证 ops 隔离**

1. ops 登录，进入“定时任务”。
2. 确认能查看任务、暂停/恢复/停止。
3. 确认没有编辑、删除按钮。
4. 确认请求 `/cp/api/v1/admin/scheduled-tasks/:id` 的 DELETE 不存在。

- [ ] **Step 8: 运行全部测试**

```bash
make quality
make validate
make -C packages/backend test
make -C packages/sandbox test
make -C packages/web test
make -C packages/ops-web test
```

- [ ] **Step 9: Checkpoint**

```bash
make checkpoint
```

---

## Self-Review Checklist

- [ ] 每个 spec 要求都有对应任务：
  - AI/手动创建：Task 8、13
  - 固定项目：Task 3
  - `scheduled` 主题：Task 1
  - DB 到期扫描 + Asynq Unique：Task 6、12
  - 最近 5 条历史 + systemPrompt/context：Task 7、9
  - 用户编辑/暂停/恢复/停止/删除：Task 3、11、13
  - ops 查看/暂停/恢复/停止，无编辑删除：Task 11、14
  - 通知：Task 7
  - 测试与质量门禁：Task 15
- [ ] 所有类型名一致：`scheduledtask.Task`、`ScheduledTask`、`ScheduledTaskDraft`、`AdminScheduledTask`。
- [ ] 所有接口路径一致：`/cp/api/v1/scheduled-tasks`、`/cp/api/v1/admin/scheduled-tasks`、`/internal/tasks/parse`。
- [ ] 没有占位符、TBD 或“类似 Task N”。
- [ ] 没有要求提交 git；所有 checkpoint 只运行测试和 `make checkpoint`。

package scheduledtask

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"vibebiz/internal/auth"
	daomodel "vibebiz/internal/dao/model"
	appmodel "vibebiz/internal/model"
	"vibebiz/internal/store"
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
	Type     string
	Time     string
	Weekdays []int
	CronExpr string
	Timezone string
	Location *time.Location
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
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description"`
	ScheduleType   string  `json:"scheduleType"`
	Time           string  `json:"time"`
	Weekdays       []int   `json:"weekdays"`
	CronExpression string  `json:"cronExpression"`
	Timezone       string  `json:"timezone"`
	Source         string  `json:"source"`
	Enabled        bool    `json:"enabled"`
	ProjectID      string  `json:"projectId"`
	NextRunAt      *string `json:"nextRunAt,omitempty"`
	LastRunAt      *string `json:"lastRunAt,omitempty"`
	LastStatus     string  `json:"lastStatus"`
	LastError      string  `json:"lastError"`
	RunCount       int64   `json:"runCount"`
	UserID         int64   `json:"userId,omitempty"`
	Username       string  `json:"username,omitempty"`
	WorkspaceID    int64   `json:"workspaceId,omitempty"`
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

type ImmediateEnqueuer interface {
	EnqueueNow(ctx context.Context, taskID int64) error
}

type Service struct {
	Repo        Repository
	Projects    store.ProjectStore
	Initializer ProjectInitializer
	Metas       MetaStore
	Sandbox     StopChatClient
	Flags       Flags
	Immediate   ImmediateEnqueuer
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
	if userID <= 0 || workspaceID <= 0 || s.Projects == nil || s.Initializer == nil {
		return Task{}, ErrInvalid
	}
	description := strings.TrimSpace(input.Description)
	if description == "" || len([]rune(description)) > 8000 {
		return Task{}, ErrInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		runes := []rune(description)
		if len(runes) > 24 {
			runes = runes[:24]
		}
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
		if s.Metas != nil {
			_ = s.Metas.Delete(ctx, publicID)
		}
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
		NextRunAt: &next, LastStatus: StatusPending, LastRunAt: &now, RunCount: 1,
		Base: daomodel.Base{ExtraP: extra},
	}
	if err := s.Repo.Create(ctx, item); err != nil {
		cleanup()
		return Task{}, err
	}
	if s.Immediate != nil {
		if err := s.Immediate.EnqueueNow(ctx, item.ID); err != nil {
			_ = s.Repo.Update(ctx, item.ID, map[string]any{
				"last_status": StatusFailed, "last_error": "first run enqueue failed: " + err.Error(),
			})
			item.LastStatus = StatusFailed
			item.LastError = "first run enqueue failed: " + err.Error()
		}
	}
	return s.view(item), nil
}

func (s *Service) List(ctx context.Context, userID int64, workspaceIDs []int64) ([]Task, error) {
	items, err := s.Repo.ListByUser(ctx, userID, workspaceIDs)
	if err != nil {
		return nil, err
	}
	out := make([]Task, 0, len(items))
	for index := range items {
		out = append(out, s.view(&items[index]))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if item.UserID != userID {
		return Task{}, ErrForbidden
	}
	return s.view(item), nil
}

func (s *Service) Update(ctx context.Context, id, userID int64, input UpdateInput) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if item.UserID != userID {
		return Task{}, ErrForbidden
	}
	name := strings.TrimSpace(input.Name)
	description := strings.TrimSpace(input.Description)
	if name == "" || len([]rune(name)) > 128 || description == "" || len([]rune(description)) > 8000 {
		return Task{}, ErrInvalid
	}
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: input.ScheduleType, Time: input.Time, Weekdays: input.Weekdays,
		CronExpression: input.CronExpression, Timezone: input.Timezone,
	})
	if err != nil {
		return Task{}, err
	}
	next, err := NextRun(schedule, s.Now().UTC())
	if err != nil {
		return Task{}, err
	}
	extra, _ := json.Marshal(extraFields{
		Source: sourceOf(item), ScheduleType: schedule.Type, Time: schedule.Time,
		Weekdays: schedule.Weekdays, ProjectPublicID: projectIDOf(item),
	})
	fields := map[string]any{
		"name": name, "prompt": description, "cron_expr": schedule.CronExpr,
		"timezone": schedule.Timezone, "next_run_at": next, "extra_p": extra,
	}
	if item.Enabled == 1 {
		fields["last_status"] = StatusPending
	}
	if err := s.Repo.Update(ctx, id, fields); err != nil {
		return Task{}, err
	}
	item.Name, item.Prompt, item.CronExpr, item.Timezone = name, description, schedule.CronExpr, schedule.Timezone
	item.NextRunAt, item.ExtraP = &next, extra
	return s.view(item), nil
}

func (s *Service) Pause(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if userID > 0 && item.UserID != userID {
		return Task{}, ErrForbidden
	}
	if err := s.Repo.Update(ctx, id, map[string]any{"enabled": 0, "next_run_at": nil}); err != nil {
		return Task{}, err
	}
	item.Enabled = 0
	item.NextRunAt = nil
	return s.view(item), nil
}

func (s *Service) Resume(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if userID > 0 && item.UserID != userID {
		return Task{}, ErrForbidden
	}
	schedule, err := NormalizeSchedule(ScheduleInput{
		Type: inferScheduleType(item), Time: timeOf(item), Weekdays: weekdaysOf(item),
		CronExpression: item.CronExpr, Timezone: item.Timezone,
	})
	if err != nil {
		return Task{}, err
	}
	next, err := NextRun(schedule, s.Now().UTC())
	if err != nil {
		return Task{}, err
	}
	if err := s.Repo.Update(ctx, id, map[string]any{"enabled": 1, "next_run_at": next}); err != nil {
		return Task{}, err
	}
	item.Enabled = 1
	item.NextRunAt = &next
	return s.view(item), nil
}

func (s *Service) Stop(ctx context.Context, id, userID int64) (Task, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return Task{}, err
	}
	if userID > 0 && item.UserID != userID {
		return Task{}, ErrForbidden
	}
	if item.LastStatus != StatusRunning {
		return s.view(item), nil
	}
	if s.Flags != nil {
		if err := s.Flags.MarkStop(ctx, id, 10*time.Minute); err != nil {
			return Task{}, err
		}
	}
	if s.Sandbox != nil {
		if err := s.Sandbox.StopChat(ctx, projectIDOf(item)); err != nil {
			return Task{}, err
		}
	}
	if err := s.Repo.Update(ctx, id, map[string]any{"last_status": StatusStopped}); err != nil {
		return Task{}, err
	}
	item.LastStatus = StatusStopped
	return s.view(item), nil
}

func (s *Service) AdminList(ctx context.Context, filter ListFilter) (Page, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	items, total, err := s.Repo.List(ctx, filter)
	if err != nil {
		return Page{}, err
	}
	ids := make([]int64, 0, len(items))
	for index := range items {
		ids = append(ids, items[index].UserID)
	}
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
	if err != nil {
		return err
	}
	if item.UserID != userID {
		return ErrForbidden
	}
	if item.LastStatus == StatusRunning {
		return ErrInvalid
	}
	if deleteProject {
		projectID := projectIDOf(item)
		if projectID == "" {
			return ErrInvalid
		}
		if err := s.Projects.Delete(ctx, projectID); err != nil && !errors.Is(err, store.ErrNotFound) {
			return err
		}
		if s.Metas != nil {
			_ = s.Metas.Delete(ctx, projectID)
		}
	}
	return s.Repo.Delete(ctx, id)
}

func (s *Service) view(item *daomodel.ScheduledTask) Task {
	extra := extraOf(item)
	enabled := item.Enabled == 1
	view := Task{
		ID: item.ID, Name: item.Name, Description: item.Prompt,
		ScheduleType: extra.ScheduleType, Time: extra.Time, Weekdays: extra.Weekdays,
		CronExpression: item.CronExpr, Timezone: item.Timezone,
		Source: extra.Source, Enabled: enabled, ProjectID: extra.ProjectPublicID,
		LastStatus: item.LastStatus, LastError: item.LastError, RunCount: item.RunCount,
		UserID: item.UserID,
	}
	if item.WorkspaceID != nil {
		view.WorkspaceID = *item.WorkspaceID
	}
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
	if extra.Source == "" {
		extra.Source = SourceManual
	}
	if extra.ScheduleType == "" {
		extra.ScheduleType = inferScheduleType(item)
	}
	return extra
}

func projectIDOf(item *daomodel.ScheduledTask) string { return extraOf(item).ProjectPublicID }
func sourceOf(item *daomodel.ScheduledTask) string    { return extraOf(item).Source }
func timeOf(item *daomodel.ScheduledTask) string      { return extraOf(item).Time }
func weekdaysOf(item *daomodel.ScheduledTask) []int   { return extraOf(item).Weekdays }

func inferScheduleType(item *daomodel.ScheduledTask) string {
	fields := strings.Fields(item.CronExpr)
	if len(fields) != 5 {
		return ScheduleCron
	}
	if fields[2] == "*" && fields[3] == "*" && fields[4] == "*" {
		return ScheduleDaily
	}
	if fields[2] == "*" && fields[3] == "*" {
		return ScheduleWeekly
	}
	return ScheduleCron
}

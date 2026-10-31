package scheduledtask

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/auth"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/store"
)

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
		if f.items[index].ID != id {
			continue
		}
		if value, ok := fields["name"].(string); ok {
			f.items[index].Name = value
		}
		if value, ok := fields["prompt"].(string); ok {
			f.items[index].Prompt = value
		}
		if value, ok := fields["cron_expr"].(string); ok {
			f.items[index].CronExpr = value
		}
		if value, ok := fields["timezone"].(string); ok {
			f.items[index].Timezone = value
		}
		if value, ok := fields["enabled"].(int); ok {
			f.items[index].Enabled = int8(value)
		}
		if value, ok := fields["next_run_at"].(*time.Time); ok {
			f.items[index].NextRunAt = value
		}
		if value, ok := fields["extra_p"].([]byte); ok {
			f.items[index].ExtraP = value
		}
		return nil
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

func (f *fakeRepo) Due(context.Context, time.Time, int) ([]daomodel.ScheduledTask, error) {
	return nil, nil
}

func (f *fakeRepo) Claim(context.Context, int64, time.Time, time.Time, time.Time) (bool, error) {
	return true, nil
}

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

type fakeInitializer struct {
	id  int64
	err error
}

func (f *fakeInitializer) InitProject(context.Context, string) (int64, error) {
	return f.id, f.err
}

type fakeImmediate struct {
	ids []int64
	err error
}

func (f *fakeImmediate) EnqueueNow(_ context.Context, taskID int64) error {
	f.ids = append(f.ids, taskID)
	return f.err
}

type fakeMetas struct {
	set map[string]auth.ProjectMeta
}

func (f *fakeMetas) Set(_ context.Context, id string, meta auth.ProjectMeta) error {
	if f.set == nil {
		f.set = map[string]auth.ProjectMeta{}
	}
	f.set[id] = meta
	return nil
}

func (f *fakeMetas) Delete(_ context.Context, id string) error {
	delete(f.set, id)
	return nil
}

func newServiceFixture(t *testing.T) (*Service, *fakeRepo, *store.MemoryProjectStore, *fakeMetas) {
	t.Helper()
	repo := &fakeRepo{topics: []daomodel.ProjectTopic{{
		Base: daomodel.Base{ID: 9}, TopicCode: TopicCodeScheduled, TopicName: "定时任务", Status: 1,
	}}}
	projects := store.NewMemoryProjectStore()
	metas := &fakeMetas{}
	service := NewService(repo, projects, &fakeInitializer{id: 77}, metas, nil, nil)
	service.Now = func() time.Time { return time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC) }
	service.NewID = func() string { return "task-project-1" }
	return service, repo, projects, metas
}

func TestCreateScheduledTaskCreatesProject(t *testing.T) {
	service, repo, projects, metas := newServiceFixture(t)
	immediate := &fakeImmediate{}
	service.Immediate = immediate
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
	if len(repo.items) != 1 || repo.items[0].ProjectID == nil {
		t.Fatalf("task = %+v", repo.items)
	}
	if metas.set["task-project-1"].OwnerUserID != 7 {
		t.Fatalf("meta = %+v", metas.set)
	}
	if len(immediate.ids) != 1 || immediate.ids[0] != view.ID {
		t.Fatalf("immediate = %+v", immediate.ids)
	}
	if view.RunCount != 1 {
		t.Fatalf("run count = %d", view.RunCount)
	}
}

func TestUpdateScheduleRecomputesNextRun(t *testing.T) {
	service, _, _, _ := newServiceFixture(t)
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

func TestCreateRejectsMissingTopic(t *testing.T) {
	service, repo, _, _ := newServiceFixture(t)
	repo.topics = nil
	_, err := service.Create(context.Background(), CreateInput{
		Description: "检查依赖", ScheduleType: ScheduleDaily, Time: "09:00",
	}, 7, 5)
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v", err)
	}
}

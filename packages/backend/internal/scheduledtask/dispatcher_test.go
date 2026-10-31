package scheduledtask

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/hibiken/asynq"

	daomodel "vibebiz/internal/dao/model"
)

type fakeDueRepo struct {
	*fakeRepo
	due     []daomodel.ScheduledTask
	claimed map[int64]bool
}

func (f *fakeDueRepo) Due(context.Context, time.Time, int) ([]daomodel.ScheduledTask, error) {
	return f.due, nil
}

func (f *fakeDueRepo) Claim(_ context.Context, id int64, _, _, _ time.Time) (bool, error) {
	f.claimed[id] = true
	return true, nil
}

type fakeEnqueuer struct {
	tasks []*asynq.Task
}

func (f *fakeEnqueuer) EnqueueContext(_ context.Context, task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "x"}, nil
}

func ptrTime(value time.Time) *time.Time { return &value }

func TestDispatcherClaimsAndEnqueues(t *testing.T) {
	now := time.Date(2026, 10, 30, 1, 0, 0, 0, time.UTC)
	item := daomodel.ScheduledTask{
		Base:     daomodel.Base{ID: 3, ExtraP: []byte(`{"scheduleType":"daily","time":"09:00","projectPublicId":"p1"}`)},
		CronExpr: "0 9 * * *", Timezone: "Asia/Shanghai",
		Enabled: 1, NextRunAt: ptrTime(now.Add(-time.Minute)),
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

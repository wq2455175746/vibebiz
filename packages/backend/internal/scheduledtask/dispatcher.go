package scheduledtask

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"
)

const (
	TaskTypeDispatch = "scheduled_task:dispatch"
	TaskTypeRun      = "scheduled_task:run"
)

type Enqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

type Dispatcher struct {
	Repo       Repository
	Client     Enqueuer
	Logger     *slog.Logger
	Now        func() time.Time
	RunTimeout time.Duration
}

func NewDispatcher(repo Repository, client Enqueuer, logger *slog.Logger) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{Repo: repo, Client: client, Logger: logger, Now: time.Now, RunTimeout: 15 * time.Minute}
}

func (d *Dispatcher) HandleTask(ctx context.Context, _ *asynq.Task) error {
	now := d.Now().UTC()
	items, err := d.Repo.Due(ctx, now, 100)
	if err != nil {
		return err
	}
	for index := range items {
		item := items[index]
		if item.NextRunAt == nil {
			continue
		}
		oldNext := *item.NextRunAt
		schedule, err := NormalizeSchedule(ScheduleInput{
			Type: inferScheduleType(&item), Time: timeOf(&item), Weekdays: weekdaysOf(&item),
			CronExpression: item.CronExpr, Timezone: item.Timezone,
		})
		if err != nil {
			_ = d.Repo.Update(ctx, item.ID, map[string]any{
				"enabled": 0, "last_error": "invalid schedule: " + err.Error(),
			})
			continue
		}
		next, err := NextRun(schedule, now)
		if err != nil {
			_ = d.Repo.Update(ctx, item.ID, map[string]any{
				"enabled": 0, "last_error": "invalid next run: " + err.Error(),
			})
			continue
		}
		claimed, err := d.Repo.Claim(ctx, item.ID, oldNext, next, now)
		if err != nil || !claimed {
			continue
		}
		if err := d.enqueueRun(ctx, item.ID); err != nil {
			d.Logger.Error("scheduled task enqueue failed", "task_id", item.ID, "error", err)
			_ = d.Repo.Update(ctx, item.ID, map[string]any{
				"last_status": StatusFailed, "last_error": err.Error(),
			})
		}
	}
	return nil
}

func (d *Dispatcher) EnqueueNow(ctx context.Context, taskID int64) error {
	return d.enqueueRun(ctx, taskID)
}

func (d *Dispatcher) enqueueRun(ctx context.Context, taskID int64) error {
	payload, _ := json.Marshal(map[string]int64{"taskId": taskID})
	_, err := d.Client.EnqueueContext(ctx, asynq.NewTask(TaskTypeRun, payload),
		asynq.Unique(50*time.Second),
		asynq.MaxRetry(2),
		asynq.Timeout(d.RunTimeout),
	)
	return err
}

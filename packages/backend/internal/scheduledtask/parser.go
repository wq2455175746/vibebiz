package scheduledtask

import (
	"context"
	"strings"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/sandboxregistry"
)

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

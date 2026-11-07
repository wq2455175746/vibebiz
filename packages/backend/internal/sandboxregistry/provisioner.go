package sandboxregistry

import (
	"context"
	"fmt"
)

type Initializer interface {
	InitProject(ctx context.Context, baseURL, projectID string) error
}

type Provisioner struct {
	selector    *Selector
	initializer Initializer
	attempts    int
}

func NewProvisioner(selector *Selector, initializer Initializer, attempts int) *Provisioner {
	if attempts < 1 {
		attempts = 1
	}
	return &Provisioner{selector: selector, initializer: initializer, attempts: attempts}
}

func (p *Provisioner) InitProject(ctx context.Context, projectID string) (int64, error) {
	exclude := make([]int64, 0, p.attempts)
	var lastErr error
	for len(exclude) < p.attempts {
		instance, err := p.selector.SelectExcluding(ctx, exclude)
		if err != nil {
			if lastErr != nil {
				return 0, fmt.Errorf("init project on sandbox: %w", lastErr)
			}
			return 0, err
		}
		if err := p.initializer.InitProject(ctx, instance.BaseURL(), projectID); err != nil {
			lastErr = err
			exclude = append(exclude, instance.ID)
			continue
		}
		return instance.ID, nil
	}
	if lastErr != nil {
		return 0, fmt.Errorf("init project on sandbox: %w", lastErr)
	}
	return 0, ErrNoHealthySandbox
}

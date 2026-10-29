package sandboxregistry

import (
	"context"
	"fmt"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
)

type Initializer interface {
	InitProject(ctx context.Context, baseURL, projectID string) error
}

type HTTPInitializer struct {
	obs *observability.Provider
}

func NewHTTPInitializer(obs *observability.Provider) *HTTPInitializer {
	return &HTTPInitializer{obs: obs}
}

func (h *HTTPInitializer) InitProject(ctx context.Context, baseURL, projectID string) error {
	return client.NewSandbox(baseURL, h.obs).InitProject(ctx, projectID)
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

package sandboxregistry

import (
	"context"
	"errors"
	"time"
)

type Resolver struct {
	repo     Repository
	selector *Selector
	ttl      time.Duration
	now      func() time.Time
}

func NewResolver(repo Repository, selector *Selector, ttl time.Duration, now func() time.Time) *Resolver {
	if now == nil {
		now = time.Now
	}
	return &Resolver{repo: repo, selector: selector, ttl: ttl, now: now}
}

func (r *Resolver) ResolveBaseURL(ctx context.Context, projectID string) (string, error) {
	for attempt := 0; attempt < 2; attempt++ {
		binding, err := r.repo.GetProjectBinding(ctx, projectID)
		if err != nil {
			return "", err
		}
		now := r.now().UTC()
		if binding.Instance != nil && binding.Instance.Usable(now, r.ttl) {
			return binding.Instance.BaseURL(), nil
		}
		instance, err := r.selector.Select(ctx)
		if err != nil {
			return "", err
		}
		err = r.repo.UpdateProjectBinding(ctx, binding.ProjectInternalID, binding.SandboxID, instance.ID)
		if errors.Is(err, ErrBindingConflict) {
			continue
		}
		if err != nil {
			return "", err
		}
		return instance.BaseURL(), nil
	}
	return "", ErrBindingConflict
}

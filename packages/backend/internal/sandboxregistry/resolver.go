package sandboxregistry

import (
	"context"
	"errors"
	"time"
)

type Resolver struct {
	repo         Repository
	selector     *Selector
	heartbeatTTL time.Duration
	leaseTTL     time.Duration
	now          func() time.Time
}

func NewResolver(repo Repository, selector *Selector, heartbeatTTL, leaseTTL time.Duration, now func() time.Time) *Resolver {
	if now == nil {
		now = time.Now
	}
	return &Resolver{repo: repo, selector: selector, heartbeatTTL: heartbeatTTL, leaseTTL: leaseTTL, now: now}
}

func (r *Resolver) ResolveRoute(ctx context.Context, projectID string) (Route, error) {
	for attempt := 0; attempt < 2; attempt++ {
		binding, err := r.repo.GetProjectBinding(ctx, projectID)
		if err != nil {
			return Route{}, err
		}
		now := r.now().UTC()
		if binding.SandboxID != nil && binding.LeaseUntil.After(now) && binding.Instance != nil && binding.Instance.Usable(now, r.heartbeatTTL) {
			if binding.LeaseUntil.Before(now.Add(r.leaseTTL / 2)) {
				_ = r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, binding.Epoch, now, r.leaseTTL)
			}
			return Route{BaseURL: binding.Instance.BaseURL(), Epoch: binding.Epoch}, nil
		}
		instance, err := r.selector.Select(ctx)
		if err != nil {
			return Route{}, err
		}
		lease, err := r.repo.AcquireProjectLease(ctx, binding.ProjectInternalID, binding.SandboxID, instance.ID, now, r.leaseTTL)
		if errors.Is(err, ErrBindingConflict) {
			continue
		}
		if err != nil {
			return Route{}, err
		}
		return Route{BaseURL: instance.BaseURL(), Epoch: lease.Epoch}, nil
	}
	return Route{}, ErrBindingConflict
}

func (r *Resolver) ResolveBaseURL(ctx context.Context, projectID string) (string, error) {
	route, err := r.ResolveRoute(ctx, projectID)
	return route.BaseURL, err
}

func (r *Resolver) Renew(ctx context.Context, projectID string, epoch int64) error {
	binding, err := r.repo.GetProjectBinding(ctx, projectID)
	if err != nil {
		return err
	}
	if binding.SandboxID == nil {
		return ErrBindingConflict
	}
	return r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, epoch, r.now().UTC(), r.leaseTTL)
}

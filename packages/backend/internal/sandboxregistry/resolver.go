package sandboxregistry

import (
	"context"
	"errors"
	"time"
)

type LeaseMetrics interface {
	RecordProjectLeaseAcquire(result string)
	RecordProjectLeaseRenewFail()
}

type Resolver struct {
	repo         Repository
	selector     *Selector
	heartbeatTTL time.Duration
	leaseTTL     time.Duration
	now          func() time.Time
	metrics      LeaseMetrics
}

func NewResolver(repo Repository, selector *Selector, heartbeatTTL, leaseTTL time.Duration, now func() time.Time) *Resolver {
	if now == nil {
		now = time.Now
	}
	return &Resolver{repo: repo, selector: selector, heartbeatTTL: heartbeatTTL, leaseTTL: leaseTTL, now: now}
}

func (r *Resolver) SetMetrics(metrics LeaseMetrics) {
	r.metrics = metrics
}

func (r *Resolver) recordAcquire(result string) {
	if r.metrics != nil {
		r.metrics.RecordProjectLeaseAcquire(result)
	}
}

func (r *Resolver) recordRenewFail() {
	if r.metrics != nil {
		r.metrics.RecordProjectLeaseRenewFail()
	}
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
				if err := r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, binding.Epoch, now, r.leaseTTL); err != nil {
					r.recordRenewFail()
					continue
				}
			}
			return Route{BaseURL: binding.Instance.BaseURL(), Epoch: binding.Epoch, SandboxID: *binding.SandboxID}, nil
		}
		instance, err := r.selector.Select(ctx)
		if err != nil {
			return Route{}, err
		}
		lease, err := r.repo.AcquireProjectLease(ctx, binding.ProjectInternalID, binding.SandboxID, instance.ID, now, r.leaseTTL)
		if errors.Is(err, ErrBindingConflict) {
			r.recordAcquire("conflict")
			continue
		}
		if err != nil {
			r.recordAcquire("error")
			return Route{}, err
		}
		r.recordAcquire("ok")
		return Route{BaseURL: instance.BaseURL(), Epoch: lease.Epoch, SandboxID: instance.ID}, nil
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
	if err := r.repo.RenewProjectLease(ctx, binding.ProjectInternalID, *binding.SandboxID, epoch, r.now().UTC(), r.leaseTTL); err != nil {
		r.recordRenewFail()
		return err
	}
	return nil
}

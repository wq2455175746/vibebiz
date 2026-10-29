package sandboxregistry

import (
	"context"
	"time"
)

type Selector struct {
	repo Repository
	ttl  time.Duration
	now  func() time.Time
}

func NewSelector(repo Repository, ttl time.Duration, now func() time.Time) *Selector {
	if now == nil {
		now = time.Now
	}
	return &Selector{repo: repo, ttl: ttl, now: now}
}

func (s *Selector) Select(ctx context.Context) (Instance, error) {
	return s.SelectExcluding(ctx, nil)
}

func (s *Selector) SelectExcluding(ctx context.Context, exclude []int64) (Instance, error) {
	return s.repo.Select(ctx, s.now().UTC(), s.ttl, exclude)
}

package client

import (
	"context"
	"io"
	"sync"

	"vibebiz/internal/observability"
)

type BaseURLSelector interface {
	Select(ctx context.Context) (string, error)
}

type BaseURLSelectorFunc func(ctx context.Context) (string, error)

func (f BaseURLSelectorFunc) Select(ctx context.Context) (string, error) {
	return f(ctx)
}

type SkillRouter struct {
	selector  BaseURLSelector
	obs       *observability.Provider
	clients   sync.Map
	newClient func(baseURL string, obs *observability.Provider) *Sandbox
}

func NewSkillRouter(selector BaseURLSelector, obs *observability.Provider) *SkillRouter {
	return &SkillRouter{selector: selector, obs: obs, newClient: NewSandbox}
}

func (r *SkillRouter) sandbox(ctx context.Context) (*Sandbox, error) {
	baseURL, err := r.selector.Select(ctx)
	if err != nil {
		return nil, err
	}
	if value, ok := r.clients.Load(baseURL); ok {
		return value.(*Sandbox), nil
	}
	created := r.newClient(baseURL, r.obs)
	actual, _ := r.clients.LoadOrStore(baseURL, created)
	return actual.(*Sandbox), nil
}

func (r *SkillRouter) ListStoredSkills(ctx context.Context, scope string, userID int64) ([]SkillInfo, error) {
	sandbox, err := r.sandbox(ctx)
	if err != nil {
		return nil, err
	}
	return sandbox.ListStoredSkills(ctx, scope, userID)
}

func (r *SkillRouter) UploadSkill(ctx context.Context, scope string, userID int64, filename string, source io.Reader, size int64, overwrite bool) (SkillInfo, error) {
	sandbox, err := r.sandbox(ctx)
	if err != nil {
		return SkillInfo{}, err
	}
	return sandbox.UploadSkill(ctx, scope, userID, filename, source, size, overwrite)
}

func (r *SkillRouter) DeleteSkill(ctx context.Context, scope string, userID int64, name string) error {
	sandbox, err := r.sandbox(ctx)
	if err != nil {
		return err
	}
	return sandbox.DeleteSkill(ctx, scope, userID, name)
}

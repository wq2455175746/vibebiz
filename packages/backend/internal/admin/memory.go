package admin

import (
	"context"
	"time"
)

type MemoryRepository struct {
	users     []User
	sandboxes []Sandbox
	quotas    []Quota
	audit     []AuditEvent
}

func NewMemoryRepository() Repository {
	created := time.Date(2026, 10, 28, 0, 0, 0, 0, time.UTC)
	return &MemoryRepository{
		users: []User{
			{ID: "1", Name: "wanghg11", Email: "2455175746@qq.com", Plan: "free", Status: "active", CreatedAt: created},
			{ID: "2", Name: "vistor", Email: "vistor_vibebiz@qq.com", Plan: "free", Status: "active", CreatedAt: created},
		},
		sandboxes: []Sandbox{
			{ID: "sbx_01", ProjectID: "p_01", Node: "local-1", State: "running", StartedAt: created},
			{ID: "sbx_02", ProjectID: "p_02", Node: "local-1", State: "stopped", StartedAt: created},
		},
		quotas: []Quota{
			{UserID: "1", Plan: "free", LLMTokens: 12000, SandboxMinutes: 45, StorageMB: 12},
			{UserID: "2", Plan: "free", LLMTokens: 12000, SandboxMinutes: 45, StorageMB: 12},
		},
		audit: []AuditEvent{
			{ID: "evt_01", Actor: "wanghg11", Action: "project.create", Target: "p_01", CreatedAt: created},
			{ID: "evt_02", Actor: "vistor", Action: "file.write", Target: "p_02", CreatedAt: created},
			{ID: "evt_03", Actor: "wanghg11", Action: "sandbox.start", Target: "sbx_01", CreatedAt: created},
		},
	}
}

func (r *MemoryRepository) Overview(_ context.Context, projectCount int) (Overview, error) {
	return Overview{
		Projects:         projectCount,
		Users:            2,
		RunningSandboxes: 1,
		TodayRequests:    128,
	}, nil
}

func (r *MemoryRepository) Users(_ context.Context) ([]User, error) {
	return clone(r.users), nil
}

func (r *MemoryRepository) Sandboxes(_ context.Context) ([]Sandbox, error) {
	return clone(r.sandboxes), nil
}

func (r *MemoryRepository) Quotas(_ context.Context) ([]Quota, error) {
	return clone(r.quotas), nil
}

func (r *MemoryRepository) Audit(_ context.Context) ([]AuditEvent, error) {
	return clone(r.audit), nil
}

func (r *MemoryRepository) RecordAudit(_ context.Context, event AuditRecord) error {
	r.audit = append([]AuditEvent{{
		ID:        "evt_memory",
		Actor:     event.ActorName,
		Action:    event.Action,
		Target:    event.ResourceID,
		CreatedAt: time.Now().UTC(),
	}}, r.audit...)
	return nil
}

func clone[T any](items []T) []T {
	out := make([]T, len(items))
	copy(out, items)
	return out
}

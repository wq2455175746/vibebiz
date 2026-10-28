package admin

import "time"

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
			{ID: "u_01", Name: "wangh11", Email: "wangh11@example.com", Plan: "free", Status: "active", CreatedAt: created},
			{ID: "u_02", Name: "liuy22", Email: "liuy22@example.com", Plan: "pro", Status: "active", CreatedAt: created},
		},
		sandboxes: []Sandbox{
			{ID: "sbx_01", ProjectID: "p_01", Node: "local-1", State: "running", StartedAt: created},
			{ID: "sbx_02", ProjectID: "p_02", Node: "local-1", State: "stopped", StartedAt: created},
		},
		quotas: []Quota{
			{UserID: "u_01", Plan: "free", LLMTokens: 12000, SandboxMinutes: 45, StorageMB: 12},
			{UserID: "u_02", Plan: "pro", LLMTokens: 120000, SandboxMinutes: 480, StorageMB: 256},
		},
		audit: []AuditEvent{
			{ID: "evt_01", Actor: "wangh11", Action: "project.create", Target: "p_01", CreatedAt: created},
			{ID: "evt_02", Actor: "liuy22", Action: "file.write", Target: "p_02", CreatedAt: created},
			{ID: "evt_03", Actor: "wangh11", Action: "sandbox.start", Target: "sbx_01", CreatedAt: created},
		},
	}
}

func (r *MemoryRepository) Overview(projectCount int) Overview {
	return Overview{
		Projects:         projectCount,
		Users:            2,
		RunningSandboxes: 1,
		TodayRequests:    128,
	}
}

func (r *MemoryRepository) Users() []User {
	return clone(r.users)
}

func (r *MemoryRepository) Sandboxes() []Sandbox {
	return clone(r.sandboxes)
}

func (r *MemoryRepository) Quotas() []Quota {
	return clone(r.quotas)
}

func (r *MemoryRepository) Audit() []AuditEvent {
	return clone(r.audit)
}

func clone[T any](items []T) []T {
	out := make([]T, len(items))
	copy(out, items)
	return out
}

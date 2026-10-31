package admin

import (
	"context"
	"strconv"
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
			{ID: "1", Name: "wanghg11", Email: "wanghg11@example.com", Plan: "free", Status: "active", CreatedAt: created},
			{ID: "2", Name: "vistor", Email: "vistor@example.com", Plan: "free", Status: "active", CreatedAt: created},
		},
		sandboxes: []Sandbox{
			{ID: "sb-1", Name: "sandbox-a", Host: "127.0.0.1", Port: 8000, Status: "online", HealthStatus: "healthy", ActiveRequests: 2, MaxConcurrency: 8, LastHeartbeatAt: &created, LastHealthAt: &created},
			{ID: "sb-2", Name: "sandbox-b", Host: "127.0.0.1", Port: 8001, Status: "offline", HealthStatus: "unhealthy", ActiveRequests: 0, MaxConcurrency: 8, LastHeartbeatAt: &created, LastHealthAt: &created, HealthError: "timeout"},
		},
		quotas: []Quota{
			{UserID: "1", Plan: "free", LLMTokensQuota: 500000, LLMTokensUsed: 12000, SandboxMinutesQuota: 600, SandboxMinutesUsed: 45, StorageQuotaMB: 1024, StorageUsedMB: 12},
			{UserID: "2", Plan: "free", LLMTokensQuota: 500000, LLMTokensUsed: 12000, SandboxMinutesQuota: 600, SandboxMinutesUsed: 45, StorageQuotaMB: 1024, StorageUsedMB: 12},
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

func (r *MemoryRepository) Users(_ context.Context, query ListQuery) (Page[User], error) {
	filtered := make([]User, 0, len(r.users))
	for _, item := range r.users {
		if MatchesKeyword(query.Keyword, item.Name, item.Email) {
			filtered = append(filtered, item)
		}
	}
	return Paginate(filtered, query), nil
}

func (r *MemoryRepository) UpdateUserStatus(_ context.Context, id int64, status int8) error {
	target := strconv.FormatInt(id, 10)
	for index := range r.users {
		if r.users[index].ID != target {
			continue
		}
		if status == 1 {
			r.users[index].Status = "active"
		} else {
			r.users[index].Status = "disabled"
		}
		return nil
	}
	return ErrNotFound
}

func (r *MemoryRepository) Sandboxes(_ context.Context, query ListQuery) (Page[Sandbox], error) {
	filtered := make([]Sandbox, 0, len(r.sandboxes))
	for _, item := range r.sandboxes {
		if MatchesKeyword(query.Keyword, item.Name, item.Host, item.Status) {
			filtered = append(filtered, item)
		}
	}
	return Paginate(filtered, query), nil
}

func (r *MemoryRepository) Quotas(_ context.Context) ([]Quota, error) {
	return clone(r.quotas), nil
}

func (r *MemoryRepository) UpdateQuota(_ context.Context, userID int64, input QuotaUpdateInput) error {
	target := strconv.FormatInt(userID, 10)
	for index := range r.quotas {
		if r.quotas[index].UserID != target {
			continue
		}
		r.quotas[index].LLMTokensQuota = input.LLMTokensQuota
		r.quotas[index].SandboxMinutesQuota = input.SandboxMinutesQuota
		r.quotas[index].StorageQuotaMB = input.StorageQuotaMB
		return nil
	}
	return ErrNotFound
}

func (r *MemoryRepository) Audit(_ context.Context, query ListQuery) (Page[AuditEvent], error) {
	filtered := make([]AuditEvent, 0, len(r.audit))
	for _, item := range r.audit {
		if MatchesKeyword(query.Keyword, item.Actor, item.Action, item.Target) {
			filtered = append(filtered, item)
		}
	}
	return Paginate(filtered, query), nil
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

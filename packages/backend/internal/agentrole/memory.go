package agentrole

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type MemoryRepository struct {
	mu    sync.Mutex
	next  int64
	items []Role
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{}
}

func (r *MemoryRepository) List(context.Context) ([]Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Role, len(r.items))
	copy(out, r.items)
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (r *MemoryRepository) ListPage(ctx context.Context, query ListQuery) (Page, error) {
	roles, err := r.List(ctx)
	if err != nil {
		return Page{}, err
	}
	query = normalizeListQuery(query)
	keyword := strings.ToLower(query.Keyword)
	filtered := make([]Role, 0, len(roles))
	for _, role := range roles {
		if keyword != "" &&
			!strings.Contains(strings.ToLower(role.RoleCode), keyword) &&
			!strings.Contains(strings.ToLower(role.RoleNameEN), keyword) &&
			!strings.Contains(strings.ToLower(role.RoleNameZH), keyword) {
			continue
		}
		filtered = append(filtered, role)
	}
	total := int64(len(filtered))
	start := (query.Page - 1) * query.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + query.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	items := make([]Role, end-start)
	copy(items, filtered[start:end])
	return Page{Items: items, Total: total, Page: query.Page, PageSize: query.PageSize}, nil
}

func (r *MemoryRepository) Create(_ context.Context, input CreateInput, createdBy string) (Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.items {
		if item.RoleCode == input.RoleCode {
			return Role{}, ErrConflict
		}
	}
	r.next++
	now := time.Now().UTC()
	item := Role{
		ID: r.next, RoleCode: input.RoleCode, RoleNameEN: input.RoleNameEN,
		RoleNameZH: input.RoleNameZH, Description: input.Description,
		SystemPrompt: input.SystemPrompt, AvatarURL: input.AvatarKey,
		IsBuiltin: false, Status: input.Status, SortOrder: input.SortOrder,
		Version: 1, CreatedBy: createdBy, CreatedAt: now, UpdatedAt: now,
	}
	r.items = append(r.items, item)
	return item, nil
}

func (r *MemoryRepository) Update(_ context.Context, id int64, input UpdateInput, _ string) (Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for index := range r.items {
		if r.items[index].ID != id {
			continue
		}
		r.items[index].Description = input.Description
		r.items[index].SystemPrompt = input.SystemPrompt
		r.items[index].Status = input.Status
		r.items[index].Version++
		r.items[index].UpdatedAt = time.Now().UTC()
		return r.items[index], nil
	}
	return Role{}, ErrNotFound
}

func (r *MemoryRepository) ListActive(ctx context.Context) ([]Role, error) {
	roles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	active := make([]Role, 0, len(roles))
	for _, role := range roles {
		if role.Status == 1 {
			active = append(active, role)
		}
	}
	return active, nil
}

func (r *MemoryRepository) GetActive(_ context.Context, id int64) (Role, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, role := range r.items {
		if role.ID == id && role.Status == 1 {
			return role, nil
		}
	}
	return Role{}, ErrNotFound
}

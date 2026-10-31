package store

import (
	"context"
	"errors"
	"sort"
	"sync"

	"vibebiz/internal/model"
)

var ErrNotFound = errors.New("project not found")

type ProjectStore interface {
	Create(ctx context.Context, project *model.Project) error
	Get(ctx context.Context, id string) (*model.Project, error)
	List(ctx context.Context) []*model.Project
	ListByWorkspaces(ctx context.Context, workspaceIDs []int64) []*model.Project
	Update(ctx context.Context, project *model.Project) error
	Delete(ctx context.Context, id string) error
	SetAgentRoles(ctx context.Context, id string, roleIDs []int64) error
	SetCover(ctx context.Context, id, key string) error
	SetPreviewPath(ctx context.Context, id, previewPath string) error
}

type MemoryProjectStore struct {
	mu    sync.RWMutex
	items map[string]*model.Project
}

func NewMemoryProjectStore() *MemoryProjectStore {
	return &MemoryProjectStore{items: make(map[string]*model.Project)}
}

func (s *MemoryProjectStore) Create(_ context.Context, project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[project.ID]; ok {
		return errors.New("project already exists")
	}
	s.items[project.ID] = clone(project)
	return nil
}

func (s *MemoryProjectStore) Get(_ context.Context, id string) (*model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	project, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(project), nil
}

func (s *MemoryProjectStore) List(_ context.Context) []*model.Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Project, 0, len(s.items))
	for _, project := range s.items {
		out = append(out, clone(project))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *MemoryProjectStore) ListByWorkspaces(_ context.Context, workspaceIDs []int64) []*model.Project {
	if len(workspaceIDs) == 0 {
		return []*model.Project{}
	}
	allowed := make(map[int64]struct{}, len(workspaceIDs))
	for _, id := range workspaceIDs {
		allowed[id] = struct{}{}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*model.Project, 0, len(s.items))
	for _, project := range s.items {
		if _, ok := allowed[project.WorkspaceID]; !ok {
			continue
		}
		out = append(out, clone(project))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out
}

func (s *MemoryProjectStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[id]; !ok {
		return ErrNotFound
	}
	delete(s.items, id)
	return nil
}

func (s *MemoryProjectStore) Update(_ context.Context, project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[project.ID]; !ok {
		return ErrNotFound
	}
	s.items[project.ID] = clone(project)
	return nil
}

func (s *MemoryProjectStore) SetAgentRoles(_ context.Context, id string, roleIDs []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	project.AgentRoleIDs = append([]int64(nil), roleIDs...)
	if len(roleIDs) == 0 {
		project.DefaultAgentRoleID = nil
	} else {
		first := roleIDs[0]
		project.DefaultAgentRoleID = &first
	}
	return nil
}

func (s *MemoryProjectStore) SetCover(_ context.Context, id, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	project.CoverS3Path = key
	project.CoverURL = ""
	return nil
}

func (s *MemoryProjectStore) SetPreviewPath(_ context.Context, id, previewPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	project, ok := s.items[id]
	if !ok {
		return ErrNotFound
	}
	project.PreviewPath = previewPath
	return nil
}

func clone(project *model.Project) *model.Project {
	copied := *project
	copied.AgentRoleIDs = append([]int64(nil), project.AgentRoleIDs...)
	return &copied
}

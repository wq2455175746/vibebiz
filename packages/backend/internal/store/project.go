package store

import (
	"errors"
	"sort"
	"sync"

	"vibebiz/internal/model"
)

var ErrNotFound = errors.New("project not found")

type ProjectStore interface {
	Create(project *model.Project) error
	Get(id string) (*model.Project, error)
	List() []*model.Project
	Update(project *model.Project) error
}

type MemoryProjectStore struct {
	mu    sync.RWMutex
	items map[string]*model.Project
}

func NewMemoryProjectStore() *MemoryProjectStore {
	return &MemoryProjectStore{items: make(map[string]*model.Project)}
}

func (s *MemoryProjectStore) Create(project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[project.ID]; ok {
		return errors.New("project already exists")
	}
	s.items[project.ID] = clone(project)
	return nil
}

func (s *MemoryProjectStore) Get(id string) (*model.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	project, ok := s.items[id]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(project), nil
}

func (s *MemoryProjectStore) List() []*model.Project {
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

func (s *MemoryProjectStore) Update(project *model.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[project.ID]; !ok {
		return ErrNotFound
	}
	s.items[project.ID] = clone(project)
	return nil
}

func clone(project *model.Project) *model.Project {
	copied := *project
	return &copied
}

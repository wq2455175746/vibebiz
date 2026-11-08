package evolution

import (
	"context"
	"sort"
	"sync"

	"vibebiz/internal/dao/model"
)

type MemoryStore struct {
	mu        sync.Mutex
	versions  []model.EvolutionVersion
	proposals []model.EvolutionProposal
	messages  []model.EvolutionMessage
	tasks     []model.EvolutionTask
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

func (s *MemoryStore) ListVersions(_ context.Context) ([]model.EvolutionVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]model.EvolutionVersion(nil), s.versions...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetVersion(_ context.Context, publicID string) (*model.EvolutionVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.versions {
		if s.versions[i].PublicID == publicID {
			item := s.versions[i]
			return &item, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) CreateVersion(_ context.Context, version *model.EvolutionVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	version.ID = int64(len(s.versions) + 1)
	s.versions = append(s.versions, *version)
	return nil
}

func (s *MemoryStore) UpdateVersionStatus(_ context.Context, publicID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.versions {
		if s.versions[i].PublicID == publicID {
			s.versions[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

func (s *MemoryStore) ListProposals(_ context.Context, userID string) ([]model.EvolutionProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]model.EvolutionProposal, 0)
	for _, item := range s.proposals {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	sortProposals(items)
	return items, nil
}

func (s *MemoryStore) ListAllProposals(_ context.Context) ([]model.EvolutionProposal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]model.EvolutionProposal(nil), s.proposals...)
	sortProposals(items)
	return items, nil
}

func (s *MemoryStore) CreateProposal(_ context.Context, proposal *model.EvolutionProposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	proposal.ID = int64(len(s.proposals) + 1)
	s.proposals = append(s.proposals, *proposal)
	return nil
}

func (s *MemoryStore) AcceptProposalsByVersion(_ context.Context, versionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.proposals {
		if s.proposals[i].VersionPublicID == versionID {
			s.proposals[i].Status = ProposalAccepted
		}
	}
	return nil
}

func (s *MemoryStore) ListMessages(_ context.Context, userID string) ([]model.EvolutionMessage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]model.EvolutionMessage, 0)
	for _, item := range s.messages {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].CreatedAt.Before(items[j].CreatedAt)
	})
	return items, nil
}

func (s *MemoryStore) CreateMessage(_ context.Context, message *model.EvolutionMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	message.ID = int64(len(s.messages) + 1)
	s.messages = append(s.messages, *message)
	return nil
}

func (s *MemoryStore) ListTasks(_ context.Context) ([]model.EvolutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := append([]model.EvolutionTask(nil), s.tasks...)
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
	return items, nil
}

func (s *MemoryStore) GetTask(_ context.Context, publicID string) (*model.EvolutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].PublicID == publicID {
			item := s.tasks[i]
			return &item, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) GetTaskByVersion(_ context.Context, versionID string) (*model.EvolutionTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].VersionPublicID == versionID {
			item := s.tasks[i]
			return &item, nil
		}
	}
	return nil, ErrNotFound
}

func (s *MemoryStore) CreateTask(_ context.Context, task *model.EvolutionTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	task.ID = int64(len(s.tasks) + 1)
	s.tasks = append(s.tasks, *task)
	return nil
}

func (s *MemoryStore) UpdateTaskStatus(_ context.Context, publicID, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tasks {
		if s.tasks[i].PublicID == publicID {
			s.tasks[i].Status = status
			return nil
		}
	}
	return ErrNotFound
}

func sortProposals(items []model.EvolutionProposal) {
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].CreatedAt.Equal(items[j].CreatedAt) {
			return items[i].ID > items[j].ID
		}
		return items[i].CreatedAt.After(items[j].CreatedAt)
	})
}

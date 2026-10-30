package storagesync

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"vibebiz/internal/client"
	"vibebiz/internal/model"
)

const defaultTimeout = 5 * time.Minute

var ErrInvalidProject = errors.New("invalid project")

type Record struct {
	ProjectID      int64
	WorkspaceID    int64
	UserID         int64
	StoragePath    string
	FileSizeBytes  int64
	FileCount      int64
	DirCount       int64
	LastModifiedAt *time.Time
	LastScannedAt  time.Time
}

type Repository interface {
	Save(ctx context.Context, record Record) error
	ListByWorkspace(ctx context.Context, workspaceID int64) ([]Record, error)
}

type ProjectStore interface {
	Get(ctx context.Context, id string) (*model.Project, error)
}

type StatsClient interface {
	ProjectStats(ctx context.Context, id string) (client.ProjectStats, error)
}

type Syncer struct {
	Projects ProjectStore
	Sandbox  StatsClient
	Repo     Repository
	Logger   *slog.Logger
	Now      func() time.Time

	mu      sync.Mutex
	running map[string]struct{}
}

func (s *Syncer) Enqueue(projectID string) {
	if projectID == "" {
		return
	}
	if !s.reserve(projectID) {
		return
	}
	go func() {
		defer s.release(projectID)
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		defer cancel()
		if err := s.Sync(ctx, projectID); err != nil && s.Logger != nil {
			s.Logger.Warn("storage.sync", "project_id", projectID, "status", "error", "error", err)
		}
	}()
}

func (s *Syncer) Sync(ctx context.Context, projectID string) error {
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil {
		return err
	}
	if project.InternalID <= 0 || project.WorkspaceID <= 0 || project.OwnerUserID <= 0 {
		return ErrInvalidProject
	}
	stats, err := s.Sandbox.ProjectStats(ctx, projectID)
	if err != nil {
		return err
	}
	record := Record{
		ProjectID:     project.InternalID,
		WorkspaceID:   project.WorkspaceID,
		UserID:        project.OwnerUserID,
		FileSizeBytes: stats.FileSizeBytes,
		FileCount:     stats.FileCount,
		DirCount:      stats.DirCount,
		LastScannedAt: s.now(),
	}
	if stats.LastModifiedAt > 0 {
		modified := time.UnixMilli(stats.LastModifiedAt).UTC()
		record.LastModifiedAt = &modified
	}
	return s.Repo.Save(ctx, record)
}

func (s *Syncer) reserve(projectID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running == nil {
		s.running = make(map[string]struct{})
	}
	if _, ok := s.running[projectID]; ok {
		return false
	}
	s.running[projectID] = struct{}{}
	return true
}

func (s *Syncer) release(projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.running, projectID)
}

func (s *Syncer) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

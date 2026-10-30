package storagesync

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type fakeStats struct {
	stats client.ProjectStats
	err   error
}

func (f *fakeStats) ProjectStats(context.Context, string) (client.ProjectStats, error) {
	return f.stats, f.err
}

type fakeRepo struct {
	saved []Record
	err   error
}

func (f *fakeRepo) Save(_ context.Context, record Record) error {
	f.saved = append(f.saved, record)
	return f.err
}

func (f *fakeRepo) ListByWorkspace(context.Context, int64) ([]Record, error) {
	return nil, nil
}

func TestSyncerSavesProjectStorage(t *testing.T) {
	now := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	projects := store.NewMemoryProjectStore()
	projects.Create(context.Background(), &model.Project{InternalID: 10, ID: "p1", WorkspaceID: 5, OwnerUserID: 7})
	repo := &fakeRepo{}
	syncer := &Syncer{
		Projects: projects,
		Sandbox:  &fakeStats{stats: client.ProjectStats{FileSizeBytes: 123, FileCount: 4, DirCount: 2, LastModifiedAt: 1793191707598}},
		Repo:     repo,
		Now:      func() time.Time { return now },
	}

	if err := syncer.Sync(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if len(repo.saved) != 1 {
		t.Fatalf("saved = %+v", repo.saved)
	}
	record := repo.saved[0]
	if record.ProjectID != 10 || record.WorkspaceID != 5 || record.UserID != 7 {
		t.Fatalf("record = %+v", record)
	}
	if record.FileSizeBytes != 123 || record.FileCount != 4 || record.DirCount != 2 {
		t.Fatalf("record = %+v", record)
	}
	if record.LastScannedAt != now || record.LastModifiedAt == nil || record.LastModifiedAt.UnixMilli() != 1793191707598 {
		t.Fatalf("record times = %+v", record)
	}
}

func TestSyncerSkipsWhenStatsFail(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	projects.Create(context.Background(), &model.Project{InternalID: 10, ID: "p1", WorkspaceID: 5, OwnerUserID: 7})
	repo := &fakeRepo{}
	syncer := &Syncer{
		Projects: projects,
		Sandbox:  &fakeStats{err: errors.New("stats failed")},
		Repo:     repo,
	}

	if err := syncer.Sync(context.Background(), "p1"); err == nil {
		t.Fatal("expected error")
	}
	if len(repo.saved) != 0 {
		t.Fatalf("saved = %+v", repo.saved)
	}
}

func TestSyncerRejectsProjectWithoutInternalID(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	projects.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 5, OwnerUserID: 7})
	syncer := &Syncer{Projects: projects, Sandbox: &fakeStats{}, Repo: &fakeRepo{}}

	if err := syncer.Sync(context.Background(), "p1"); !errors.Is(err, ErrInvalidProject) {
		t.Fatalf("err = %v", err)
	}
}

package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/model"
)

func TestMemoryProjectStoreCreateAndGet(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", Prompt: "make a game", State: model.ProjectStateReady, CreatedAt: time.Now()}

	if err := s.Create(context.Background(), project); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "demo" {
		t.Fatalf("name = %q, want demo", got.Name)
	}
}

func TestMemoryProjectStoreDuplicate(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1"}
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(context.Background(), project); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestMemoryProjectStoreNotFound(t *testing.T) {
	s := NewMemoryProjectStore()
	if _, err := s.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryProjectStoreReturnsCopy(t *testing.T) {
	s := NewMemoryProjectStore()
	s.Create(context.Background(), &model.Project{ID: "p1", Name: "original"})

	got, _ := s.Get(context.Background(), "p1")
	got.Name = "mutated"

	again, _ := s.Get(context.Background(), "p1")
	if again.Name != "original" {
		t.Fatalf("store mutated through returned pointer: %q", again.Name)
	}
}

func TestMemoryProjectStoreListSortedNewestFirst(t *testing.T) {
	s := NewMemoryProjectStore()
	now := time.Now()
	s.Create(context.Background(), &model.Project{ID: "old", CreatedAt: now.Add(-time.Hour)})
	s.Create(context.Background(), &model.Project{ID: "new", CreatedAt: now})

	list := s.List(context.Background())
	if len(list) != 2 || list[0].ID != "new" {
		t.Fatalf("unexpected order: %+v", list)
	}
}

func TestMemoryProjectStoreListByWorkspaces(t *testing.T) {
	s := NewMemoryProjectStore()
	now := time.Now()
	s.Create(context.Background(), &model.Project{ID: "w1-old", WorkspaceID: 1, OwnerUserID: 7, CreatedAt: now.Add(-time.Hour)})
	s.Create(context.Background(), &model.Project{ID: "w1-new", WorkspaceID: 1, OwnerUserID: 8, CreatedAt: now})
	s.Create(context.Background(), &model.Project{ID: "w2", WorkspaceID: 2, OwnerUserID: 9, CreatedAt: now.Add(-2 * time.Hour)})
	s.Create(context.Background(), &model.Project{ID: "legacy", CreatedAt: now})

	list := s.ListByWorkspaces(context.Background(), []int64{1})
	if len(list) != 2 || list[0].ID != "w1-new" || list[1].ID != "w1-old" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if list[0].WorkspaceID != 1 || list[0].OwnerUserID != 8 {
		t.Fatalf("ownership lost: %+v", list[0])
	}

	list = s.ListByWorkspaces(context.Background(), []int64{2, 1})
	if len(list) != 3 || list[0].ID != "w1-new" {
		t.Fatalf("multi workspace list: %+v", list)
	}

	if list := s.ListByWorkspaces(context.Background(), nil); len(list) != 0 {
		t.Fatalf("empty workspace ids returned %+v", list)
	}
}

func TestMemoryProjectStoreSetAndGetAgentRoles(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", WorkspaceID: 1}
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentRoles(context.Background(), "p1", []int64{2, 4}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.AgentRoleIDs) != 2 || got.AgentRoleIDs[0] != 2 || got.AgentRoleIDs[1] != 4 {
		t.Fatalf("role ids = %v", got.AgentRoleIDs)
	}
	if got.DefaultAgentRoleID == nil || *got.DefaultAgentRoleID != 2 {
		t.Fatalf("default role = %v", got.DefaultAgentRoleID)
	}
}

func TestMemoryProjectStoreClearAgentRoles(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", WorkspaceID: 1, AgentRoleIDs: []int64{2}}
	if err := s.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAgentRoles(context.Background(), "p1", nil); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(context.Background(), "p1")
	if len(got.AgentRoleIDs) != 0 || got.DefaultAgentRoleID != nil {
		t.Fatalf("roles not cleared: %+v", got)
	}
}

func TestMemoryProjectStoreSetCover(t *testing.T) {
	s := NewMemoryProjectStore()
	if err := s.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 1}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCover(context.Background(), "p1", "projects/p1/cover.png"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got.CoverS3Path != "projects/p1/cover.png" {
		t.Fatalf("cover = %q", got.CoverS3Path)
	}
}

func TestMemoryProjectStoreDelete(t *testing.T) {
	s := NewMemoryProjectStore()
	s.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 1})

	if err := s.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get(context.Background(), "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete err = %v, want ErrNotFound", err)
	}
	if err := s.Delete(context.Background(), "p1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v, want ErrNotFound", err)
	}
}

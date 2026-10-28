package store

import (
	"errors"
	"testing"
	"time"

	"vibebiz/internal/model"
)

func TestMemoryProjectStoreCreateAndGet(t *testing.T) {
	s := NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", Prompt: "make a game", State: model.ProjectStateReady, CreatedAt: time.Now()}

	if err := s.Create(project); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get("p1")
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
	if err := s.Create(project); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(project); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestMemoryProjectStoreNotFound(t *testing.T) {
	s := NewMemoryProjectStore()
	if _, err := s.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestMemoryProjectStoreReturnsCopy(t *testing.T) {
	s := NewMemoryProjectStore()
	s.Create(&model.Project{ID: "p1", Name: "original"})

	got, _ := s.Get("p1")
	got.Name = "mutated"

	again, _ := s.Get("p1")
	if again.Name != "original" {
		t.Fatalf("store mutated through returned pointer: %q", again.Name)
	}
}

func TestMemoryProjectStoreListSortedNewestFirst(t *testing.T) {
	s := NewMemoryProjectStore()
	now := time.Now()
	s.Create(&model.Project{ID: "old", CreatedAt: now.Add(-time.Hour)})
	s.Create(&model.Project{ID: "new", CreatedAt: now})

	list := s.List()
	if len(list) != 2 || list[0].ID != "new" {
		t.Fatalf("unexpected order: %+v", list)
	}
}

package leaderboard

import (
	"context"
	"fmt"
	"testing"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type fakePublished struct{ items []publish.Published }

func (f fakePublished) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type fakeProjects struct{ items []*model.Project }

func (f fakeProjects) List(context.Context) []*model.Project { return f.items }

type fakeUsers struct{ names map[int64]string }

func (f fakeUsers) Usernames(context.Context, []int64) (map[int64]string, error) { return f.names, nil }

type fakeCovers struct{}

func (fakeCovers) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example/" + key, nil
}

type flakyUsers struct {
	calls int
	names map[int64]string
}

func (f *flakyUsers) Usernames(context.Context, []int64) (map[int64]string, error) {
	f.calls++
	if f.calls == 1 {
		return nil, fmt.Errorf("resolver unavailable")
	}
	return f.names, nil
}

func newTestService(t *testing.T) (*Service, *Store) {
	t.Helper()
	store, _ := newTestStore(t)
	svc := NewService(
		store,
		fakePublished{items: []publish.Published{
			{ProjectID: "p1", Title: "小游戏", PublishedAt: time.Now()},
			{ProjectID: "p2", Title: "答题", PublishedAt: time.Now()},
		}},
		fakeProjects{items: []*model.Project{
			{ID: "p1", Name: "Demo1", OwnerUserID: 1, CoverS3Path: "covers/p1.png"},
			{ID: "p2", Name: "Demo2", OwnerUserID: 2},
			{ID: "p3", Name: "HiddenProject", OwnerUserID: 1, IsHidden: true},
		}},
		fakeUsers{names: map[int64]string{1: "alice", 2: "bob"}},
	)
	svc.Covers = fakeCovers{}
	return svc, store
}

func TestListPublicFiltersStaleAndTracksExposure(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	for _, id := range []string{"p1", "p1", "p2", "p3"} {
		if err := store.RecordView(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %+v, want 2", items)
	}
	if items[0].ProjectID != "p1" || items[0].Rank != 1 || items[0].Visits != 2 ||
		items[0].Name != "小游戏" || items[0].Author != "alice" ||
		items[0].CoverURL != "https://s3.example/covers/p1.png" {
		t.Fatalf("item[0] = %+v", items[0])
	}
	if items[1].ProjectID != "p2" || items[1].Rank != 2 {
		t.Fatalf("item[1] = %+v", items[1])
	}
	counters, _ := store.Counters(ctx, []string{"p1", "p2", "p3"})
	if counters["p1"].Exposure != 1 || counters["p2"].Exposure != 1 || counters["p3"].Exposure != 0 {
		t.Fatalf("counters = %+v", counters)
	}
	ranked, _ := store.Top(ctx, 10)
	for _, item := range ranked {
		if item.ProjectID == "p3" {
			t.Fatal("stale p3 should be pruned from ranking")
		}
	}

	if _, err := svc.ListPublic(ctx, 50); err != nil {
		t.Fatal(err)
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 2 {
		t.Fatalf("exposure = %d, want 2", counters["p1"].Exposure)
	}
}

func TestListPublicHonorsHiddenAndAdminDoesNotTrackExposure(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	_ = store.RecordView(ctx, "p1")
	_ = store.RecordView(ctx, "p1")
	_ = store.RecordView(ctx, "p2")
	_ = store.Hide(ctx, "p1", true)

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].ProjectID != "p2" {
		t.Fatalf("items = %+v", items)
	}

	rows, err := svc.ListAdmin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !rows[0].Hidden {
		t.Fatalf("rows = %+v", rows)
	}
	counters, _ := store.Counters(ctx, []string{"p1", "p2"})
	if counters["p1"].Exposure != 0 || counters["p2"].Exposure != 1 {
		t.Fatalf("admin must not track exposure: %+v", counters)
	}
}

func TestListPublicKeepsGloballyHiddenRanking(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	svc := NewService(
		store,
		fakePublished{items: []publish.Published{
			{ProjectID: "hidden", Title: "隐藏", PublishedAt: time.Now()},
		}},
		fakeProjects{items: []*model.Project{
			{ID: "hidden", Name: "Hidden", OwnerUserID: 1, IsHidden: true},
			{ID: "stale", Name: "Stale", OwnerUserID: 1},
		}},
		fakeUsers{names: map[int64]string{1: "alice"}},
	)
	_ = store.RecordView(ctx, "hidden")
	_ = store.Hide(ctx, "hidden", true)
	_ = store.RecordView(ctx, "stale")

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items = %+v, want none", items)
	}

	ranked, _ := store.Top(ctx, 10)
	found := false
	for _, item := range ranked {
		if item.ProjectID == "stale" {
			t.Fatal("unpublished project should be pruned")
		}
		if item.ProjectID == "hidden" {
			found = true
		}
	}
	if !found {
		t.Fatal("globally hidden project ranking should survive")
	}
	hiddenSet, _ := store.HiddenSet(ctx)
	if !hiddenSet["hidden"] {
		t.Fatal("ops-hidden flag should survive")
	}
}

func TestListPublicReadsCandidateWindow(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	const total = 60
	published := make([]publish.Published, 0, total)
	projects := make([]*model.Project, 0, total)
	for i := 0; i < total; i++ {
		id := fmt.Sprintf("p%02d", i)
		published = append(published, publish.Published{ProjectID: id, Title: id, PublishedAt: time.Now()})
		project := &model.Project{ID: id, Name: id, OwnerUserID: 1}
		if i == total-1 {
			project.IsHidden = true
		}
		projects = append(projects, project)
		for n := 0; n <= i; n++ {
			if err := store.RecordView(ctx, id); err != nil {
				t.Fatal(err)
			}
		}
	}
	svc := NewService(store, fakePublished{items: published}, fakeProjects{items: projects}, fakeUsers{names: map[int64]string{1: "alice"}})

	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 50 {
		t.Fatalf("len = %d, want 50", len(items))
	}
	if items[0].ProjectID != "p58" {
		t.Fatalf("first item = %+v, want p58", items[0])
	}
}

func TestListPublicRetriesMetaWhenResolverFails(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	published := fakePublished{items: []publish.Published{
		{ProjectID: "p1", Title: "小游戏", PublishedAt: time.Now()},
	}}
	projects := fakeProjects{items: []*model.Project{
		{ID: "p1", Name: "Demo1", OwnerUserID: 1},
	}}
	_ = store.RecordView(ctx, "p1")

	svc := NewService(store, published, projects, &flakyUsers{names: map[int64]string{1: "alice"}})
	items, err := svc.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Author != "" {
		t.Fatalf("items = %+v, want empty author", items)
	}
	meta, _ := store.Meta(ctx, []string{"p1"})
	if _, ok := meta["p1"]; ok {
		t.Fatal("meta should not be persisted when resolver fails")
	}

	good := NewService(store, published, projects, fakeUsers{names: map[int64]string{1: "alice"}})
	items, err = good.ListPublic(ctx, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Author != "alice" {
		t.Fatalf("items = %+v, want author alice", items)
	}
	meta, _ = store.Meta(ctx, []string{"p1"})
	if meta["p1"].Author != "alice" {
		t.Fatalf("meta = %+v, want author alice", meta["p1"])
	}
}

func TestStatsAndUpsertMeta(t *testing.T) {
	svc, store := newTestService(t)
	ctx := context.Background()
	_ = store.RecordExposure(ctx, []string{"p1", "p1"})
	_ = store.RecordClick(ctx, "p1")

	stats, err := svc.Stats(ctx, []StatInput{{ProjectID: "p1", Name: "小游戏"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != 1 || stats[0].Exposure != 2 || stats[0].Clicks != 1 ||
		stats[0].DwellSeconds != 0 || stats[0].Spend != 0 {
		t.Fatalf("stats = %+v", stats)
	}

	if err := svc.UpsertMeta(ctx, &model.Project{ID: "p1", Name: "Demo", OwnerUserID: 1}, "小游戏"); err != nil {
		t.Fatal(err)
	}
	meta, _ := store.Meta(ctx, []string{"p1"})
	if meta["p1"].Title != "小游戏" || meta["p1"].Author != "alice" {
		t.Fatalf("meta = %+v", meta["p1"])
	}
}

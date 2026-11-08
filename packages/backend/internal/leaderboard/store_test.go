package leaderboard

import (
	"context"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewStore(client, "vibebiz"), mr
}

func TestRecordViewKeepsCandidateWindow(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 55; i++ {
		id := fmt.Sprintf("p%02d", i)
		for n := 0; n <= i; n++ {
			if err := store.RecordView(ctx, id); err != nil {
				t.Fatalf("record view: %v", err)
			}
		}
	}

	ranked, err := store.Top(ctx, 100)
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if len(ranked) != 55 {
		t.Fatalf("len = %d, want 55", len(ranked))
	}
	if ranked[0].ProjectID != "p54" || ranked[0].Visits != 55 {
		t.Fatalf("top entry = %+v", ranked[0])
	}

	top, err := store.Top(ctx, 50)
	if err != nil {
		t.Fatalf("top 50: %v", err)
	}
	if len(top) != 50 {
		t.Fatalf("len = %d, want 50", len(top))
	}
	if top[len(top)-1].Visits != 6 {
		t.Fatalf("last entry = %+v, want visits 6", top[len(top)-1])
	}
}

func TestRecordViewCapsCandidatesAtTwoHundred(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	const total = 205
	for i := total - 1; i >= 0; i-- {
		id := fmt.Sprintf("p%03d", i)
		for n := 0; n <= i; n++ {
			if err := store.RecordView(ctx, id); err != nil {
				t.Fatalf("record view: %v", err)
			}
		}
	}

	ranked, err := store.Top(ctx, 300)
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	if len(ranked) != 200 {
		t.Fatalf("len = %d, want 200", len(ranked))
	}
	if ranked[0].ProjectID != "p204" || ranked[0].Visits != 205 {
		t.Fatalf("top entry = %+v, want p204 with 205 views", ranked[0])
	}
	seen := make(map[string]bool, len(ranked))
	for _, item := range ranked {
		seen[item.ProjectID] = true
	}
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("p%03d", i)
		if seen[id] {
			t.Fatalf("lowest project %s should be evicted", id)
		}
	}
}

func TestCountersHideDeleteAndPrune(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()

	if err := store.RecordExposure(ctx, []string{"p1", "p1", "p2"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordClick(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	counters, err := store.Counters(ctx, []string{"p1", "p2", "missing"})
	if err != nil {
		t.Fatal(err)
	}
	if counters["p1"].Exposure != 2 || counters["p1"].Clicks != 1 || counters["missing"].Exposure != 0 {
		t.Fatalf("counters = %+v", counters)
	}

	if err := store.Hide(ctx, "p1", true); err != nil {
		t.Fatal(err)
	}
	hidden, err := store.HiddenSet(ctx)
	if err != nil || !hidden["p1"] {
		t.Fatalf("hidden = %+v, err = %v", hidden, err)
	}
	if err := store.Hide(ctx, "p1", false); err != nil {
		t.Fatal(err)
	}
	hidden, _ = store.HiddenSet(ctx)
	if hidden["p1"] {
		t.Fatal("p1 should be visible")
	}

	if err := store.PutMeta(ctx, "p1", Meta{Title: "小游戏", Author: "alice"}); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordView(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := store.PruneRanking(ctx, []string{"p1"}); err != nil {
		t.Fatal(err)
	}
	ranked, _ := store.Top(ctx, 10)
	if len(ranked) != 0 {
		t.Fatalf("ranked = %+v, want empty", ranked)
	}
	meta, _ := store.Meta(ctx, []string{"p1"})
	if _, ok := meta["p1"]; ok {
		t.Fatal("meta should be pruned")
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 2 || counters["p1"].Clicks != 1 {
		t.Fatalf("counters should survive prune: %+v", counters["p1"])
	}

	if err := store.Delete(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	counters, _ = store.Counters(ctx, []string{"p1"})
	if counters["p1"].Exposure != 0 || counters["p1"].Clicks != 0 {
		t.Fatalf("counters should be deleted: %+v", counters["p1"])
	}
}

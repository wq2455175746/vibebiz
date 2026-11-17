package businesscard

import (
	"context"
	"strconv"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestLikeStore(t *testing.T, maxLen int64, shards int) (*RedisLikeStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisLikeStore(client, "vibebiz", maxLen, shards), mr
}

func streamEntries(t *testing.T, mr *miniredis.Miniredis, key string) []miniredis.StreamEntry {
	t.Helper()
	entries, err := mr.Stream(key)
	if err != nil {
		t.Fatalf("read stream %s: %v", key, err)
	}
	return entries
}

func streamField(t *testing.T, entry miniredis.StreamEntry, field string) string {
	t.Helper()
	for i := 0; i+1 < len(entry.Values); i += 2 {
		if entry.Values[i] == field {
			return entry.Values[i+1]
		}
	}
	t.Fatalf("field %q missing in %v", field, entry.Values)
	return ""
}

func TestRedisLikeStoreToggleIsIdempotent(t *testing.T) {
	store, mr := newTestLikeStore(t, 1_000_000, 1)

	state, err := store.SetLiked(context.Background(), 10, 7, true)
	if err != nil || !state.Liked || state.Count != 1 || !state.Found {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	state, err = store.SetLiked(context.Background(), 10, 7, true)
	if err != nil || !state.Liked || state.Count != 1 {
		t.Fatalf("repeat state=%+v err=%v", state, err)
	}
	state, err = store.SetLiked(context.Background(), 10, 7, false)
	if err != nil || state.Liked || state.Count != 0 {
		t.Fatalf("unlike state=%+v err=%v", state, err)
	}
	state, err = store.SetLiked(context.Background(), 10, 7, false)
	if err != nil || state.Liked || state.Count != 0 {
		t.Fatalf("repeat unlike state=%+v err=%v", state, err)
	}
	if got := streamEntries(t, mr, "vibebiz:card:like:stream"); len(got) != 2 {
		t.Fatalf("stream len = %d, want 2", len(got))
	}
}

func TestRedisLikeStoreStreamEventFields(t *testing.T) {
	store, mr := newTestLikeStore(t, 1_000_000, 1)
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLiked(context.Background(), 10, 7, false); err != nil {
		t.Fatal(err)
	}

	entries := streamEntries(t, mr, "vibebiz:card:like:stream")
	if len(entries) != 2 {
		t.Fatalf("stream len = %d, want 2", len(entries))
	}
	for i, action := range []string{"like", "unlike"} {
		entry := entries[i]
		if streamField(t, entry, "eventId") == "" {
			t.Fatalf("eventId empty in %+v", entry)
		}
		if got := streamField(t, entry, "cardId"); got != "10" {
			t.Fatalf("cardId = %q, want 10", got)
		}
		if got := streamField(t, entry, "userId"); got != "7" {
			t.Fatalf("userId = %q, want 7", got)
		}
		if got := streamField(t, entry, "action"); got != action {
			t.Fatalf("action = %q, want %q", got, action)
		}
		ts, err := strconv.ParseInt(streamField(t, entry, "ts"), 10, 64)
		if err != nil || ts <= 0 {
			t.Fatalf("ts = %q err = %v", streamField(t, entry, "ts"), err)
		}
	}
}

func TestRedisLikeStoreCountsDistinctUsers(t *testing.T) {
	store, mr := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	for _, userID := range []int64{7, 8, 7} {
		if _, err := store.SetLiked(ctx, 10, userID, true); err != nil {
			t.Fatal(err)
		}
	}

	state, err := store.SetLiked(ctx, 10, 9, true)
	if err != nil || state.Count != 3 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if got := streamEntries(t, mr, "vibebiz:card:like:stream"); len(got) != 3 {
		t.Fatalf("stream len = %d, want 3", len(got))
	}
}

func TestRedisLikeStoreStatesAndCountsMissingKeys(t *testing.T) {
	store, _ := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	states, err := store.States(ctx, []int64{10, 11}, 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []int64{10, 11} {
		if states[id].Found || states[id].Liked || states[id].Count != 0 {
			t.Fatalf("state[%d] = %+v, want zero", id, states[id])
		}
	}

	counts, err := store.Counts(ctx, []int64{10, 11})
	if err != nil {
		t.Fatal(err)
	}
	if len(counts) != 2 || counts[10] != 0 || counts[11] != 0 {
		t.Fatalf("counts = %+v, want zeros", counts)
	}

	emptyStates, err := store.States(ctx, nil, 7)
	if err != nil || len(emptyStates) != 0 {
		t.Fatalf("empty states = %+v err=%v", emptyStates, err)
	}
	emptyCounts, err := store.Counts(ctx, nil)
	if err != nil || len(emptyCounts) != 0 {
		t.Fatalf("empty counts = %+v err=%v", emptyCounts, err)
	}
}

func TestRedisLikeStoreStatesReflectLikes(t *testing.T) {
	store, _ := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	if _, err := store.SetLiked(ctx, 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLiked(ctx, 10, 8, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLiked(ctx, 11, 8, true); err != nil {
		t.Fatal(err)
	}

	states, err := store.States(ctx, []int64{10, 11, 12}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !states[10].Found || !states[10].Liked || states[10].Count != 2 {
		t.Fatalf("state[10] = %+v", states[10])
	}
	if !states[11].Found || states[11].Liked || states[11].Count != 1 {
		t.Fatalf("state[11] = %+v", states[11])
	}
	if states[12].Found || states[12].Count != 0 {
		t.Fatalf("state[12] = %+v", states[12])
	}
}

func TestRedisLikeStoreSeedCountDoesNotOverwrite(t *testing.T) {
	store, _ := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	if err := store.SeedCount(ctx, 10, 100); err != nil {
		t.Fatal(err)
	}
	if err := store.SeedCount(ctx, 10, 200); err != nil {
		t.Fatal(err)
	}
	counts, err := store.Counts(ctx, []int64{10})
	if err != nil {
		t.Fatal(err)
	}
	if counts[10] != 100 {
		t.Fatalf("count = %d, want 100", counts[10])
	}

	state, err := store.SetLiked(ctx, 10, 7, true)
	if err != nil || state.Count != 101 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestRedisLikeStoreSeedStateSeedsCountAndMembership(t *testing.T) {
	store, _ := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	if err := store.SeedState(ctx, 10, 7, LikeState{Count: 5, Liked: true, Found: true}); err != nil {
		t.Fatal(err)
	}
	states, err := store.States(ctx, []int64{10}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !states[10].Found || states[10].Count != 5 || !states[10].Liked {
		t.Fatalf("state = %+v", states[10])
	}

	if err := store.SeedState(ctx, 10, 7, LikeState{Count: 99, Liked: false, Found: true}); err != nil {
		t.Fatal(err)
	}
	states, err = store.States(ctx, []int64{10}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if states[10].Count != 5 || states[10].Liked {
		t.Fatalf("state after reseed = %+v", states[10])
	}
}

func TestRedisLikeStoreSeedStateSkipsMembershipForAnonymous(t *testing.T) {
	store, _ := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	if err := store.SeedState(ctx, 11, 0, LikeState{Count: 2, Liked: true, Found: true}); err != nil {
		t.Fatal(err)
	}
	states, err := store.States(ctx, []int64{11}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if !states[11].Found || states[11].Count != 2 || states[11].Liked {
		t.Fatalf("state = %+v", states[11])
	}
}

func TestRedisLikeStoreUnlikeWithoutLikeKeepsCountNonNegative(t *testing.T) {
	store, mr := newTestLikeStore(t, 1_000_000, 1)
	ctx := context.Background()

	if err := store.SeedCount(ctx, 10, 0); err != nil {
		t.Fatal(err)
	}
	state, err := store.SetLiked(ctx, 10, 7, false)
	if err != nil || state.Liked || state.Count != 0 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	if got := streamEntries(t, mr, "vibebiz:card:like:stream"); len(got) != 0 {
		t.Fatalf("stream len = %d, want 0", len(got))
	}
}

func TestRedisLikeStoreTrimsStreamToMaxLen(t *testing.T) {
	store, mr := newTestLikeStore(t, 2, 1)
	ctx := context.Background()

	for userID := int64(1); userID <= 5; userID++ {
		if _, err := store.SetLiked(ctx, 10, userID, true); err != nil {
			t.Fatal(err)
		}
	}
	if got := streamEntries(t, mr, "vibebiz:card:like:stream"); len(got) > 2 {
		t.Fatalf("stream len = %d, want <= 2", len(got))
	}
}

func TestRedisLikeStoreStreamKeySharding(t *testing.T) {
	single, singleRedis := newTestLikeStore(t, 1_000_000, 1)
	if _, err := single.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if got := streamEntries(t, singleRedis, "vibebiz:card:like:stream"); len(got) != 1 {
		t.Fatalf("single shard stream len = %d, want 1", len(got))
	}

	sharded, shardedRedis := newTestLikeStore(t, 1_000_000, 4)
	if _, err := sharded.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if got := streamEntries(t, shardedRedis, "vibebiz:card:like:stream:2"); len(got) != 1 {
		t.Fatalf("shard 2 stream len = %d, want 1", len(got))
	}
	if got := streamEntries(t, shardedRedis, "vibebiz:card:like:stream"); len(got) != 0 {
		t.Fatalf("unsharded stream len = %d, want 0", len(got))
	}
}

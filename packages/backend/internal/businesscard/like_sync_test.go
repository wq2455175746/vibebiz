package businesscard

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/alicebob/miniredis/v2"
	"github.com/hibiken/asynq"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/observability"
)

type fakeLikeSyncRepo struct {
	events     []LikeEvent
	recounts   [][]int64
	applyErr   error
	recountErr error
}

func (f *fakeLikeSyncRepo) Apply(_ context.Context, events []LikeEvent) ([]int64, error) {
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	f.events = append(f.events, events...)
	seen := make(map[int64]struct{}, len(events))
	out := make([]int64, 0, len(events))
	for _, event := range events {
		if _, ok := seen[event.CardID]; ok {
			continue
		}
		seen[event.CardID] = struct{}{}
		out = append(out, event.CardID)
	}
	return out, nil
}

func (f *fakeLikeSyncRepo) Recount(_ context.Context, cardIDs []int64) (map[int64]int64, error) {
	if f.recountErr != nil {
		return nil, f.recountErr
	}
	f.recounts = append(f.recounts, append([]int64(nil), cardIDs...))
	out := make(map[int64]int64, len(cardIDs))
	for _, id := range cardIDs {
		out[id] = 0
	}
	return out, nil
}

func newLikeSyncFixture(t *testing.T, shards int) (*RedisLikeStore, *LikeSync, *fakeLikeSyncRepo, *redis.Client) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisLikeStore(client, "vibebiz", 1_000_000, shards)
	repo := &fakeLikeSyncRepo{}
	sync := &LikeSync{
		Client: client, Repo: repo, Prefix: "vibebiz",
		Group: "card-like-sync", Consumer: "test", Batch: 100,
	}
	return store, sync, repo, client
}

func TestLikeSyncAppliesBatchAndAcks(t *testing.T) {
	store, sync, repo, client := newLikeSyncFixture(t, 1)
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 || repo.events[0].CardID != 10 || repo.events[0].UserID != 7 || repo.events[0].Action != "like" {
		t.Fatalf("events = %+v", repo.events)
	}
	if pending, _ := client.XPending(context.Background(), "vibebiz:card:like:stream", "card-like-sync").Result(); pending.Count != 0 {
		t.Fatalf("pending = %d", pending.Count)
	}
}

func TestLikeSyncCreatesConsumerGroup(t *testing.T) {
	store, sync, _, client := newLikeSyncFixture(t, 1)
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	groups, err := client.XInfoGroups(context.Background(), "vibebiz:card:like:stream").Result()
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 || groups[0].Name != "card-like-sync" {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestLikeSyncHandleTaskDelegatesToRun(t *testing.T) {
	store, sync, repo, _ := newLikeSyncFixture(t, 1)
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.HandleTask(context.Background(), asynq.NewTask(TaskTypeLikeSync, nil)); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events = %+v", repo.events)
	}
}

func TestLikeSyncLeavesEventsPendingOnRepositoryFailure(t *testing.T) {
	store, sync, repo, client := newLikeSyncFixture(t, 1)
	repo.applyErr = errors.New("db down")
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
	pending, err := client.XPending(context.Background(), "vibebiz:card:like:stream", "card-like-sync").Result()
	if err != nil {
		t.Fatal(err)
	}
	if pending.Count != 1 {
		t.Fatalf("pending = %d, want 1", pending.Count)
	}
}

func TestLikeSyncLeavesEventsPendingOnRecountFailure(t *testing.T) {
	store, sync, repo, client := newLikeSyncFixture(t, 1)
	repo.recountErr = errors.New("recount failed")
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(context.Background()); err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
	pending, err := client.XPending(context.Background(), "vibebiz:card:like:stream", "card-like-sync").Result()
	if err != nil {
		t.Fatal(err)
	}
	if pending.Count != 1 {
		t.Fatalf("pending = %d, want 1", pending.Count)
	}
}

type orderedLikeSyncRepo struct {
	applyErr error
	applied  []LikeEvent
	state    map[[2]int64]string
}

func (f *orderedLikeSyncRepo) Apply(_ context.Context, events []LikeEvent) ([]int64, error) {
	if f.applyErr != nil {
		return nil, f.applyErr
	}
	seen := make(map[int64]struct{}, len(events))
	affected := make([]int64, 0, len(events))
	for _, event := range events {
		f.applied = append(f.applied, event)
		f.state[[2]int64{event.CardID, event.UserID}] = event.Action
		if _, ok := seen[event.CardID]; !ok {
			seen[event.CardID] = struct{}{}
			affected = append(affected, event.CardID)
		}
	}
	return affected, nil
}

func (f *orderedLikeSyncRepo) Recount(_ context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	for _, id := range cardIDs {
		out[id] = 0
	}
	return out, nil
}

func TestLikeSyncDrainsPendingBeforeNewEvents(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	store := NewRedisLikeStore(client, "vibebiz", 1_000_000, 1)
	repo := &orderedLikeSyncRepo{state: map[[2]int64]string{}}
	sync := &LikeSync{
		Client: client, Repo: repo, Prefix: "vibebiz",
		Group: "card-like-sync", Consumer: "test", Batch: 100,
	}
	const key = "vibebiz:card:like:stream"

	if _, err := store.SetLiked(ctx, 10, 7, true); err != nil {
		t.Fatal(err)
	}
	repo.applyErr = errors.New("db down")
	if err := sync.Run(ctx); err == nil {
		t.Fatal("Run() error = nil, want failure")
	}
	if pending, _ := client.XPending(ctx, key, "card-like-sync").Result(); pending.Count != 1 {
		t.Fatalf("pending = %d, want 1", pending.Count)
	}

	repo.applyErr = nil
	if _, err := store.SetLiked(ctx, 10, 7, false); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}

	if len(repo.applied) != 2 {
		t.Fatalf("applied = %+v, want 2 events", repo.applied)
	}
	if repo.applied[0].Action != "like" || repo.applied[1].Action != "unlike" {
		t.Fatalf("applied order = %q, %q; want like then unlike", repo.applied[0].Action, repo.applied[1].Action)
	}
	if state := repo.state[[2]int64{10, 7}]; state != "unlike" {
		t.Fatalf("final state = %q, want unlike", state)
	}
	if pending, _ := client.XPending(ctx, key, "card-like-sync").Result(); pending.Count != 0 {
		t.Fatalf("pending = %d, want 0", pending.Count)
	}
}

func TestLikeSyncReclaimsIdlePendingMessages(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	store := NewRedisLikeStore(client, "vibebiz", 1_000_000, 1)
	if _, err := store.SetLiked(ctx, 10, 7, true); err != nil {
		t.Fatal(err)
	}
	const key = "vibebiz:card:like:stream"
	if err := client.XGroupCreateMkStream(ctx, key, "card-like-sync", "0").Err(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: "card-like-sync", Consumer: "dead",
		Streams: []string{key, ">"}, Count: 10, Block: -1,
	}).Result(); err != nil {
		t.Fatal(err)
	}
	mr.SetTime(time.Now().Add(6 * time.Minute))

	repo := &fakeLikeSyncRepo{}
	sync := &LikeSync{
		Client: client, Repo: repo, Prefix: "vibebiz",
		Group: "card-like-sync", Consumer: "test", Batch: 100,
	}
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 || repo.events[0].CardID != 10 {
		t.Fatalf("events = %+v", repo.events)
	}
	if pending, _ := client.XPending(ctx, key, "card-like-sync").Result(); pending.Count != 0 {
		t.Fatalf("pending = %d, want 0", pending.Count)
	}
}

func TestLikeSyncConsumesShardedStreams(t *testing.T) {
	store, sync, repo, _ := newLikeSyncFixture(t, 2)
	sync.Shards = 2
	ctx := context.Background()
	if _, err := store.SetLiked(ctx, 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetLiked(ctx, 11, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 2 {
		t.Fatalf("events = %+v", repo.events)
	}
	keys := sync.streamKeys()
	if len(keys) != 2 || keys[0] != "vibebiz:card:like:stream:0" || keys[1] != "vibebiz:card:like:stream:1" {
		t.Fatalf("keys = %v", keys)
	}
}

func TestLikeSyncTrimsStreamByMaxLen(t *testing.T) {
	store, sync, repo, client := newLikeSyncFixture(t, 1)
	ctx := context.Background()
	sync.MaxLen = 2
	for userID := int64(1); userID <= 5; userID++ {
		if _, err := store.SetLiked(ctx, 10, userID, true); err != nil {
			t.Fatal(err)
		}
	}
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 5 {
		t.Fatalf("events = %d, want 5", len(repo.events))
	}
	length, err := client.XLen(ctx, "vibebiz:card:like:stream").Result()
	if err != nil {
		t.Fatal(err)
	}
	if length > 2 {
		t.Fatalf("stream len = %d, want <= 2", length)
	}
}

func TestLikeSyncTrimsStreamByTTL(t *testing.T) {
	store, sync, _, client := newLikeSyncFixture(t, 1)
	ctx := context.Background()
	sync.TTL = time.Millisecond
	if _, err := store.SetLiked(ctx, 10, 7, true); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}
	length, err := client.XLen(ctx, "vibebiz:card:like:stream").Result()
	if err != nil {
		t.Fatal(err)
	}
	if length != 0 {
		t.Fatalf("stream len = %d, want 0", length)
	}
}

func TestLikeSyncIgnoresEmptyStream(t *testing.T) {
	_, sync, repo, _ := newLikeSyncFixture(t, 1)
	if err := sync.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 0 || len(repo.recounts) != 0 {
		t.Fatalf("repo = %+v", repo)
	}
}

func TestLikeSyncRepositoryApplyUpsertsAndSoftDeletes(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLLikeRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO business_card_like.*SELECT id, \? FROM business_card WHERE id = \?`).
		WithArgs(int64(7), int64(10)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`(?s)INSERT INTO business_card_like.*SELECT id, \? FROM business_card WHERE id = \?`).
		WithArgs(int64(8), int64(10)).
		WillReturnResult(sqlmock.NewResult(2, 1))
	mock.ExpectExec("UPDATE business_card_like").
		WithArgs(int64(11), int64(8)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	affected, err := repo.Apply(context.Background(), []LikeEvent{
		{EventID: "e1", CardID: 10, UserID: 7, Action: "like"},
		{EventID: "e2", CardID: 10, UserID: 8, Action: "like"},
		{EventID: "e3", CardID: 11, UserID: 8, Action: "unlike"},
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(affected, func(i, j int) bool { return affected[i] < affected[j] })
	if len(affected) != 2 || affected[0] != 10 || affected[1] != 11 {
		t.Fatalf("affected = %v, want [10 11]", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLikeSyncRepositoryApplyRollsBackOnFailure(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLLikeRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO business_card_like.*SELECT id, \? FROM business_card WHERE id = \?`).
		WithArgs(int64(7), int64(10)).
		WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()

	affected, err := repo.Apply(context.Background(), []LikeEvent{
		{CardID: 10, UserID: 7, Action: "like"},
	})
	if err == nil {
		t.Fatal("Apply() error = nil, want failure")
	}
	if affected != nil {
		t.Fatalf("affected = %v, want nil", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLikeSyncRepositoryApplySkipsMissingCard(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLLikeRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec(`(?s)INSERT INTO business_card_like.*SELECT id, \? FROM business_card WHERE id = \?`).
		WithArgs(int64(7), int64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	affected, err := repo.Apply(context.Background(), []LikeEvent{
		{CardID: 99, UserID: 7, Action: "like"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(affected) != 0 {
		t.Fatalf("affected = %v, want empty", affected)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLikeSyncDropsInvalidEventsWithWarning(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	const key = "vibebiz:card:like:stream"
	for _, values := range []map[string]any{
		{"eventId": "bad-ids", "cardId": "0", "userId": "7", "action": "like", "ts": "1"},
		{"eventId": "bad-action", "cardId": "10", "userId": "7", "action": "share", "ts": "1"},
	} {
		if err := client.XAdd(ctx, &redis.XAddArgs{Stream: key, Values: values}).Err(); err != nil {
			t.Fatal(err)
		}
	}

	var buf bytes.Buffer
	repo := &fakeLikeSyncRepo{}
	sync := &LikeSync{
		Client: client, Repo: repo, Prefix: "vibebiz",
		Group: "card-like-sync", Consumer: "test", Batch: 100,
		Logger: slog.New(slog.NewTextHandler(&buf, nil)),
	}
	if err := sync.Run(ctx); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("events = %+v, want none", repo.events)
	}
	if pending, _ := client.XPending(ctx, key, "card-like-sync").Result(); pending.Count != 0 {
		t.Fatalf("pending = %d, want 0", pending.Count)
	}
	if got := strings.Count(buf.String(), "card like sync dropped invalid event"); got != 2 {
		t.Fatalf("warning count = %d, want 2; log = %s", got, buf.String())
	}
}

func TestLikeSyncRepositoryRecountReturnsCount(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLLikeRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("UPDATE business_card bc").
		WithArgs(int64(10)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT .*like_count.* FROM .*business_card").
		WithArgs(int64(10)).
		WillReturnRows(sqlmock.NewRows([]string{"like_count"}).AddRow(3))
	mock.ExpectCommit()

	counts, err := repo.Recount(context.Background(), []int64{10})
	if err != nil {
		t.Fatal(err)
	}
	if counts[10] != 3 {
		t.Fatalf("counts = %v, want 3", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLikeSyncRecordsMetrics(t *testing.T) {
	store, sync, _, _ := newLikeSyncFixture(t, 1)
	provider, err := observability.New(context.Background(), "vibebiz-test", "cp")
	if err != nil {
		t.Fatal(err)
	}
	defer provider.Shutdown(context.Background())
	sync.Metrics = provider.Metrics
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	if err := sync.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	provider.MetricsHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := rec.Body.String()
	if !strings.Contains(body, "vibebiz_card_like_stream_pending 0") {
		t.Fatalf("pending gauge missing: %s", body)
	}
	if !strings.Contains(body, "vibebiz_card_like_sync_duration_seconds_count 1") {
		t.Fatalf("sync duration samples missing: %s", body)
	}
}

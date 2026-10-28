package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestRedisStore(t *testing.T) (*RedisStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewRedisStore(client, "test"), mr
}

func TestRedisStoreCreateGetDelete(t *testing.T) {
	store, _ := newTestRedisStore(t)
	s := &Session{
		ID:                 "sid-1",
		Realm:              RealmUser,
		UserID:             7,
		Username:           "u",
		Email:              "u@example.com",
		WorkspaceIDs:       []int64{1, 2},
		CurrentWorkspaceID: 2,
		CreatedAt:          time.Unix(1700000000, 0).UTC(),
	}

	if err := store.Create(context.Background(), s, time.Hour); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := store.Get(context.Background(), RealmUser, "sid-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != s.ID || got.UserID != s.UserID || got.Username != s.Username || got.Email != s.Email || got.Realm != s.Realm {
		t.Fatalf("session = %+v, want %+v", got, s)
	}
	if len(got.WorkspaceIDs) != 2 || got.WorkspaceIDs[0] != 1 || got.WorkspaceIDs[1] != 2 {
		t.Fatalf("workspace ids = %v", got.WorkspaceIDs)
	}
	if got.CurrentWorkspaceID != 2 {
		t.Fatalf("current workspace = %d, want 2", got.CurrentWorkspaceID)
	}
	if !got.CreatedAt.Equal(s.CreatedAt) {
		t.Fatalf("created at = %v, want %v", got.CreatedAt, s.CreatedAt)
	}

	if err := store.Delete(context.Background(), RealmUser, "sid-1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(context.Background(), RealmUser, "sid-1"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("get deleted err = %v, want ErrSessionNotFound", err)
	}
}

func TestRedisStoreUpdateKeepsTTL(t *testing.T) {
	store, mr := newTestRedisStore(t)
	s := &Session{ID: "sid-update", Realm: RealmUser, UserID: 1, CurrentWorkspaceID: 1}
	if err := store.Create(context.Background(), s, time.Hour); err != nil {
		t.Fatalf("create: %v", err)
	}
	s.CurrentWorkspaceID = 2
	s.Username = "updated"
	if err := store.Update(context.Background(), s); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, err := store.Get(context.Background(), RealmUser, "sid-update")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.CurrentWorkspaceID != 2 || got.Username != "updated" {
		t.Fatalf("session = %+v", got)
	}
	if ttl := mr.TTL("test:session:user:sid-update"); ttl <= 0 || ttl > time.Hour {
		t.Fatalf("ttl = %v, want remaining hour", ttl)
	}
}

func TestRedisStoreUpdateMissingSession(t *testing.T) {
	store, _ := newTestRedisStore(t)
	err := store.Update(context.Background(), &Session{ID: "missing", Realm: RealmUser})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("err = %v, want ErrSessionNotFound", err)
	}
}

func TestRedisStoreIsolatesRealms(t *testing.T) {
	store, _ := newTestRedisStore(t)
	s := &Session{ID: "same", Realm: RealmUser, UserID: 1}
	if err := store.Create(context.Background(), s, time.Minute); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := store.Get(context.Background(), RealmOps, "same"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("ops realm err = %v, want ErrSessionNotFound", err)
	}
}

func TestRedisStoreExpiry(t *testing.T) {
	store, mr := newTestRedisStore(t)
	s := &Session{ID: "sid-ttl", Realm: RealmUser, UserID: 1}
	if err := store.Create(context.Background(), s, time.Hour); err != nil {
		t.Fatalf("create: %v", err)
	}
	if ttl := mr.TTL("test:session:user:sid-ttl"); ttl != time.Hour {
		t.Fatalf("ttl = %v, want 1h", ttl)
	}
	mr.FastForward(2 * time.Hour)
	if _, err := store.Get(context.Background(), RealmUser, "sid-ttl"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("expired get err = %v, want ErrSessionNotFound", err)
	}
}

func TestRedisStoreDeleteMissingIsNoop(t *testing.T) {
	store, _ := newTestRedisStore(t)
	if err := store.Delete(context.Background(), RealmUser, "missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestRedisStorePropagatesRedisErrors(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := NewRedisStore(client, "test")
	mr.Close()

	if _, err := store.Get(context.Background(), RealmUser, "sid"); err == nil || errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("get err = %v, want raw redis error", err)
	}
}

package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRedisStoreDeleteByUser(t *testing.T) {
	store, _ := newTestRedisStore(t)
	ctx := context.Background()
	for _, session := range []*Session{
		{ID: "u1", Realm: RealmUser, UserID: 7},
		{ID: "u2", Realm: RealmUser, UserID: 8},
		{ID: "o1", Realm: RealmOps, UserID: 7},
	} {
		if err := store.Create(ctx, session, time.Hour); err != nil {
			t.Fatal(err)
		}
	}

	if err := store.DeleteByUser(ctx, RealmUser, 7); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(ctx, RealmUser, "u1"); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("u1 err = %v, want ErrSessionNotFound", err)
	}
	if _, err := store.Get(ctx, RealmUser, "u2"); err != nil {
		t.Fatalf("u2 should remain: %v", err)
	}
	if _, err := store.Get(ctx, RealmOps, "o1"); err != nil {
		t.Fatalf("ops session should remain: %v", err)
	}
}

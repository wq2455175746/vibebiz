package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newTestMetaStore(t *testing.T) (*RedisProjectMetaStore, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return NewProjectMetaStore(client, "test"), mr
}

func TestProjectMetaSetGetDelete(t *testing.T) {
	store, mr := newTestMetaStore(t)
	meta := ProjectMeta{WorkspaceID: 3, OwnerUserID: 7}

	if err := store.Set(context.Background(), "p1", meta); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !mr.Exists("test:project:meta:p1") {
		t.Fatal("redis key missing")
	}
	if ttl := mr.TTL("test:project:meta:p1"); ttl != 0 {
		t.Fatalf("ttl = %v, want none", ttl)
	}
	got, err := store.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != meta {
		t.Fatalf("meta = %+v, want %+v", got, meta)
	}
	if err := store.Delete(context.Background(), "p1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get(context.Background(), "p1"); !errors.Is(err, ErrProjectMetaNotFound) {
		t.Fatalf("get deleted err = %v, want ErrProjectMetaNotFound", err)
	}
}

func TestProjectMetaDeleteMissingIsNoop(t *testing.T) {
	store, _ := newTestMetaStore(t)
	if err := store.Delete(context.Background(), "missing"); err != nil {
		t.Fatalf("delete missing: %v", err)
	}
}

func runProjectAccess(t *testing.T, metas ProjectMetaStore, session *Session, projectID string) *httptest.ResponseRecorder {
	t.Helper()
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if session != nil {
				c.Set(sessionContextKey, session)
			}
			return next(c)
		}
	})
	e.GET("/projects/:id", func(c echo.Context) error {
		projectID := c.Param("id")
		return c.String(http.StatusOK, projectID)
	}, RequireProjectAccess(metas))

	req := httptest.NewRequest(http.MethodGet, "/projects/"+projectID, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func TestRequireProjectAccessAllowsMember(t *testing.T) {
	store, _ := newTestMetaStore(t)
	if err := store.Set(context.Background(), "p1", ProjectMeta{WorkspaceID: 2, OwnerUserID: 7}); err != nil {
		t.Fatal(err)
	}
	rec := runProjectAccess(t, store, &Session{Realm: RealmUser, UserID: 8, WorkspaceIDs: []int64{1, 2}}, "p1")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestRequireProjectAccessDeniesNonMember(t *testing.T) {
	store, _ := newTestMetaStore(t)
	if err := store.Set(context.Background(), "p1", ProjectMeta{WorkspaceID: 2, OwnerUserID: 7}); err != nil {
		t.Fatal(err)
	}
	rec := runProjectAccess(t, store, &Session{Realm: RealmUser, UserID: 9, WorkspaceIDs: []int64{1}}, "p1")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if env := decodeTestEnvelope(t, rec); env.Code != 40401 {
		t.Fatalf("code = %d, want 40401", env.Code)
	}
}

func TestRequireProjectAccessMissingMeta(t *testing.T) {
	store, _ := newTestMetaStore(t)
	rec := runProjectAccess(t, store, &Session{Realm: RealmUser, WorkspaceIDs: []int64{1}}, "missing")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestRequireProjectAccessRedisError(t *testing.T) {
	store, mr := newTestMetaStore(t)
	mr.Close()
	rec := runProjectAccess(t, store, &Session{Realm: RealmUser, WorkspaceIDs: []int64{1}}, "p1")
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if env := decodeTestEnvelope(t, rec); env.Code != 50301 {
		t.Fatalf("code = %d, want 50301", env.Code)
	}
}

func TestRequireProjectAccessNoSession(t *testing.T) {
	store, _ := newTestMetaStore(t)
	rec := runProjectAccess(t, store, nil, "p1")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"

	"vibebiz/internal/leaderboard"
	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type leaderboardPublished struct{ items []publish.Published }

func (f leaderboardPublished) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type leaderboardProjects struct{ items []*model.Project }

func (f leaderboardProjects) List(context.Context) []*model.Project { return f.items }

type leaderboardUsers struct{}

func (leaderboardUsers) Usernames(context.Context, []int64) (map[int64]string, error) {
	return map[int64]string{1: "alice"}, nil
}

func newLeaderboardHandlerForTest(t *testing.T) (*echo.Echo, *leaderboard.Service) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	store := leaderboard.NewStore(client, "vibebiz")
	_ = store.RecordView(context.Background(), "p1")
	svc := leaderboard.NewService(
		store,
		leaderboardPublished{items: []publish.Published{{ProjectID: "p1", Title: "小游戏", PublishedAt: time.Now()}}},
		leaderboardProjects{items: []*model.Project{{
			ID: "p1", Name: "Demo", OwnerUserID: 1, CoverURL: "https://cdn.example.com/p1.png",
		}}},
		leaderboardUsers{},
	)
	h := NewLeaderboardHandler(svc, "https://play.example.com")
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/leaderboard", h.ListPublic, sessionForTest(3))
	e.GET("/cp/api/v1/admin/leaderboard", h.ListAdmin)
	e.PUT("/cp/api/v1/admin/leaderboard/:projectId/hidden", h.Hide)
	e.DELETE("/cp/api/v1/admin/leaderboard/:projectId", h.Delete)
	return e, svc
}

func TestLeaderboardPublicAndAdminFlow(t *testing.T) {
	e, _ := newLeaderboardHandlerForTest(t)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/leaderboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "小游戏") ||
		!strings.Contains(rec.Body.String(), "https://play.example.com/p/p1/") ||
		!strings.Contains(rec.Body.String(), `"coverUrl":"https://cdn.example.com/p1.png"`) {
		t.Fatalf("public status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/leaderboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":1`) {
		t.Fatalf("admin status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/leaderboard/p1/hidden", strings.NewReader(`{"hidden":true}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"hidden":true`) {
		t.Fatalf("hide status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/leaderboard", nil))
	if !strings.Contains(rec.Body.String(), `"items":[]`) {
		t.Fatalf("hidden public body = %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/admin/leaderboard/p1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/leaderboard", nil))
	if !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Fatalf("after delete body = %s", rec.Body.String())
	}
}

func TestLeaderboardHideRejectsInvalidBody(t *testing.T) {
	e, _ := newLeaderboardHandlerForTest(t)
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/leaderboard/p1/hidden", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

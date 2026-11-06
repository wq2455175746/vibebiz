package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/usertrace"
)

type fakeAccessLogRepo struct {
	query usertrace.Query
	page  usertrace.Page
	err   error
}

func (f *fakeAccessLogRepo) List(_ context.Context, query usertrace.Query) (usertrace.Page, error) {
	f.query = query
	return f.page, f.err
}

func TestAccessLogListParsesFilters(t *testing.T) {
	repo := &fakeAccessLogRepo{page: usertrace.Page{
		Items:    []usertrace.Log{{ID: "1", EventType: "request", Username: "wang", Path: "/cp/api/v1/projects", StatusCode: 200, DurationMs: 12, EventAt: time.Unix(1700000000, 0).UTC()}},
		Total:    1,
		Page:     2,
		PageSize: 20,
	}}
	h := NewAccessLogHandler(repo)
	e := echo.New()
	e.GET("/cp/api/v1/admin/access-logs", h.List)

	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/access-logs?page=2&pageSize=20&keyword=wang&eventType=request&from=2026-11-01&to=2026-11-06", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.query.Page != 2 || repo.query.PageSize != 20 || repo.query.Keyword != "wang" || repo.query.EventType != "request" {
		t.Fatalf("query = %+v", repo.query)
	}
	if repo.query.From == nil || repo.query.To == nil {
		t.Fatalf("time range = %+v", repo.query)
	}
	if repo.query.To.Sub(*repo.query.From) != 6*24*time.Hour {
		t.Fatalf("time range = %v - %v", repo.query.From, repo.query.To)
	}

	var env struct {
		Code   int `json:"code"`
		Result struct {
			Logs  []usertrace.Log `json:"logs"`
			Total int64           `json:"total"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || env.Result.Total != 1 || len(env.Result.Logs) != 1 || env.Result.Logs[0].Path != "/cp/api/v1/projects" {
		t.Fatalf("response = %+v", env)
	}
}

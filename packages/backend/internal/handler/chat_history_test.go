package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type fakeHistoryClient struct {
	tail  client.ChatHistoryTail
	err   error
	limit int
}

func (f *fakeHistoryClient) ReadChatHistory(_ context.Context, _ string, limit int) (client.ChatHistoryTail, error) {
	f.limit = limit
	return f.tail, f.err
}

func historyTestEcho(h *ChatHistoryHandler, sessionWorkspaces ...int64) *echo.Echo {
	e := echo.New()
	route := e.GET("/cp/api/v1/projects/:id/chat/history", h.Get)
	_ = route
	e.Add(http.MethodGet, "/cp/api/v1/projects/:id/chat/history", func(c echo.Context) error {
		c.Set("auth.session", sessionForTest(sessionWorkspaces...))
		return h.Get(c)
	})
	return e
}

func TestChatHistoryParsesJSONL(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	project := &model.Project{ID: "p1", Name: "demo", WorkspaceID: 3, OwnerUserID: 7, CreatedAt: time.Now()}
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"type":"session","version":1}`,
		`{"type":"message","id":"a1","parentId":null,"timestamp":"2026-10-29T10:00:00Z","message":{"role":"user","content":"hi","status":"completed","tools":[],"references":[{"kind":"resource","id":"r1","name":"需求.pdf","path":".resources/r1.pdf","contentType":"application/pdf","resourceKind":"document"}]}}`,
		`not-json`,
		`{"type":"message","id":"b2","parentId":"a1","timestamp":"2026-10-29T10:00:01Z","message":{"role":"assistant","content":"ok","status":"completed","tools":[{"id":"t1","name":"Read","status":"done"}]}}`,
	}, "\n")
	sandbox := &fakeHistoryClient{tail: client.ChatHistoryTail{Content: content, Version: "v1"}}
	h := NewChatHistoryHandler(sandbox, projects)
	e := historyTestEcho(h, 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/chat/history", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result chatHistoryResponse `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.Limit != defaultChatHistoryLimit || len(env.Result.Messages) != 2 {
		t.Fatalf("result = %+v", env.Result)
	}
	if env.Result.Messages[1].Text != "ok" || env.Result.Messages[1].Tools[0].Name != "Read" {
		t.Fatalf("messages = %+v", env.Result.Messages)
	}
	if len(env.Result.Messages[0].References) != 1 || env.Result.Messages[0].References[0].Name != "需求.pdf" {
		t.Fatalf("references = %+v", env.Result.Messages[0].References)
	}
	if sandbox.limit != defaultChatHistoryLimit {
		t.Fatalf("limit = %d", sandbox.limit)
	}
}

func TestChatHistoryRejectsForeignWorkspace(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	project := &model.Project{ID: "p1", WorkspaceID: 3, CreatedAt: time.Now()}
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	h := NewChatHistoryHandler(&fakeHistoryClient{}, projects)
	e := historyTestEcho(h, 9)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/chat/history", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestChatHistoryRejectsInvalidLimit(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	project := &model.Project{ID: "p1", WorkspaceID: 3, CreatedAt: time.Now()}
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	h := NewChatHistoryHandler(&fakeHistoryClient{}, projects)
	e := historyTestEcho(h, 3)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/chat/history?limit=999", nil)
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

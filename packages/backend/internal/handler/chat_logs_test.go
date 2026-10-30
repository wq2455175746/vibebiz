package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
)

type fakeChatLogsClient struct {
	logs  []client.ChatLog
	err   error
	limit int
}

func (f *fakeChatLogsClient) ChatLogs(_ context.Context, _ string, limit int) ([]client.ChatLog, error) {
	f.limit = limit
	return f.logs, f.err
}

func TestChatLogs(t *testing.T) {
	sandbox := &fakeChatLogsClient{logs: []client.ChatLog{{
		TS:      "2026-10-30T01:00:00.000Z",
		Level:   "info",
		Source:  "tool",
		Message: "Read index.html",
	}}}
	h := NewChatLogsHandler(sandbox, nil)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/logs", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/logs?limit=80", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if sandbox.limit != 80 {
		t.Fatalf("limit = %d, want 80", sandbox.limit)
	}
	var env struct {
		Result struct {
			Logs []client.ChatLog `json:"logs"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Logs) != 1 || env.Result.Logs[0].Message != "Read index.html" {
		t.Fatalf("result = %+v", env.Result)
	}
}

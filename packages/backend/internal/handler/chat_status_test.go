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

type fakeChatStatusClient struct {
	status client.ChatStatus
	err    error
}

func (f fakeChatStatusClient) ChatStatus(context.Context, string) (client.ChatStatus, error) {
	return f.status, f.err
}

func TestChatStatus(t *testing.T) {
	h := NewChatStatusHandler(fakeChatStatusClient{status: client.ChatStatus{Active: true, TurnID: "turn-1"}}, nil)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/chat/status", h.Get)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/chat/status", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result client.ChatStatus `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if !env.Result.Active || env.Result.TurnID != "turn-1" {
		t.Fatalf("result = %+v", env.Result)
	}
}

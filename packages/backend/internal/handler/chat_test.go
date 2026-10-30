package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/observability"
	"vibebiz/internal/role"
)

type fakeChat struct {
	body string
}

func (f *fakeChat) Chat(_ context.Context, _ string, _ []byte) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(f.body)), nil
}

type errorChat struct {
	body string
}

func (f *errorChat) Chat(_ context.Context, _ string, _ []byte) (io.ReadCloser, error) {
	return &errorAfterReader{data: f.body}, nil
}

type errorAfterReader struct {
	data string
	sent bool
}

func (r *errorAfterReader) Read(p []byte) (int, error) {
	if r.sent {
		return 0, errors.New("stream broke")
	}
	r.sent = true
	return copy(p, r.data), nil
}

func (r *errorAfterReader) Close() error {
	return nil
}

func TestChatRecordsReadError(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	obs, err := observability.New(context.Background(), "vibebiz-dp", "dp")
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Shutdown(context.Background())

	var logs bytes.Buffer
	obs.Logger = slog.New(slog.NewJSONHandler(&logs, nil))

	h := NewChatHandler(&errorChat{body: "data: {\"type\":\"agent\"}\n\n"}, obs)
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)

	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(`{"message":"hi"}`))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	output := logs.String()
	if !strings.Contains(output, `"msg":"chat.end"`) || !strings.Contains(output, `"status":"error"`) {
		t.Fatalf("expected chat.end failure log, got: %s", output)
	}
}

func TestChatStreamsSSE(t *testing.T) {
	h := NewChatHandler(&fakeChat{body: "data: {\"type\":\"agent\"}\n\ndata: [DONE]\n\n"}, nil)
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)

	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(`{"message":"hi"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if ct := rec.Header().Get(echo.HeaderContentType); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("missing DONE marker: %s", rec.Body.String())
	}
}

type fakeRoleResolver struct {
	context *role.Context
	err     error
}

func (f fakeRoleResolver) Resolve(context.Context, string) (*role.Context, error) {
	return f.context, f.err
}

type fakeStorageSyncer struct {
	ids []string
}

func (f *fakeStorageSyncer) Enqueue(projectID string) {
	f.ids = append(f.ids, projectID)
}

func TestChatEnqueuesStorageSyncOnSuccess(t *testing.T) {
	syncer := &fakeStorageSyncer{}
	h := NewChatHandler(&fakeChat{body: "data: [DONE]\n\n"}, nil)
	h.Storage = syncer
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(`{"message":"hi"}`)))
	if len(syncer.ids) != 1 || syncer.ids[0] != "p1" {
		t.Fatalf("enqueued = %+v", syncer.ids)
	}
}

func TestChatEnqueuesStorageSyncOnReadError(t *testing.T) {
	syncer := &fakeStorageSyncer{}
	h := NewChatHandler(&errorChat{body: "data: {\"type\":\"agent\"}\n\n"}, nil)
	h.Storage = syncer
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(`{"message":"hi"}`)))
	if len(syncer.ids) != 1 || syncer.ids[0] != "p1" {
		t.Fatalf("enqueued = %+v", syncer.ids)
	}
}

type captureChat struct{ payload []byte }

func (c *captureChat) Chat(_ context.Context, _ string, payload []byte) (io.ReadCloser, error) {
	c.payload = append([]byte(nil), payload...)
	return io.NopCloser(strings.NewReader("data: [DONE]\n\n")), nil
}

func TestChatInjectsProjectRolesAndOverridesClientPayload(t *testing.T) {
	chat := &captureChat{}
	h := NewChatHandler(chat, nil)
	h.Roles = fakeRoleResolver{context: &role.Context{Roles: []agentrole.Role{{
		ID: 1, RoleCode: "alex", RoleNameEN: "Alex", RoleNameZH: "全栈工程师",
		Description: "code", SystemPrompt: "role prompt",
	}}}}
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)
	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat",
		strings.NewReader(`{"message":"hi","agentRoles":[{"systemPrompt":"hacked"}]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if !strings.Contains(string(chat.payload), `"roleCode":"alex"`) {
		t.Fatalf("payload = %s", chat.payload)
	}
	if strings.Contains(string(chat.payload), "hacked") {
		t.Fatalf("client role leaked: %s", chat.payload)
	}
}

func serveRoleChat(t *testing.T, h *ChatHandler, body string) (*echo.Echo, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	e.POST("/dp/api/v1/projects/:id/chat", h.Chat)
	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/p1/chat", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return e, rec
}

func TestChatStripsClientRolesWhenNoRolesResolve(t *testing.T) {
	chat := &captureChat{}
	h := NewChatHandler(chat, nil)
	h.Roles = fakeRoleResolver{context: &role.Context{}}
	_, rec := serveRoleChat(t, h, `{"message":"hi","agentRoles":[{"systemPrompt":"hacked"}],"teamMode":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(string(chat.payload), "hacked") {
		t.Fatalf("client role leaked: %s", chat.payload)
	}
	if strings.Contains(string(chat.payload), "teamMode") || strings.Contains(string(chat.payload), "agentRoles") {
		t.Fatalf("client role fields leaked: %s", chat.payload)
	}
}

func TestChatStripsClientRolesOnResolverError(t *testing.T) {
	chat := &captureChat{}
	h := NewChatHandler(chat, nil)
	h.Roles = fakeRoleResolver{err: errors.New("role lookup failed")}
	_, rec := serveRoleChat(t, h, `{"message":"hi","agentRoles":[{"systemPrompt":"hacked"}],"teamMode":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if strings.Contains(string(chat.payload), "hacked") {
		t.Fatalf("client role leaked: %s", chat.payload)
	}
	if strings.Contains(string(chat.payload), "teamMode") || strings.Contains(string(chat.payload), "agentRoles") {
		t.Fatalf("client role fields leaked: %s", chat.payload)
	}
}

func TestChatHandlesNullBody(t *testing.T) {
	chat := &captureChat{}
	h := NewChatHandler(chat, nil)
	h.Roles = fakeRoleResolver{context: &role.Context{Roles: []agentrole.Role{{ID: 1, RoleCode: "alex"}}}}
	_, rec := serveRoleChat(t, h, `null`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("missing DONE marker: %s", rec.Body.String())
	}
	if !strings.Contains(string(chat.payload), `"roleCode":"alex"`) {
		t.Fatalf("roles not injected: %s", chat.payload)
	}
}

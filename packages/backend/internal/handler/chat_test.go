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

	"vibebiz/internal/observability"
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

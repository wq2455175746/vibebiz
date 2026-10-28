package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/observability"
)

type fakePreview struct {
	url string
}

func (f fakePreview) PreviewURL(string) string {
	return f.url
}

func TestPreviewProxyInjectsTraceParent(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	obs, err := observability.New(context.Background(), "vibebiz-dp", "dp")
	if err != nil {
		t.Fatal(err)
	}
	defer obs.Shutdown(context.Background())

	var traceparent string
	sandbox := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceparent = r.Header.Get("traceparent")
		_, _ = w.Write([]byte("ok"))
	}))
	defer sandbox.Close()

	h := NewPreviewHandler(fakePreview{url: sandbox.URL + "/internal/projects/p1/preview/"}, obs)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/preview/*", h.Serve)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/preview/index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if traceparent == "" {
		t.Fatal("outgoing preview request missing traceparent")
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

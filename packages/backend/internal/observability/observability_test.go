package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
)

func TestMetricsHandlerExposesProjectCounter(t *testing.T) {
	ctx := context.Background()
	p, err := New(ctx, "vibebiz-test", "cp")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Shutdown(ctx)

	p.HTTPRequests.WithLabelValues("vibebiz-test", "cp", "GET /projects/:id", "GET", "200", "p1").Inc()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics", nil)
	p.MetricsHandler.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "vibebiz_http_requests_total") {
		t.Fatalf("metrics missing counter: %s", body)
	}
	if !strings.Contains(body, `project_id="p1"`) {
		t.Fatalf("metrics missing project_id: %s", body)
	}
}

func TestMiddlewareAddsRequestIDToContext(t *testing.T) {
	ctx := context.Background()
	p, err := New(ctx, "vibebiz-test", "cp")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Shutdown(ctx)

	e := echo.New()
	e.Use(echomiddleware.RequestID())
	e.Use(p.Middleware())
	e.GET("/x", func(c echo.Context) error {
		if got := requestIDFromContext(c.Request().Context()); got != "req-42" {
			t.Fatalf("request id in context = %q", got)
		}
		return c.NoContent(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(echo.HeaderXRequestID, "req-42")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestNewWithoutOTLPEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	ctx := context.Background()
	p, err := New(ctx, "vibebiz-test", "dp")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if p.Logger == nil || p.Tracer == nil || p.MetricsHandler == nil {
		t.Fatal("provider not fully initialized")
	}
	if err := p.Shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

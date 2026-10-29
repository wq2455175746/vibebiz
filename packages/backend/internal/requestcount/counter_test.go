package requestcount

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

func newTestCounter(t *testing.T) (*Counter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	counter := NewCounter(client, "vibebiz")
	counter.now = func() time.Time { return time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC) }
	return counter, mr
}

func TestCounterMiddlewareAndToday(t *testing.T) {
	counter, mr := newTestCounter(t)
	e := echo.New()
	e.Use(counter.Middleware())
	e.GET("/cp/api/v1/projects", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))
	}
	value, err := counter.Today(context.Background())
	if err != nil || value != 3 {
		t.Fatalf("today = %d, err = %v", value, err)
	}
	if ttl := mr.TTL("vibebiz:cp:requests:2026-10-29"); ttl <= 0 || ttl > ttlDuration {
		t.Fatalf("ttl = %v", ttl)
	}
}

func TestCounterSkipsHealthAndMetrics(t *testing.T) {
	counter, _ := newTestCounter(t)
	e := echo.New()
	e.Use(counter.Middleware())
	e.GET("/health", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/metrics", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil))

	value, err := counter.Today(context.Background())
	if err != nil || value != 0 {
		t.Fatalf("today = %d, err = %v", value, err)
	}
}

func TestCounterSurvivesRestart(t *testing.T) {
	counter, _ := newTestCounter(t)
	if err := counter.Inc(context.Background()); err != nil {
		t.Fatal(err)
	}
	restarted := NewCounter(counter.client, "vibebiz")
	restarted.now = counter.now
	value, err := restarted.Today(context.Background())
	if err != nil || value != 1 {
		t.Fatalf("today = %d, err = %v", value, err)
	}
}

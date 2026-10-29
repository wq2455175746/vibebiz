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

	"vibebiz/internal/sandboxregistry"
)

type fakeHeartbeatRepository struct {
	sandboxregistry.Repository
	received sandboxregistry.Heartbeat
	now      time.Time
	instance sandboxregistry.Instance
	err      error
}

func (f *fakeHeartbeatRepository) UpsertHeartbeat(_ context.Context, heartbeat sandboxregistry.Heartbeat, now time.Time) (sandboxregistry.Instance, error) {
	f.received = heartbeat
	f.now = now
	return f.instance, f.err
}

func newHeartbeatRequest(body string, token string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/internal/sandboxes/heartbeat", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set("X-Sandbox-Token", token)
	}
	return req
}

func TestSandboxHeartbeatRejectsMissingToken(t *testing.T) {
	h := NewSandboxRegistryHandler(&fakeHeartbeatRepository{}, "secret", nil)
	e := echo.New()
	e.POST("/cp/api/v1/internal/sandboxes/heartbeat", h.Heartbeat)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newHeartbeatRequest(`{"name":"a","host":"127.0.0.1","port":8000,"maxConcurrency":8}`, ""))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestSandboxHeartbeatRejectsDisabledRegistry(t *testing.T) {
	h := NewSandboxRegistryHandler(&fakeHeartbeatRepository{}, "", nil)
	e := echo.New()
	e.POST("/cp/api/v1/internal/sandboxes/heartbeat", h.Heartbeat)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newHeartbeatRequest(`{"name":"a","host":"127.0.0.1","port":8000,"maxConcurrency":8}`, "secret"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
}

func TestSandboxHeartbeatUpsertsInstance(t *testing.T) {
	repo := &fakeHeartbeatRepository{instance: sandboxregistry.Instance{ID: 1, PublicID: "sb-1"}}
	h := NewSandboxRegistryHandler(repo, "secret", nil)
	e := echo.New()
	e.POST("/cp/api/v1/internal/sandboxes/heartbeat", h.Heartbeat)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newHeartbeatRequest(`{"name":"sandbox-a","host":"10.0.0.11","port":8000,"activeRequests":2,"maxConcurrency":8}`, "secret"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.received.Name != "sandbox-a" || repo.received.Scheme != "http" || repo.received.Host != "10.0.0.11" {
		t.Fatalf("heartbeat = %+v", repo.received)
	}
	if repo.received.ActiveRequests != 2 || repo.received.MaxConcurrency != 8 {
		t.Fatalf("concurrency = %+v", repo.received)
	}
	var env struct {
		Result struct {
			InstanceID string `json:"instanceId"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.InstanceID != "sb-1" {
		t.Fatalf("instance id = %q", env.Result.InstanceID)
	}
}

func TestSandboxHeartbeatValidatesPortAndConcurrency(t *testing.T) {
	h := NewSandboxRegistryHandler(&fakeHeartbeatRepository{}, "secret", nil)
	e := echo.New()
	e.POST("/cp/api/v1/internal/sandboxes/heartbeat", h.Heartbeat)

	for _, body := range []string{
		`{"name":"a","host":"127.0.0.1","port":0,"maxConcurrency":8}`,
		`{"name":"a","host":"127.0.0.1","port":8000,"maxConcurrency":0}`,
		`{"name":"","host":"127.0.0.1","port":8000,"maxConcurrency":8}`,
	} {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, newHeartbeatRequest(body, "secret"))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body = %s, status = %d, want 400", body, rec.Code)
		}
	}
}

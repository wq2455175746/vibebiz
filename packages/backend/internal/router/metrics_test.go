package router

import (
	"net/http"
	"strings"
	"testing"

	"vibebiz/internal/store"
)

func TestControlMetricsEndpoint(t *testing.T) {
	e, _, _ := newCP(t, store.NewMemoryProjectStore())

	serve(e, http.MethodGet, "/cp/api/v1/projects")

	rec := serve(e, http.MethodGet, "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "vibebiz_http_requests_total") {
		t.Fatalf("missing http counter: %s", body)
	}
	if !strings.Contains(body, `plane="cp"`) {
		t.Fatalf("missing cp plane label: %s", body)
	}
}

func TestDataMetricsEndpoint(t *testing.T) {
	sandbox := newSandboxServer(t)
	e, _, authState := newDP(t, sandbox.URL)

	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/projects/p1/files", authState.userCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("files status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = serve(e, http.MethodGet, "/metrics")
	if rec.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "vibebiz_http_requests_total") {
		t.Fatalf("missing http counter: %s", body)
	}
	if !strings.Contains(body, `project_id="p1"`) {
		t.Fatalf("missing project label: %s", body)
	}
	if !strings.Contains(body, "vibebiz_sandbox_requests_total") {
		t.Fatalf("missing sandbox counter: %s", body)
	}
}

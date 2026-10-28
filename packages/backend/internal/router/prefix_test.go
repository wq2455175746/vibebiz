package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlDoesNotServeDataOrUnprefixedRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	for _, target := range []string{
		"/dp/api/v1/projects",
		"/api/v1/projects",
		"/cp/api/v1/projects/p1/files",
		"/cp/api/v1/projects/p1/preview/index.html",
	} {
		rec := serveWithAuth(e, http.MethodGet, target, authState.userCookie)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, rec.Code)
		}
	}
}

func TestDataDoesNotServeControlOrUnprefixedRoutes(t *testing.T) {
	e, _, _ := newDP(t, "http://127.0.0.1:1")

	for _, target := range []string{
		"/cp/api/v1/admin/overview",
		"/cp/api/v1/projects",
		"/api/v1/projects/p1/files",
		"/dp/api/v1/projects",
		"/dp/api/v1/admin/overview",
		"/dp/api/v1/me/profile",
		"/dp/api/v1/workspaces/current",
		"/dp/api/v1/storage",
		"/dp/api/v1/notifications",
		"/dp/api/v1/notifications/ws",
	} {
		rec := serve(e, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, rec.Code)
		}
	}
}

func TestControlServesNotificationRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/notifications", authState.userCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("web notifications status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/notifications", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("web notifications without session status = %d, want 401", rec.Code)
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/notifications", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin notifications status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/notifications", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("admin notifications with user session status = %d, want 401", rec.Code)
	}
}

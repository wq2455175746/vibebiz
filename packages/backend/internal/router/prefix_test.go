package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlDoesNotServeDataOrUnprefixedRoutes(t *testing.T) {
	e, _ := newCP(t, store.NewMemoryProjectStore())

	for _, target := range []string{
		"/dp/api/v1/projects",
		"/api/v1/projects",
		"/cp/api/v1/projects/p1/files",
		"/cp/api/v1/projects/p1/preview/index.html",
	} {
		rec := serve(e, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, rec.Code)
		}
	}
}

func TestDataDoesNotServeControlOrUnprefixedRoutes(t *testing.T) {
	e, _ := newDP(t, "http://127.0.0.1:1")

	for _, target := range []string{
		"/cp/api/v1/admin/overview",
		"/cp/api/v1/projects",
		"/api/v1/projects/p1/files",
		"/dp/api/v1/projects",
		"/dp/api/v1/admin/overview",
	} {
		rec := serve(e, http.MethodGet, target)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, want 404", target, rec.Code)
		}
	}
}

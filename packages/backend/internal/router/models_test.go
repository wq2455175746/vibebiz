package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlServesModelAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/models", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/models", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
}

package router

import (
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestSettingsRoutesRequireUserSession(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPut, "/cp/api/v1/auth/workspace"},
		{http.MethodGet, "/cp/api/v1/me/profile"},
		{http.MethodPut, "/cp/api/v1/me/profile"},
		{http.MethodPost, "/cp/api/v1/me/avatar/presign"},
		{http.MethodPut, "/cp/api/v1/me/avatar"},
		{http.MethodPost, "/cp/api/v1/me/avatar/upload"},
		{http.MethodGet, "/cp/api/v1/workspaces/current"},
		{http.MethodPut, "/cp/api/v1/workspaces/current"},
		{http.MethodPost, "/cp/api/v1/workspaces/current/avatar/presign"},
		{http.MethodPut, "/cp/api/v1/workspaces/current/avatar"},
		{http.MethodPost, "/cp/api/v1/workspaces/current/avatar/upload"},
		{http.MethodGet, "/cp/api/v1/workspaces/current/members"},
		{http.MethodPost, "/cp/api/v1/workspaces/current/members"},
		{http.MethodGet, "/cp/api/v1/workspaces/current/folders"},
		{http.MethodPost, "/cp/api/v1/workspaces/current/folders"},
		{http.MethodGet, "/cp/api/v1/storage"},
	}
	for _, tc := range cases {
		if rec := serve(e, tc.method, tc.path); rec.Code != http.StatusUnauthorized {
			t.Fatalf("anonymous %s %s status = %d, want 401", tc.method, tc.path, rec.Code)
		}
		if rec := serveWithAuth(e, tc.method, tc.path, authState.opsCookie); rec.Code != http.StatusUnauthorized {
			t.Fatalf("ops %s %s status = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

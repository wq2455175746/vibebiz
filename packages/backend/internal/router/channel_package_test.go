package router

import (
	"net/http"
	"strings"
	"testing"

	"vibebiz/internal/store"
)

func TestCPChannelPackageRoutesMounted(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/channel-guides", authState.userCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") {
		t.Fatalf("guides status = %d body=%s", rec.Code, rec.Body.String())
	}

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/cp/api/v1/projects/p1/channel-packages"},
		{http.MethodGet, "/cp/api/v1/projects/p1/channel-packages"},
		{http.MethodGet, "/cp/api/v1/projects/p1/channel-packages/pkg-1/download"},
		{http.MethodDelete, "/cp/api/v1/projects/p1/channel-packages/pkg-1"},
	} {
		rec := serve(e, tc.method, tc.path)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

func TestDPDoesNotExposeChannelPackageRoutes(t *testing.T) {
	e, _, authState := newDP(t, "http://127.0.0.1:1")
	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/dp/api/v1/channel-guides"},
		{http.MethodPost, "/dp/api/v1/projects/p1/channel-packages"},
		{http.MethodGet, "/dp/api/v1/projects/p1/channel-packages"},
		{http.MethodGet, "/dp/api/v1/projects/p1/channel-packages/pkg-1/download"},
		{http.MethodDelete, "/dp/api/v1/projects/p1/channel-packages/pkg-1"},
	} {
		rec := serveWithAuth(e, tc.method, tc.path, authState.userCookie)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
}

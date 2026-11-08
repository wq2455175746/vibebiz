package router

import (
	"net/http"
	"strings"
	"testing"

	"vibebiz/internal/store"
)

func TestCPChannelGuideAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/channel-guides", authState.opsCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") {
		t.Fatalf("admin list status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithAuthBody(e, http.MethodPost, "/cp/api/v1/admin/channel-guides", authState.opsCookie, `{"channel":"taptap_mobile","title":"TapTap","markdown":"# TapTap","enabled":true,"sortOrder":40}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/channel-guides", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user admin status = %d, want 401", rec.Code)
	}
}

func TestDPDoesNotExposeChannelGuideAdminRoutes(t *testing.T) {
	e, _, authState := newDP(t, "http://127.0.0.1:1")
	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/admin/channel-guides", authState.opsCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

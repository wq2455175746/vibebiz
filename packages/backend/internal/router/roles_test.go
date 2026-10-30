package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"vibebiz/internal/store"
)

func TestControlServesRoleAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/roles", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/roles", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
}

func TestControlServesRoleUpdateRoute(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuthBody(e, http.MethodPost, "/cp/api/v1/admin/roles", authState.opsCookie,
		`{"roleCode":"frank","roleNameEn":"Frank","roleNameZh":"测试","systemPrompt":"p","status":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var created struct {
		Result struct {
			ID int64 `json:"id"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	target := fmt.Sprintf("/cp/api/v1/admin/roles/%d", created.Result.ID)
	rec = serveWithAuthBody(e, http.MethodPut, target, authState.opsCookie, `{"description":"d","systemPrompt":"p","status":0}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuthBody(e, http.MethodPut, target, authState.userCookie, `{"description":"d","systemPrompt":"p","status":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
}

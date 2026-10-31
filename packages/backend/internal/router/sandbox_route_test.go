package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/server"
)

func TestDPRoutesProjectsToBoundSandbox(t *testing.T) {
	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[{"path":"a.txt","type":"file","size":1,"mtime":1}]}}`))
	}))
	defer serverA.Close()
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[{"path":"b.txt","type":"file","size":1,"mtime":1}]}}`))
	}))
	defer serverB.Close()

	obs := newProvider(t, "dp")
	authState := newRouterAuth(t)
	authState.metas.items["p1"] = auth.ProjectMeta{WorkspaceID: 1, OwnerUserID: 7}
	authState.metas.items["p2"] = auth.ProjectMeta{WorkspaceID: 1, OwnerUserID: 7}
	e := server.New(config.Config{Plane: "dp", ServiceName: "vibebiz-dp"}, obs)
	RegisterData(e, client.NewRouter(mapResolver{"p1": serverA.URL, "p2": serverB.URL}, obs), obs, authState.authn, authState.metas, nil, nil, nil, nil, nil)

	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/projects/p1/files", authState.userCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "a.txt") {
		t.Fatalf("p1 status=%d body=%s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/dp/api/v1/projects/p2/files", authState.userCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "b.txt") {
		t.Fatalf("p2 status=%d body=%s", rec.Code, rec.Body.String())
	}
}

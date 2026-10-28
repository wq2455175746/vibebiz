package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/client"
	"vibebiz/internal/config"
	"vibebiz/internal/observability"
	"vibebiz/internal/server"
	"vibebiz/internal/store"
)

func newProvider(t *testing.T, plane string) *observability.Provider {
	t.Helper()
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	p, err := observability.New(context.Background(), "vibebiz-"+plane, plane)
	if err != nil {
		t.Fatalf("observability.New: %v", err)
	}
	t.Cleanup(func() {
		_ = p.Shutdown(context.Background())
	})
	return p
}

func newCP(t *testing.T, projects store.ProjectStore) (*echo.Echo, *observability.Provider) {
	t.Helper()
	obs := newProvider(t, "cp")
	e := server.New(config.Config{Plane: "cp", ServiceName: "vibebiz-cp"}, obs)
	RegisterControl(e, admin.NewMemoryRepository(), projects, client.NewSandbox("http://127.0.0.1:1", obs), obs)
	return e, obs
}

func newDP(t *testing.T, sandboxURL string) (*echo.Echo, *observability.Provider) {
	t.Helper()
	obs := newProvider(t, "dp")
	e := server.New(config.Config{Plane: "dp", ServiceName: "vibebiz-dp"}, obs)
	RegisterData(e, client.NewSandbox(sandboxURL, obs), obs)
	return e, obs
}

func newSandboxServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/files"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"files":[]}}`))
		case strings.HasSuffix(r.URL.Path, "/file"):
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"path":"index.html","content":"x","version":"v1","size":1,"mtime":1,"isBinary":false}}`))
		default:
			_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{}}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func serve(e *echo.Echo, method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

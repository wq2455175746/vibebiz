package router

import (
	"context"
	"net/http"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"vibebiz/internal/admin"
	"vibebiz/internal/config"
	"vibebiz/internal/handler"
	"vibebiz/internal/model"
	"vibebiz/internal/observability"
	"vibebiz/internal/server"
	"vibebiz/internal/store"
)

func TestHTTPRequestSpanHasProjectID(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	ctx := context.Background()

	obs, err := observability.New(ctx, "vibebiz-cp", "cp")
	if err != nil {
		t.Fatalf("observability.New: %v", err)
	}
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	obs.TracerProvider = tp
	obs.Tracer = tp.Tracer("vibebiz-cp")
	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
		_ = obs.Shutdown(ctx)
	})

	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", Name: "demo", State: model.ProjectStateReady, WorkspaceID: 1, OwnerUserID: 7, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	authState := newRouterAuth(t)
	e := server.New(config.Config{Plane: "cp", ServiceName: "vibebiz-cp"}, obs)
	RegisterControl(e, admin.NewMemoryRepository(), projects, staticProvisioner{id: 1}, obs, authState.authn, authState.handler, &handler.SettingsHandler{}, &handler.StorageHandler{}, authState.metas, newNotificationHandler(t), nil, newModelHandler(t), newRoleHandler(t), nil, 0, nil, nil, nil, nil, nil, nil, nil, nil)

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects/p1", authState.userCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	found := false
	for _, span := range recorder.Ended() {
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "project.id" && attr.Value.AsString() == "p1" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("no span carried project.id=p1; spans=%v", recorder.Ended())
	}
}

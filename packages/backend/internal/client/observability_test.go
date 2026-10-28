package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"vibebiz/internal/observability"
)

func TestSandboxSpanHasProjectID(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	ctx := context.Background()

	obs, err := observability.New(ctx, "vibebiz-dp", "dp")
	if err != nil {
		t.Fatalf("observability.New: %v", err)
	}
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	obs.TracerProvider = tp
	obs.Tracer = tp.Tracer("vibebiz-dp")
	t.Cleanup(func() {
		_ = tp.Shutdown(ctx)
		_ = obs.Shutdown(ctx)
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"ok","result":{"path":"index.html","content":"x","version":"v1","size":1,"mtime":1,"isBinary":false}}`))
	}))
	defer srv.Close()

	if _, err := NewSandbox(srv.URL, obs).ReadFile(ctx, "p1", "index.html"); err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	found := false
	for _, span := range recorder.Ended() {
		if span.Name() != "sandbox.read_file" {
			continue
		}
		for _, attr := range span.Attributes() {
			if string(attr.Key) == "project.id" && attr.Value.AsString() == "p1" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("sandbox.read_file span missing project.id=p1; spans=%v", recorder.Ended())
	}
}

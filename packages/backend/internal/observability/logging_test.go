package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestLogAddsTraceSpanAndRequestID(t *testing.T) {
	ctx := context.Background()
	p, err := New(ctx, "vibebiz-test", "cp")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Shutdown(ctx)

	var buf bytes.Buffer
	p.Logger = slog.New(slog.NewJSONHandler(&buf, nil))

	traceID, err := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
	if err != nil {
		t.Fatal(err)
	}
	spanID, err := trace.SpanIDFromHex("00f067aa0ba902b7")
	if err != nil {
		t.Fatal(err)
	}
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	logCtx := trace.ContextWithSpanContext(ctx, sc)
	logCtx = contextWithRequestID(logCtx, "req-1")

	p.Log(logCtx, slog.LevelInfo, "business.event", "project_id", "p1")

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("decode log: %v (%s)", err, buf.String())
	}
	if entry["trace_id"] != traceID.String() {
		t.Fatalf("trace_id = %v", entry["trace_id"])
	}
	if entry["span_id"] != spanID.String() {
		t.Fatalf("span_id = %v", entry["span_id"])
	}
	if entry["request_id"] != "req-1" {
		t.Fatalf("request_id = %v", entry["request_id"])
	}
	if entry["project_id"] != "p1" {
		t.Fatalf("project_id = %v", entry["project_id"])
	}
}

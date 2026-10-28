package observability

import (
	"context"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

type Provider struct {
	*Metrics

	ServiceName    string
	Plane          string
	Logger         *slog.Logger
	MetricsHandler http.Handler
	Tracer         trace.Tracer
	TracerProvider trace.TracerProvider
	Shutdown       func(context.Context) error
}

func New(ctx context.Context, serviceName, plane string) (*Provider, error) {
	tracing, err := setupTracing(ctx, serviceName, plane)
	if err != nil {
		return nil, err
	}

	metrics := newMetrics()
	return &Provider{
		Metrics:        metrics,
		ServiceName:    serviceName,
		Plane:          plane,
		Logger:         newLogger(serviceName, plane),
		MetricsHandler: metrics.Handler(),
		Tracer:         tracing.tracerProvider.Tracer(serviceName),
		TracerProvider: tracing.tracerProvider,
		Shutdown:       tracing.shutdown,
	}, nil
}

func (p *Provider) Log(ctx context.Context, level slog.Level, msg string, attrs ...any) {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		attrs = append(attrs, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	if requestID := requestIDFromContext(ctx); requestID != "" {
		attrs = append(attrs, "request_id", requestID)
	}
	p.Logger.Log(ctx, level, msg, attrs...)
}

func (p *Provider) RecordSandboxRequest(operation, status, projectID string) {
	p.Metrics.RecordSandboxRequest(p.ServiceName, p.Plane, operation, status, projectID)
}

func (p *Provider) RecordSSEEvent(projectID, eventType string) {
	p.Metrics.RecordSSEEvent(projectID, eventType)
}

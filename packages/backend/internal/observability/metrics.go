package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	HTTPRequests    *prometheus.CounterVec
	HTTPDuration    *prometheus.HistogramVec
	SandboxRequests *prometheus.CounterVec
	SSEEvents       *prometheus.CounterVec

	registry *prometheus.Registry
}

func newMetrics() *Metrics {
	registry := prometheus.NewRegistry()
	m := &Metrics{
		HTTPRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"service", "plane", "route", "method", "status", "project_id"}),
		HTTPDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "vibebiz_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "plane", "route", "method"}),
		SandboxRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_sandbox_requests_total",
			Help: "Total number of sandbox calls.",
		}, []string{"service", "plane", "operation", "status", "project_id"}),
		SSEEvents: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_sse_events_total",
			Help: "Total number of SSE events relayed.",
		}, []string{"project_id", "type"}),
		registry: registry,
	}

	registry.MustRegister(
		m.HTTPRequests,
		m.HTTPDuration,
		m.SandboxRequests,
		m.SSEEvents,
	)
	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) RecordSandboxRequest(serviceName, plane, operation, status, projectID string) {
	m.SandboxRequests.WithLabelValues(serviceName, plane, operation, status, projectID).Inc()
}

func (m *Metrics) RecordSSEEvent(projectID, eventType string) {
	m.SSEEvents.WithLabelValues(projectID, eventType).Inc()
}

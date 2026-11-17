package observability

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	HTTPRequests          *prometheus.CounterVec
	HTTPDuration          *prometheus.HistogramVec
	SandboxRequests       *prometheus.CounterVec
	SSEEvents             *prometheus.CounterVec
	ProjectLeaseAcquire   *prometheus.CounterVec
	ProjectLeaseRenewFail *prometheus.CounterVec
	Comments              *prometheus.CounterVec
	Likes                 *prometheus.CounterVec
	LikeSyncSeconds       prometheus.Histogram
	LikeStreamPending     prometheus.Gauge

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
		ProjectLeaseAcquire: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_project_lease_acquire_total",
			Help: "Total number of project lease acquisition attempts.",
		}, []string{"service", "plane", "result"}),
		ProjectLeaseRenewFail: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_project_lease_renew_fail_total",
			Help: "Total number of project lease renewal failures.",
		}, []string{"service", "plane"}),
		Comments: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_card_comments_total",
			Help: "Total number of card comment mutations.",
		}, []string{"action", "status"}),
		Likes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "vibebiz_card_likes_total",
			Help: "Total number of card like mutations.",
		}, []string{"action", "status"}),
		LikeSyncSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "vibebiz_card_like_sync_duration_seconds",
			Help:    "Card like stream sync duration.",
			Buckets: prometheus.DefBuckets,
		}),
		LikeStreamPending: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "vibebiz_card_like_stream_pending",
			Help: "Card like stream pending messages.",
		}),
		registry: registry,
	}

	registry.MustRegister(
		m.HTTPRequests,
		m.HTTPDuration,
		m.SandboxRequests,
		m.SSEEvents,
		m.ProjectLeaseAcquire,
		m.ProjectLeaseRenewFail,
		m.Comments,
		m.Likes,
		m.LikeSyncSeconds,
		m.LikeStreamPending,
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

func (m *Metrics) RecordProjectLeaseAcquire(serviceName, plane, result string) {
	m.ProjectLeaseAcquire.WithLabelValues(serviceName, plane, result).Inc()
}

func (m *Metrics) RecordProjectLeaseRenewFail(serviceName, plane string) {
	m.ProjectLeaseRenewFail.WithLabelValues(serviceName, plane).Inc()
}

func (m *Metrics) RecordCardComment(action, status string) {
	m.Comments.WithLabelValues(action, status).Inc()
}

func (m *Metrics) RecordCardLike(action, status string) {
	m.Likes.WithLabelValues(action, status).Inc()
}

func (m *Metrics) ObserveCardLikeSync(seconds float64) {
	m.LikeSyncSeconds.Observe(seconds)
}

func (m *Metrics) SetCardLikeStreamPending(n float64) {
	m.LikeStreamPending.Set(n)
}

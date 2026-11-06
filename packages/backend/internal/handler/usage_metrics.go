package handler

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"vibebiz/internal/usage"
)

type UsageMetricsSource interface {
	Series(ctx context.Context, filter usage.Filter) ([]usage.DailyRow, error)
	Today() time.Time
	ParseDate(value string) (time.Time, error)
}

type UsageMetricsHandler struct {
	Source UsageMetricsSource
}

func NewUsageMetricsHandler(source UsageMetricsSource) *UsageMetricsHandler {
	return &UsageMetricsHandler{Source: source}
}

func (h *UsageMetricsHandler) Serve(c echo.Context) error {
	if h.Source == nil {
		return c.String(http.StatusServiceUnavailable, "usage metrics unavailable")
	}
	from := h.Source.Today()
	to := from
	var err error
	if raw := c.QueryParam("from"); raw != "" {
		from, err = h.Source.ParseDate(raw)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid from")
		}
	}
	if raw := c.QueryParam("to"); raw != "" {
		to, err = h.Source.ParseDate(raw)
		if err != nil {
			return c.String(http.StatusBadRequest, "invalid to")
		}
	}
	if to.Before(from) || to.Sub(from) > 31*24*time.Hour {
		return c.String(http.StatusBadRequest, "invalid range")
	}
	userID, _ := strconv.ParseInt(c.QueryParam("user_id"), 10, 64)
	rows, err := h.Source.Series(c.Request().Context(), usage.Filter{From: from, To: to, UserID: userID})
	if err != nil {
		return c.String(http.StatusInternalServerError, "read usage failed")
	}
	renderUsageMetrics(c, rows)
	return nil
}

func renderUsageMetrics(c echo.Context, rows []usage.DailyRow) {
	registry := prometheus.NewRegistry()
	factories := map[string]func() *prometheus.GaugeVec{
		"projects_created": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_projects_created",
				Help: "Daily projects created per user.",
			}, []string{"user_id", "date"})
		},
		"input_tokens": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_input_tokens",
				Help: "Daily input tokens per user.",
			}, []string{"user_id", "date"})
		},
		"output_tokens": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_output_tokens",
				Help: "Daily output tokens per user.",
			}, []string{"user_id", "date"})
		},
		"model_requests": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_model_requests",
				Help: "Daily model requests per user.",
			}, []string{"user_id", "date"})
		},
		"upload_count": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_upload_count",
				Help: "Daily uploaded files per user.",
			}, []string{"user_id", "date"})
		},
		"upload_bytes": func() *prometheus.GaugeVec {
			return prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: "vibebiz_user_daily_upload_bytes",
				Help: "Daily uploaded bytes per user.",
			}, []string{"user_id", "date"})
		},
	}
	gauges := make(map[string]*prometheus.GaugeVec, len(factories))
	for name, factory := range factories {
		gauge := factory()
		gauges[name] = gauge
		registry.MustRegister(gauge)
	}
	for _, row := range rows {
		userID := strconv.FormatInt(row.UserID, 10)
		gauges["projects_created"].WithLabelValues(userID, row.Date).Set(float64(row.ProjectsCreated))
		gauges["input_tokens"].WithLabelValues(userID, row.Date).Set(float64(row.InputTokens))
		gauges["output_tokens"].WithLabelValues(userID, row.Date).Set(float64(row.OutputTokens))
		gauges["model_requests"].WithLabelValues(userID, row.Date).Set(float64(row.ModelRequests))
		gauges["upload_count"].WithLabelValues(userID, row.Date).Set(float64(row.UploadCount))
		gauges["upload_bytes"].WithLabelValues(userID, row.Date).Set(float64(row.UploadBytes))
	}
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{}).ServeHTTP(c.Response(), c.Request())
}

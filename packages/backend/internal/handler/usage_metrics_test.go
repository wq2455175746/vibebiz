package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/usage"
)

type fakeMetricsSource struct {
	rows []usage.DailyRow
}

func (f *fakeMetricsSource) Today() time.Time { return time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC) }

func (f *fakeMetricsSource) ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, time.UTC)
}

func (f *fakeMetricsSource) Series(context.Context, usage.Filter) ([]usage.DailyRow, error) {
	return f.rows, nil
}

func TestUsageMetricsHandlerRendersText(t *testing.T) {
	h := NewUsageMetricsHandler(&fakeMetricsSource{rows: []usage.DailyRow{{
		UserID: 1, Date: "2026-11-06", ProjectsCreated: 3, InputTokens: 1234,
		OutputTokens: 567, ModelRequests: 2, UploadCount: 4, UploadBytes: 1048576,
	}}})
	e := echo.New()
	e.GET("/metrics/usage", h.Serve)

	req := httptest.NewRequest(http.MethodGet, "/metrics/usage?from=2026-11-06&to=2026-11-06", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"vibebiz_user_daily_projects_created{",
		"vibebiz_user_daily_input_tokens{",
		"vibebiz_user_daily_output_tokens{",
		"vibebiz_user_daily_model_requests{",
		"vibebiz_user_daily_upload_count{",
		"vibebiz_user_daily_upload_bytes{",
		`user_id="1"`, `date="2026-11-06"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q:\n%s", want, body)
		}
	}
}

func TestUsageMetricsHandlerRejectsInvalidRange(t *testing.T) {
	h := NewUsageMetricsHandler(&fakeMetricsSource{})
	e := echo.New()
	e.GET("/metrics/usage", h.Serve)

	req := httptest.NewRequest(http.MethodGet, "/metrics/usage?from=2026-11-06&to=2026-11-01", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

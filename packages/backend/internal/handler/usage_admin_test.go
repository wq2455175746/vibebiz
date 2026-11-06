package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/usage"
)

type fakeUsageReader struct {
	today  time.Time
	rows   []usage.DailyRow
	filter usage.Filter
}

func (f *fakeUsageReader) Today() time.Time { return f.today }

func (f *fakeUsageReader) ParseDate(value string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", value, time.UTC)
}

func (f *fakeUsageReader) Series(_ context.Context, filter usage.Filter) ([]usage.DailyRow, error) {
	f.filter = filter
	return f.rows, nil
}

func TestAdminUsageDailyAggregatesTotals(t *testing.T) {
	today := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	reader := &fakeUsageReader{
		today: today,
		rows: []usage.DailyRow{
			{UserID: 1, Date: "2026-11-05", ProjectsCreated: 2, InputTokens: 10, OutputTokens: 5, ModelRequests: 1, UploadCount: 1, UploadBytes: 100},
			{UserID: 1, Date: "2026-11-06", ProjectsCreated: 1, InputTokens: 20, OutputTokens: 7, ModelRequests: 2, UploadCount: 2, UploadBytes: 200},
		},
	}
	h := &AdminHandler{Usage: reader}
	e := echo.New()
	e.GET("/admin/usage/daily", h.UsageDaily)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/daily?from=2026-11-05&to=2026-11-06&userId=1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code   int `json:"code"`
		Result struct {
			From   string           `json:"from"`
			To     string           `json:"to"`
			UserID int64            `json:"userId"`
			Series []usage.DailyRow `json:"series"`
			Totals usage.Usage      `json:"totals"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.From != "2026-11-05" || env.Result.To != "2026-11-06" || env.Result.UserID != 1 {
		t.Fatalf("result = %+v", env.Result)
	}
	if len(env.Result.Series) != 2 {
		t.Fatalf("series = %+v", env.Result.Series)
	}
	if env.Result.Totals.ProjectsCreated != 3 || env.Result.Totals.InputTokens != 30 || env.Result.Totals.UploadBytes != 300 {
		t.Fatalf("totals = %+v", env.Result.Totals)
	}
	if reader.filter.UserID != 1 {
		t.Fatalf("filter = %+v", reader.filter)
	}
}

func TestAdminUsageDailyRejectsInvalidRange(t *testing.T) {
	h := &AdminHandler{Usage: &fakeUsageReader{today: time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)}}
	e := echo.New()
	e.GET("/admin/usage/daily", h.UsageDaily)

	req := httptest.NewRequest(http.MethodGet, "/admin/usage/daily?from=2026-11-06&to=2026-11-01", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

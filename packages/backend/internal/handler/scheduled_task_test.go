package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/usage"
)

func TestScheduledTaskErrorQuotaExceeded(t *testing.T) {
	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)

	if err := scheduledTaskError(c, &usage.QuotaExceededError{
		QuotaType: "project", Used: 1, Limit: 1,
	}); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}
	var env struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 42901 {
		t.Fatalf("code = %d, want 42901", env.Code)
	}
}

package admin

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type auditRecorder struct {
	events []AuditRecord
}

func (r *auditRecorder) RecordAudit(_ context.Context, event AuditRecord) error {
	r.events = append(r.events, event)
	return nil
}

func TestAuditMiddlewareRecordsPublish(t *testing.T) {
	e := echo.New()
	recorder := &auditRecorder{}
	e.Use(AuditMiddleware(recorder, slog.Default()))
	e.POST("/cp/api/v1/admin/notifications/:id/publish", func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{ID: "sid", Realm: auth.RealmOps, UserID: 9, Username: "admin"})
		return response.OK(c, map[string]any{"id": 12})
	})

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/notifications/12/publish", nil)
	req.Header.Set(echo.HeaderXRequestID, "req-1")
	req.Header.Set("User-Agent", "audit-test")
	req.RemoteAddr = "127.0.0.1:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || len(recorder.events) != 1 {
		t.Fatalf("status = %d, events = %+v", rec.Code, recorder.events)
	}
	event := recorder.events[0]
	if event.Action != "notification.publish" || event.ResourceType != "notification" || event.ResourceID != "12" {
		t.Fatalf("event = %+v", event)
	}
	if event.ActorName != "admin" || event.OpsUserID != 9 || event.ResultStatus != "success" {
		t.Fatalf("event = %+v", event)
	}
	if event.RequestID != "req-1" || event.UserAgent != "audit-test" {
		t.Fatalf("request fields = %+v", event)
	}
}

func TestAuditMiddlewareRecordsFailure(t *testing.T) {
	e := echo.New()
	recorder := &auditRecorder{}
	e.Use(AuditMiddleware(recorder, slog.Default()))
	e.DELETE("/cp/api/v1/admin/notifications/:id", func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{ID: "sid", Realm: auth.RealmOps, UserID: 9, Username: "admin"})
		return response.Fail(c, http.StatusConflict, 40901, "conflict")
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/admin/notifications/12", nil))

	if len(recorder.events) != 1 || recorder.events[0].ResultStatus != "failed" {
		t.Fatalf("events = %+v", recorder.events)
	}
	if recorder.events[0].Action != "notification.delete" {
		t.Fatalf("action = %s", recorder.events[0].Action)
	}
}

func TestAuditMiddlewareSkipsReads(t *testing.T) {
	e := echo.New()
	recorder := &auditRecorder{}
	e.Use(AuditMiddleware(recorder, slog.Default()))
	e.GET("/cp/api/v1/admin/users", func(c echo.Context) error {
		return response.OK(c, []string{})
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/users", nil))
	if len(recorder.events) != 0 {
		t.Fatalf("events = %+v, want none", recorder.events)
	}
}

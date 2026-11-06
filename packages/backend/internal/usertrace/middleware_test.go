package usertrace

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
)

type fakeAppender struct {
	events []Event
}

func (f *fakeAppender) Append(event Event) {
	f.events = append(f.events, event)
}

func TestMiddlewareRecordsAuthenticatedRequest(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7, Username: "wang", CurrentWorkspaceID: 3})
			return next(c)
		}
	})
	e.GET("/cp/api/v1/projects/:id", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1", nil)
	req.Header.Set("User-Agent", "Chrome/120.0")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if len(appender.events) != 1 {
		t.Fatalf("events = %+v", appender.events)
	}
	event := appender.events[0]
	if event.EventType != EventRequest || event.UserID != 7 || event.WorkspaceID != 3 || event.ProjectID != "p1" {
		t.Fatalf("event = %+v", event)
	}
	if event.RequestMethod != http.MethodGet || event.StatusCode != http.StatusOK {
		t.Fatalf("event = %+v", event)
	}
}

func TestMiddlewareSkipsAnonymousAndOps(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.GET("/cp/api/v1/projects", func(c echo.Context) error { return c.NoContent(http.StatusOK) })
	e.GET("/cp/api/v1/admin/users", func(c echo.Context) error {
		c.Set("auth.session", &auth.Session{Realm: auth.RealmOps, UserID: 9})
		return c.NoContent(http.StatusOK)
	})

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/users", nil))
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))

	if len(appender.events) != 0 {
		t.Fatalf("events = %+v, want none", appender.events)
	}
}

func TestMiddlewareRecordsHTTPErrorStatus(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7, Username: "wang"})
			return next(c)
		}
	})
	e.PUT("/cp/api/v1/projects/:id/file", func(c echo.Context) error {
		return &echo.HTTPError{Code: http.StatusConflict, Message: "version conflict"}
	})

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPut, "/cp/api/v1/projects/p1/file", nil))

	if len(appender.events) != 1 {
		t.Fatalf("events = %+v", appender.events)
	}
	if appender.events[0].StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want %d", appender.events[0].StatusCode, http.StatusConflict)
	}
}

func TestMiddlewareRecordsPlainErrorAs500(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7})
			return next(c)
		}
	})
	e.GET("/cp/api/v1/projects/:id", func(c echo.Context) error {
		return errors.New("boom")
	})

	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1", nil))

	if len(appender.events) != 1 {
		t.Fatalf("events = %+v", appender.events)
	}
	if appender.events[0].StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", appender.events[0].StatusCode, http.StatusInternalServerError)
	}
}

func TestMiddlewareSkipsOptionsHealthMetrics(t *testing.T) {
	appender := &fakeAppender{}
	e := echo.New()
	e.Use(Middleware(appender, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7})
			return next(c)
		}
	})
	noContent := func(c echo.Context) error { return c.NoContent(http.StatusNoContent) }
	e.GET("/cp/api/v1/projects", noContent)
	e.OPTIONS("/cp/api/v1/projects", noContent)
	e.GET("/health", noContent)
	e.GET("/metrics", noContent)

	for _, tc := range []struct {
		method string
		path   string
	}{
		{http.MethodOptions, "/cp/api/v1/projects"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/metrics"},
	} {
		e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(tc.method, tc.path, nil))
	}

	if len(appender.events) != 0 {
		t.Fatalf("events = %+v, want none", appender.events)
	}
}

func TestMiddlewareNilAppenderDoesNotRecord(t *testing.T) {
	e := echo.New()
	e.Use(Middleware(nil, nil))
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("auth.session", &auth.Session{Realm: auth.RealmUser, UserID: 7})
			return next(c)
		}
	})
	e.GET("/cp/api/v1/projects", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

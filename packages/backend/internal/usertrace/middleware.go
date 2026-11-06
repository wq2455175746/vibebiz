package usertrace

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
)

func Middleware(appender Appender, logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if appender == nil {
				return next(c)
			}
			if c.Request().Method == http.MethodOptions || c.Request().URL.Path == "/health" || c.Request().URL.Path == "/metrics" {
				return next(c)
			}

			start := time.Now()
			handlerErr := next(c)

			session, ok := auth.SessionFrom(c)
			if !ok || session.Realm != auth.RealmUser {
				return handlerErr
			}
			status := c.Response().Status
			if handlerErr != nil {
				status = http.StatusInternalServerError
				var httpErr *echo.HTTPError
				if errors.As(handlerErr, &httpErr) {
					status = httpErr.Code
				}
			}
			info := CaptureRequest(c)
			appender.Append(Event{
				EventType:     EventRequest,
				UserID:        session.UserID,
				Username:      session.Username,
				WorkspaceID:   session.CurrentWorkspaceID,
				ProjectID:     projectIDFromRoute(c),
				IP:            info.IP,
				DeviceID:      info.DeviceID,
				DeviceType:    info.DeviceType,
				OS:            info.OS,
				Browser:       info.Browser,
				UserAgent:     info.UserAgent,
				Referer:       info.Referer,
				RequestMethod: info.Method,
				RequestPath:   info.Path,
				RequestQuery:  info.Query,
				Route:         info.Route,
				StatusCode:    status,
				DurationMs:    int(time.Since(start).Milliseconds()),
				RequestID:     info.RequestID,
				TraceID:       info.TraceID,
			})
			return handlerErr
		}
	}
}

func projectIDFromRoute(c echo.Context) string {
	if strings.Contains(c.Path(), "/projects/:id") {
		return c.Param("id")
	}
	return ""
}

package admin

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/trace"

	"vibebiz/internal/auth"
)

var auditPathPattern = regexp.MustCompile(`[^a-z0-9]+`)

func AuditMiddleware(recorder AuditRecorder, logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !isMutating(c.Request().Method) {
				return next(c)
			}
			handlerErr := next(c)
			session, ok := auth.SessionFrom(c)
			if !ok {
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
			action, resourceType := auditAction(c.Request().Method, c.Path())
			record := AuditRecord{
				OpsUserID:     session.UserID,
				ActorName:     session.Username,
				Action:        action,
				ResourceType:  resourceType,
				ResourceID:    c.Param("id"),
				RequestMethod: c.Request().Method,
				RequestPath:   c.Request().URL.Path,
				RequestID:     requestID(c),
				TraceID:       traceID(c),
				IP:            c.RealIP(),
				UserAgent:     c.Request().UserAgent(),
				ResultStatus:  "success",
			}
			if status >= http.StatusBadRequest {
				record.ResultStatus = "failed"
				record.ErrorMessage = statusText(handlerErr, status)
			}
			ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request().Context()), 2*time.Second)
			defer cancel()
			if recorder != nil {
				if err := recorder.RecordAudit(ctx, record); err != nil && logger != nil {
					logger.Error("admin audit record failed", "action", action, "error", err)
				}
			}
			return handlerErr
		}
	}
}

func isMutating(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func auditAction(method, route string) (string, string) {
	switch route {
	case "/cp/api/v1/admin/notifications":
		return "notification.create", "notification"
	case "/cp/api/v1/admin/notifications/:id":
		if method == http.MethodDelete {
			return "notification.delete", "notification"
		}
		return "notification.update", "notification"
	case "/cp/api/v1/admin/notifications/:id/publish":
		return "notification.publish", "notification"
	case "/cp/api/v1/admin/notifications/:id/unpublish":
		return "notification.unpublish", "notification"
	}
	resourceType := "admin"
	if strings.Contains(route, "notifications") {
		resourceType = "notification"
	}
	slug := strings.Trim(auditPathPattern.ReplaceAllString(strings.TrimPrefix(route, "/cp/api/v1/admin"), "."), ".")
	if slug == "" {
		slug = "root"
	}
	return strings.ToLower(method) + "." + slug, resourceType
}

func requestID(c echo.Context) string {
	if id := c.Response().Header().Get(echo.HeaderXRequestID); id != "" {
		return id
	}
	return c.Request().Header.Get(echo.HeaderXRequestID)
}

func traceID(c echo.Context) string {
	spanContext := trace.SpanContextFromContext(c.Request().Context())
	if !spanContext.IsValid() {
		return ""
	}
	return spanContext.TraceID().String()
}

func statusText(err error, status int) string {
	if err != nil {
		return err.Error()
	}
	return http.StatusText(status)
}

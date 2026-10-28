package observability

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func (p *Provider) Middleware() echo.MiddlewareFunc {
	otelMiddleware := otelecho.Middleware(p.ServiceName, otelecho.WithTracerProvider(p.TracerProvider))

	observe := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			if requestID == "" {
				requestID = c.Request().Header.Get(echo.HeaderXRequestID)
			}
			ctx := contextWithRequestID(c.Request().Context(), requestID)
			c.SetRequest(c.Request().WithContext(ctx))

			route := c.Path()
			if route == "/metrics" || route == "/health" {
				return next(c)
			}
			if route == "" {
				route = "unmatched"
			}

			start := time.Now()
			err := next(c)

			status := c.Response().Status
			if err != nil {
				var httpErr *echo.HTTPError
				if errors.As(err, &httpErr) {
					status = httpErr.Code
				} else if !c.Response().Committed {
					status = http.StatusInternalServerError
				}
			}

			projectID := c.Param("id")
			span := trace.SpanFromContext(c.Request().Context())
			if projectID != "" && span.IsRecording() {
				span.SetAttributes(attribute.String("project.id", projectID))
			}

			method := c.Request().Method
			duration := time.Since(start)
			p.HTTPRequests.WithLabelValues(
				p.ServiceName, p.Plane, route, method, strconv.Itoa(status), projectID,
			).Inc()
			p.HTTPDuration.WithLabelValues(
				p.ServiceName, p.Plane, route, method,
			).Observe(duration.Seconds())

			attrs := []any{
				"project_id", projectID,
				"route", route,
				"method", method,
				"status", status,
				"duration_ms", duration.Milliseconds(),
			}
			if err != nil {
				p.Log(ctx, slog.LevelError, "request.complete", append(attrs, "error", err.Error())...)
			} else {
				p.Log(ctx, slog.LevelInfo, "request.complete", attrs...)
			}
			return err
		}
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return otelMiddleware(observe(next))
	}
}

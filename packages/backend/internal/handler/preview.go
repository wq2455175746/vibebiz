package handler

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

var previewTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableCompression = true
	return transport
}()

type PreviewClient interface {
	PreviewURL(id string) string
}

type PreviewHandler struct {
	Sandbox PreviewClient
	Obs     *observability.Provider
}

func NewPreviewHandler(sandbox PreviewClient, obs *observability.Provider) *PreviewHandler {
	return &PreviewHandler{Sandbox: sandbox, Obs: obs}
}

func (h *PreviewHandler) Serve(c echo.Context) error {
	projectID := c.Param("id")
	if h.Obs != nil {
		ctx, span := h.Obs.Tracer.Start(c.Request().Context(), "preview.proxy",
			trace.WithAttributes(
				attribute.String("project.id", projectID),
				attribute.String("http.route", c.Path()),
			),
		)
		defer span.End()
		c.SetRequest(c.Request().WithContext(ctx))
		h.Obs.Log(ctx, slog.LevelInfo, "preview.proxy", "project_id", projectID, "route", c.Path())
	}

	target, err := url.Parse(h.Sandbox.PreviewURL(projectID))
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50003, "invalid preview target")
	}

	wildcard := c.Param("*")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path + wildcard
			req.Host = target.Host
			otel.GetTextMapPropagator().Inject(req.Context(), propagation.HeaderCarrier(req.Header))
		},
		Transport:     previewTransport,
		FlushInterval: -1,
	}
	proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

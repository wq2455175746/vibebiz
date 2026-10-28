package server

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"vibebiz/internal/config"
	"vibebiz/internal/observability"
)

func New(cfg config.Config, obs *observability.Provider) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Debug = cfg.Debug

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CorsOrigins,
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
	}))

	if obs != nil {
		e.Use(obs.Middleware())
		e.GET("/metrics", echo.WrapHandler(obs.MetricsHandler))
	} else {
		e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))
	}

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{"status": "ok", "plane": cfg.Plane, "time": time.Now().Unix()})
	})

	return e
}

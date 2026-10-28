package router

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/response"
)

func registerErrorHandler(e *echo.Echo) {
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		status := http.StatusInternalServerError
		message := "internal server error"
		var httpErr *echo.HTTPError
		if errors.As(err, &httpErr) {
			status = httpErr.Code
			if msg, ok := httpErr.Message.(string); ok {
				message = msg
			}
		}
		_ = response.Fail(c, status, status, message)
	}
}

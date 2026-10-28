package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Envelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Result  any    `json:"result,omitempty"`
}

func OK(c echo.Context, result any) error {
	return c.JSON(http.StatusOK, Envelope{Code: 0, Message: "ok", Result: result})
}

func Created(c echo.Context, result any) error {
	return c.JSON(http.StatusCreated, Envelope{Code: 0, Message: "ok", Result: result})
}

func Fail(c echo.Context, status, code int, message string) error {
	return c.JSON(status, Envelope{Code: code, Message: message})
}

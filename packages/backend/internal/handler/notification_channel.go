package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/notificationchannel"
	"vibebiz/internal/response"
)

type NotificationChannelHandler struct {
	Service *notificationchannel.Service
}

func NewNotificationChannelHandler(service *notificationchannel.Service) *NotificationChannelHandler {
	return &NotificationChannelHandler{Service: service}
}

type notificationChannelRequest struct {
	Name    string                     `json:"name"`
	Type    notificationchannel.Type   `json:"type"`
	Config  notificationchannel.Config `json:"config"`
	Enabled *bool                      `json:"enabled"`
}

func (r notificationChannelRequest) input() notificationchannel.Input {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return notificationchannel.Input{Name: r.Name, Type: r.Type, Config: r.Config, Enabled: enabled}
}

func (h *NotificationChannelHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	items, err := h.Service.List(c.Request().Context(), session.UserID)
	if err != nil {
		return notificationChannelError(c, err, false)
	}
	return response.OK(c, map[string]any{"channels": items})
}

func (h *NotificationChannelHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req notificationChannelRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	channel, err := h.Service.Create(c.Request().Context(), session.UserID, req.input())
	if err != nil {
		return notificationChannelError(c, err, false)
	}
	return response.Created(c, channel)
}

func (h *NotificationChannelHandler) Update(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := notificationChannelID(c)
	if err != nil {
		return err
	}
	var req notificationChannelRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	channel, err := h.Service.Update(c.Request().Context(), session.UserID, id, req.input())
	if err != nil {
		return notificationChannelError(c, err, false)
	}
	return response.OK(c, channel)
}

func (h *NotificationChannelHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := notificationChannelID(c)
	if err != nil {
		return err
	}
	if err := h.Service.Delete(c.Request().Context(), session.UserID, id); err != nil {
		return notificationChannelError(c, err, false)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *NotificationChannelHandler) Test(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := notificationChannelID(c)
	if err != nil {
		return err
	}
	if _, err := h.Service.Test(c.Request().Context(), session.UserID, id); err != nil {
		return notificationChannelError(c, err, true)
	}
	return response.OK(c, map[string]any{"sent": true})
}

func notificationChannelID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, response.Fail(c, http.StatusBadRequest, 40001, "invalid channel id")
	}
	return id, nil
}

func notificationChannelError(c echo.Context, err error, test bool) error {
	switch {
	case errors.Is(err, notificationchannel.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40013, "invalid notification channel")
	case errors.Is(err, notificationchannel.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40405, "notification channel not found")
	case errors.Is(err, notificationchannel.ErrNameConflict):
		return response.Fail(c, http.StatusConflict, 40904, "notification channel name already exists")
	case errors.Is(err, notificationchannel.ErrCipherUnavailable), errors.Is(err, notificationchannel.ErrSenderUnavailable), errors.Is(err, notificationchannel.ErrEmailNotConfigured):
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "notification channel service unavailable")
	case errors.Is(err, notificationchannel.ErrProviderRejected):
		return response.Fail(c, http.StatusBadGateway, 50204, "notification provider rejected test")
	default:
		if test {
			return response.Fail(c, http.StatusBadGateway, 50205, "notification test failed")
		}
		return response.Fail(c, http.StatusInternalServerError, 50008, "notification channel request failed")
	}
}

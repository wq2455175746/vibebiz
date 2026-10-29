package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
	"vibebiz/internal/response"
)

type NotificationHandler struct {
	Service  *notification.Service
	Hub      *notification.Hub
	Upgrader websocket.Upgrader
}

func NewNotificationHandler(service *notification.Service, hub *notification.Hub, origins []string) *NotificationHandler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[strings.TrimRight(origin, "/")] = struct{}{}
	}
	return &NotificationHandler{
		Service: service,
		Hub:     hub,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimRight(r.Header.Get("Origin"), "/")
				if origin == "" {
					return true
				}
				if _, ok := allowed[origin]; ok {
					return true
				}
				return strings.EqualFold(origin, "http://"+r.Host) || strings.EqualFold(origin, "https://"+r.Host)
			},
		},
	}
}

type notificationRequest struct {
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	Type     model.NotificationType `json:"type"`
	Version  string                 `json:"version"`
	Link     string                 `json:"link"`
	ExpireAt string                 `json:"expireAt"`
}

func (r notificationRequest) input() (notification.CreateInput, error) {
	expireAt, err := time.Parse(time.RFC3339, r.ExpireAt)
	if err != nil {
		return notification.CreateInput{}, notification.ErrInvalid
	}
	return notification.CreateInput{
		Title: r.Title, Content: r.Content, Type: r.Type,
		Version: r.Version, Link: r.Link, ExpireAt: expireAt,
	}, nil
}

func (h *NotificationHandler) AdminList(c echo.Context) error {
	page := queryInt(c, "page", 1)
	pageSize := queryInt(c, "pageSize", 20)
	result, err := h.Service.List(c.Request().Context(), dao.NotificationFilter{
		Type: c.QueryParam("type"), Status: c.QueryParam("status"),
	}, page, pageSize)
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, result)
}

func queryInt(c echo.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.QueryParam(name))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func (h *NotificationHandler) AdminCreate(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req notificationRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	input, err := req.input()
	if err != nil {
		return notificationError(c, err)
	}
	view, err := h.Service.Create(c.Request().Context(), input, notification.Creator{ID: session.UserID, Name: session.Username})
	if err != nil {
		return notificationError(c, err)
	}
	return response.Created(c, view)
}

func (h *NotificationHandler) AdminUpdate(c echo.Context) error {
	id, err := notificationID(c)
	if err != nil {
		return err
	}
	var req notificationRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	input, err := req.input()
	if err != nil {
		return notificationError(c, err)
	}
	view, err := h.Service.Update(c.Request().Context(), id, input)
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, view)
}

func (h *NotificationHandler) AdminPublish(c echo.Context) error {
	return h.transition(c, h.Service.Publish)
}

func (h *NotificationHandler) AdminUnpublish(c echo.Context) error {
	return h.transition(c, h.Service.Unpublish)
}

func (h *NotificationHandler) AdminDelete(c echo.Context) error {
	id, err := notificationID(c)
	if err != nil {
		return err
	}
	if err := h.Service.Delete(c.Request().Context(), id); err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *NotificationHandler) WebList(c echo.Context) error {
	items, err := h.Service.Published(c.Request().Context())
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, map[string]any{"notifications": items})
}

func (h *NotificationHandler) Socket(c echo.Context) error {
	conn, err := h.Upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		slog.Default().Error("notification websocket upgrade failed", "error", err)
		return nil
	}
	h.Hub.Handle(conn)
	return nil
}

func (h *NotificationHandler) transition(c echo.Context, action func(context.Context, int64) (notification.View, error)) error {
	id, err := notificationID(c)
	if err != nil {
		return err
	}
	view, err := action(c.Request().Context(), id)
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, view)
}

func notificationID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return 0, response.Fail(c, http.StatusBadRequest, 40001, "invalid id")
	}
	return id, nil
}

func notificationError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, notification.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid notification")
	case errors.Is(err, notification.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "notification not found")
	case errors.Is(err, notification.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "notification state conflict")
	case errors.Is(err, notification.ErrCacheUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "notification cache unavailable")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "internal error")
	}
}

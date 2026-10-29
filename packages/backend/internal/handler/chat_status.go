package handler

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type ChatStatusClient interface {
	ChatStatus(ctx context.Context, id string) (client.ChatStatus, error)
}

type ChatStatusHandler struct {
	Sandbox ChatStatusClient
	Obs     *observability.Provider
}

func NewChatStatusHandler(sandbox ChatStatusClient, obs *observability.Provider) *ChatStatusHandler {
	return &ChatStatusHandler{Sandbox: sandbox, Obs: obs}
}

func (h *ChatStatusHandler) Get(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()
	status, err := h.Sandbox.ChatStatus(ctx, projectID)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "chat.status", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "read chat status failed")
	}
	return response.OK(c, status)
}

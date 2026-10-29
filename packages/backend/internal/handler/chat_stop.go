package handler

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type ChatStopClient interface {
	StopChat(ctx context.Context, id string) error
}

type ChatStopHandler struct {
	Sandbox ChatStopClient
	Obs     *observability.Provider
}

func NewChatStopHandler(sandbox ChatStopClient, obs *observability.Provider) *ChatStopHandler {
	return &ChatStopHandler{Sandbox: sandbox, Obs: obs}
}

func (h *ChatStopHandler) Stop(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()
	if err := h.Sandbox.StopChat(ctx, projectID); err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "chat.stop", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "stop chat failed")
	}
	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "chat.stop", "project_id", projectID, "status", "ok")
	}
	return response.OK(c, map[string]any{"stopped": true})
}

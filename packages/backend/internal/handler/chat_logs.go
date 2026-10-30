package handler

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type ChatLogsClient interface {
	ChatLogs(ctx context.Context, id string, limit int) ([]client.ChatLog, error)
}

type ChatLogsHandler struct {
	Sandbox ChatLogsClient
	Obs     *observability.Provider
}

func NewChatLogsHandler(sandbox ChatLogsClient, obs *observability.Provider) *ChatLogsHandler {
	return &ChatLogsHandler{Sandbox: sandbox, Obs: obs}
}

func (h *ChatLogsHandler) Get(c echo.Context) error {
	projectID := c.Param("id")
	limit := 100
	if raw := c.QueryParam("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			return response.Fail(c, http.StatusBadRequest, 40007, "invalid log limit")
		}
		limit = value
	}
	logs, err := h.Sandbox.ChatLogs(c.Request().Context(), projectID, limit)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(c.Request().Context(), slog.LevelError, "chat.logs", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "read chat logs failed")
	}
	return response.OK(c, map[string]any{"logs": logs, "limit": limit})
}

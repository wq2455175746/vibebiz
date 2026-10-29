package handler

import (
	"context"
	"log/slog"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type SkillClient interface {
	ListSkills(ctx context.Context, id string) ([]client.SkillInfo, error)
}

type SkillHandler struct {
	Sandbox SkillClient
	Obs     *observability.Provider
}

func NewSkillHandler(sandbox SkillClient, obs *observability.Provider) *SkillHandler {
	return &SkillHandler{Sandbox: sandbox, Obs: obs}
}

func (h *SkillHandler) List(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()
	skills, err := h.Sandbox.ListSkills(ctx, projectID)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "skills.list", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "list skills failed")
	}
	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "skills.list", "project_id", projectID, "status", "ok", "count", len(skills))
	}
	return response.OK(c, map[string]any{"skills": skills})
}

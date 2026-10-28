package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"vibebiz/internal/model"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type ProjectInitializer interface {
	InitProject(ctx context.Context, id string) error
}

type ProjectHandler struct {
	Store   store.ProjectStore
	Sandbox ProjectInitializer
	Obs     *observability.Provider
	Now     func() time.Time
}

func NewProjectHandler(projectStore store.ProjectStore, sandbox ProjectInitializer, obs *observability.Provider) *ProjectHandler {
	return &ProjectHandler{Store: projectStore, Sandbox: sandbox, Obs: obs, Now: time.Now}
}

type createProjectRequest struct {
	Prompt string `json:"prompt"`
	Name   string `json:"name"`
}

func (h *ProjectHandler) Create(c echo.Context) error {
	var req createProjectRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}

	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return response.Fail(c, http.StatusBadRequest, 40002, "prompt is required")
	}

	id := uuid.NewString()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = defaultProjectName(prompt)
	}

	now := h.Now().UTC()
	project := &model.Project{
		ID:        id,
		Name:      name,
		Prompt:    prompt,
		State:     model.ProjectStateReady,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx := c.Request().Context()
	if h.Obs != nil {
		var span trace.Span
		ctx, span = h.Obs.Tracer.Start(ctx, "project.create",
			trace.WithAttributes(
				attribute.String("project.id", id),
				attribute.String("sandbox.operation", "init"),
			),
		)
		defer span.End()
	}

	if err := h.Sandbox.InitProject(ctx, id); err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
		}
		return response.Fail(c, http.StatusBadGateway, 50201, "workspace init failed")
	}

	if err := h.Store.Create(project); err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
		}
		return response.Fail(c, http.StatusInternalServerError, 50001, "create project failed")
	}

	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "project.create", "project_id", id, "state", project.State)
	}

	return response.Created(c, project)
}

func (h *ProjectHandler) List(c echo.Context) error {
	return response.OK(c, map[string]any{"projects": h.Store.List()})
}

func (h *ProjectHandler) Get(c echo.Context) error {
	project, err := h.Store.Get(c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	return response.OK(c, project)
}

func defaultProjectName(prompt string) string {
	name := strings.Join(strings.Fields(prompt), " ")
	if utf8.RuneCountInString(name) <= 24 {
		return name
	}
	runes := []rune(name)
	return string(runes[:24]) + "..."
}

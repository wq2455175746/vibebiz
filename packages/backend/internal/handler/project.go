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

	"vibebiz/internal/auth"
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
	Metas   auth.ProjectMetaStore
	Obs     *observability.Provider
	Now     func() time.Time
}

func NewProjectHandler(projectStore store.ProjectStore, sandbox ProjectInitializer, obs *observability.Provider, metas auth.ProjectMetaStore) *ProjectHandler {
	return &ProjectHandler{Store: projectStore, Sandbox: sandbox, Metas: metas, Obs: obs, Now: time.Now}
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

	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	workspaceID := session.CurrentWorkspaceID
	if workspaceID == 0 {
		return response.Fail(c, http.StatusForbidden, 40302, "no workspace available")
	}
	if !containsInt64(session.WorkspaceIDs, workspaceID) {
		return response.Fail(c, http.StatusForbidden, 40303, "workspace not accessible")
	}

	id := uuid.NewString()
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = defaultProjectName(prompt)
	}

	now := h.Now().UTC()
	project := &model.Project{
		ID:          id,
		Name:        name,
		Prompt:      prompt,
		State:       model.ProjectStateReady,
		WorkspaceID: workspaceID,
		OwnerUserID: session.UserID,
		CreatedAt:   now,
		UpdatedAt:   now,
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

	if h.Metas != nil {
		meta := auth.ProjectMeta{WorkspaceID: workspaceID, OwnerUserID: session.UserID}
		if err := h.Metas.Set(ctx, id, meta); err != nil {
			_ = h.Store.Delete(id)
			if h.Obs != nil {
				h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
			}
			return response.Fail(c, http.StatusInternalServerError, 50003, "project metadata write failed")
		}
	}

	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "project.create", "project_id", id, "state", project.State)
	}

	return response.Created(c, project)
}

func (h *ProjectHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if session.CurrentWorkspaceID == 0 {
		return response.OK(c, map[string]any{"projects": []*model.Project{}})
	}
	return response.OK(c, map[string]any{"projects": h.Store.ListByWorkspaces([]int64{session.CurrentWorkspaceID})})
}

func (h *ProjectHandler) Get(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, err := h.Store.Get(c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	return response.OK(c, project)
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id := c.Param("id")
	project, err := h.Store.Get(id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	if err := h.Store.Delete(id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50004, "delete project failed")
	}
	if h.Metas != nil {
		if err := h.Metas.Delete(c.Request().Context(), id); err != nil && h.Obs != nil {
			h.Obs.Log(c.Request().Context(), slog.LevelError, "project.delete", "project_id", id, "status", "error", "error", err.Error())
		}
	}
	return response.OK(c, map[string]any{"id": id})
}

func containsInt64(ids []int64, target int64) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func defaultProjectName(prompt string) string {
	name := strings.Join(strings.Fields(prompt), " ")
	if utf8.RuneCountInString(name) <= 24 {
		return name
	}
	runes := []rune(name)
	return string(runes[:24]) + "..."
}

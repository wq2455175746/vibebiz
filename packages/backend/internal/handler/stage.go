package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/observability"
	"vibebiz/internal/projectstage"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type StageSandbox interface {
	ReadFile(ctx context.Context, id, path string) (*client.FileContent, error)
	ChatStatus(ctx context.Context, id string) (client.ChatStatus, error)
}

type StageHandler struct {
	Projects store.ProjectStore
	Stages   projectstage.Store
	Sandbox  StageSandbox
	Obs      *observability.Provider
	Now      func() time.Time
}

func NewStageHandler(projects store.ProjectStore, stages projectstage.Store, sandbox StageSandbox) *StageHandler {
	return &StageHandler{Projects: projects, Stages: stages, Sandbox: sandbox, Now: time.Now}
}

type stageView struct {
	StageKey     string     `json:"stageKey"`
	Seq          int        `json:"seq"`
	Title        string     `json:"title"`
	RoleCode     string     `json:"roleCode"`
	RoleName     string     `json:"roleName"`
	ArtifactPath string     `json:"artifactPath"`
	Workspace    string     `json:"workspace"`
	Status       string     `json:"status"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

type stageListResponse struct {
	Mode         string      `json:"mode"`
	CurrentStage string      `json:"currentStage"`
	Stages       []stageView `json:"stages"`
}

type stageMutationResponse struct {
	Stage     stageView  `json:"stage"`
	NextStage *stageView `json:"nextStage,omitempty"`
}

func (h *StageHandler) List(c echo.Context) error {
	project, ok := h.flowProject(c)
	if !ok {
		return nil
	}
	items, err := h.Stages.List(c.Request().Context(), project.ID)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50005, "list project stages failed")
	}
	byKey := make(map[string]model.ProjectStage, len(items))
	current := ""
	for _, item := range items {
		byKey[item.StageKey] = item
		if item.Status == model.ProjectStageActive {
			current = item.StageKey
		}
	}
	stages := make([]stageView, 0, len(projectstage.Definitions))
	for _, definition := range projectstage.DefinitionsForMode(project.Mode) {
		item, exists := byKey[definition.Key]
		if !exists {
			continue
		}
		stages = append(stages, viewStage(definition, item))
	}
	return response.OK(c, stageListResponse{Mode: project.Mode, CurrentStage: current, Stages: stages})
}

func (h *StageHandler) Start(c echo.Context) error {
	project, ok := h.flowProject(c)
	if !ok {
		return nil
	}
	definition, found := projectstage.DefinitionForMode(project.Mode, c.Param("stageKey"))
	if !found {
		return response.Fail(c, http.StatusNotFound, 40402, "stage not found")
	}
	ctx := c.Request().Context()
	stage, err := h.Stages.Get(ctx, project.ID, definition.Key)
	if err != nil {
		return h.stageError(c, err)
	}
	if stage.Status != model.ProjectStageActive || definition.Workspace == projectstage.WorkspacePlaceholder {
		return response.Fail(c, http.StatusConflict, 40901, "stage is not active")
	}
	if stage.StartedAt == nil {
		now := h.now()
		started, err := h.Stages.Start(ctx, project.ID, definition.Key, now)
		if err != nil {
			return h.stageError(c, err)
		}
		if !started {
			stage, err = h.Stages.Get(ctx, project.ID, definition.Key)
			if err != nil {
				return h.stageError(c, err)
			}
			if stage.StartedAt == nil {
				return response.Fail(c, http.StatusConflict, 40901, "stage is not active")
			}
		} else {
			stage.StartedAt = &now
		}
	}
	return response.OK(c, stageMutationResponse{Stage: viewStage(definition, *stage)})
}

func (h *StageHandler) Complete(c echo.Context) error {
	project, ok := h.flowProject(c)
	if !ok {
		return nil
	}
	definition, found := projectstage.DefinitionForMode(project.Mode, c.Param("stageKey"))
	if !found {
		return response.Fail(c, http.StatusNotFound, 40402, "stage not found")
	}
	ctx := c.Request().Context()
	stage, err := h.Stages.Get(ctx, project.ID, definition.Key)
	if err != nil {
		return h.stageError(c, err)
	}
	if stage.Status != model.ProjectStageActive || definition.Workspace == projectstage.WorkspacePlaceholder {
		return response.Fail(c, http.StatusConflict, 40901, "stage is not active")
	}
	if stage.StartedAt == nil {
		return response.Fail(c, http.StatusConflict, 40902, "stage has not started")
	}
	if definition.Workspace != projectstage.WorkspaceArtifact {
		status, err := h.Sandbox.ChatStatus(ctx, project.ID)
		if err != nil {
			return failFromSandbox(c, err, "read agent status failed")
		}
		if status.Active {
			return response.Fail(c, http.StatusConflict, 40903, "agent is still running")
		}
		file, err := h.Sandbox.ReadFile(ctx, project.ID, definition.ArtifactPath)
		if err != nil {
			var apiErr *client.APIError
			if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
				return response.Fail(c, http.StatusBadRequest, 40011, "stage artifact missing")
			}
			return failFromSandbox(c, err, "read stage artifact failed")
		}
		if strings.TrimSpace(file.Content) == "" {
			return response.Fail(c, http.StatusBadRequest, 40011, "stage artifact is empty")
		}
	}
	now := h.now()
	nextKey, err := h.Stages.Complete(ctx, project.ID, definition.Key, now)
	if err != nil {
		if errors.Is(err, projectstage.ErrStageConflict) {
			return response.Fail(c, http.StatusConflict, 40901, "stage is not active")
		}
		return h.stageError(c, err)
	}
	stage.Status = model.ProjectStageDone
	stage.CompletedAt = &now
	result := stageMutationResponse{Stage: viewStage(definition, *stage)}
	if nextKey != "" {
		if nextDefinition, exists := projectstage.DefinitionForMode(project.Mode, nextKey); exists {
			if nextStage, err := h.Stages.Get(ctx, project.ID, nextKey); err == nil {
				view := viewStage(nextDefinition, *nextStage)
				result.NextStage = &view
			}
		}
	}
	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "project.stage.complete", "project_id", project.ID, "stage", definition.Key, "next_stage", nextKey)
	}
	return response.OK(c, result)
}

func (h *StageHandler) flowProject(c echo.Context) (*model.Project, bool) {
	session, ok := auth.SessionFrom(c)
	if !ok {
		_ = response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
		return nil, false
	}
	project, err := h.Projects.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			_ = response.Fail(c, http.StatusNotFound, 40401, "project not found")
		} else {
			_ = response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
		}
		return nil, false
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		_ = response.Fail(c, http.StatusNotFound, 40401, "project not found")
		return nil, false
	}
	if project.Mode != model.ProjectModeEnterprise && project.Mode != model.ProjectModeClone {
		_ = response.Fail(c, http.StatusBadRequest, 40012, "not a staged project")
		return nil, false
	}
	return project, true
}

func (h *StageHandler) stageError(c echo.Context, err error) error {
	if errors.Is(err, projectstage.ErrStageNotFound) {
		return response.Fail(c, http.StatusNotFound, 40402, "stage not found")
	}
	if errors.Is(err, projectstage.ErrStageConflict) {
		return response.Fail(c, http.StatusConflict, 40901, "stage conflict")
	}
	return response.Fail(c, http.StatusInternalServerError, 50005, "project stage operation failed")
}

func (h *StageHandler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
}

func viewStage(definition projectstage.Definition, stage model.ProjectStage) stageView {
	return stageView{
		StageKey: definition.Key, Seq: definition.Seq, Title: definition.Title,
		RoleCode: definition.RoleCode, RoleName: definition.RoleName,
		ArtifactPath: definition.ArtifactPath, Workspace: definition.Workspace,
		Status: stage.Status, StartedAt: stage.StartedAt, CompletedAt: stage.CompletedAt,
	}
}

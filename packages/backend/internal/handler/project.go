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

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/model"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
	"vibebiz/internal/sandboxregistry"
	"vibebiz/internal/store"
)

type ProjectInitializer interface {
	InitProject(ctx context.Context, id string) (int64, error)
}

type HarnessImporter interface {
	Import(ctx context.Context, projectID string) error
}

type ProjectStageInitializer interface {
	CreateInitial(ctx context.Context, projectID string) error
	DeleteByProject(ctx context.Context, projectID string) error
}

const maxProjectAgentRoles = 5

var errInvalidAgentRole = errors.New("invalid agent role")

type AgentRoleStore interface {
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
}

type CoverURLSigner interface {
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type ProjectHandler struct {
	Store       store.ProjectStore
	Sandbox     ProjectInitializer
	Metas       auth.ProjectMetaStore
	Roles       AgentRoleStore
	Covers      CoverURLSigner
	CoverExpiry time.Duration
	Harness     HarnessImporter
	Stages      ProjectStageInitializer
	Obs         *observability.Provider
	Now         func() time.Time
}

func NewProjectHandler(projectStore store.ProjectStore, sandbox ProjectInitializer, obs *observability.Provider, metas auth.ProjectMetaStore, roles AgentRoleStore) *ProjectHandler {
	return &ProjectHandler{Store: projectStore, Sandbox: sandbox, Metas: metas, Roles: roles, Obs: obs, Now: time.Now}
}

type createProjectRequest struct {
	Prompt       string  `json:"prompt"`
	Name         string  `json:"name"`
	Mode         string  `json:"mode"`
	AgentRoleIDs []int64 `json:"agentRoleIds"`
}

func (h *ProjectHandler) normalizeAgentRoles(ctx context.Context, ids []int64) ([]int64, bool, error) {
	seen := make(map[int64]struct{}, len(ids))
	result := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, false, errInvalidAgentRole
		}
		if _, ok := seen[id]; ok {
			continue
		}
		role, err := h.Roles.GetActive(ctx, id)
		if err != nil {
			return nil, false, errInvalidAgentRole
		}
		seen[id] = struct{}{}
		if role.RoleCode == "team" {
			return []int64{role.ID}, true, nil
		}
		result = append(result, role.ID)
	}
	if len(result) > maxProjectAgentRoles {
		return nil, false, errInvalidAgentRole
	}
	return result, false, nil
}

func (h *ProjectHandler) enterpriseRoleIDs(ctx context.Context) ([]int64, error) {
	roles, err := h.Roles.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	wanted := []string{"team", "emma", "bob", "senior", "qa", "alex"}
	byCode := make(map[string]int64, len(roles))
	for _, role := range roles {
		byCode[role.RoleCode] = role.ID
	}
	ids := make([]int64, 0, len(wanted))
	for _, code := range wanted {
		id, ok := byCode[code]
		if !ok {
			return nil, errInvalidAgentRole
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func normalizeProjectMode(value string) (string, error) {
	mode := strings.TrimSpace(value)
	if mode == "" {
		return model.ProjectModeStandard, nil
	}
	if mode != model.ProjectModeStandard && mode != model.ProjectModeEnterprise {
		return "", errors.New("invalid project mode")
	}
	return mode, nil
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
	mode, err := normalizeProjectMode(req.Mode)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40009, "invalid project mode")
	}
	if mode == model.ProjectModeEnterprise && (h.Harness == nil || h.Stages == nil) {
		return response.Fail(c, http.StatusServiceUnavailable, 50303, "enterprise harness unavailable")
	}

	roleIDs, hasTeam, err := h.normalizeAgentRoles(c.Request().Context(), req.AgentRoleIDs)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid agent roles")
	}
	if mode == model.ProjectModeEnterprise && !hasTeam {
		return response.Fail(c, http.StatusBadRequest, 40008, "enterprise project requires team role")
	}
	if mode == model.ProjectModeEnterprise {
		roleIDs, err = h.enterpriseRoleIDs(c.Request().Context())
		if err != nil {
			return response.Fail(c, http.StatusBadRequest, 40010, "enterprise roles unavailable")
		}
	}
	var defaultRoleID *int64
	if len(roleIDs) > 0 {
		first := roleIDs[0]
		defaultRoleID = &first
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
		ID:                 id,
		Name:               name,
		Prompt:             prompt,
		Mode:               mode,
		State:              model.ProjectStateReady,
		WorkspaceID:        workspaceID,
		OwnerUserID:        session.UserID,
		DefaultAgentRoleID: defaultRoleID,
		AgentRoleIDs:       roleIDs,
		CreatedAt:          now,
		UpdatedAt:          now,
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

	sandboxID, err := h.Sandbox.InitProject(ctx, id)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
		}
		if errors.Is(err, sandboxregistry.ErrNoHealthySandbox) {
			return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
		}
		return response.Fail(c, http.StatusBadGateway, 50201, "workspace init failed")
	}
	project.SandboxID = &sandboxID

	if err := h.Store.Create(ctx, project); err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
		}
		return response.Fail(c, http.StatusInternalServerError, 50001, "create project failed")
	}

	if h.Metas != nil {
		meta := auth.ProjectMeta{WorkspaceID: workspaceID, OwnerUserID: session.UserID}
		if err := h.Metas.Set(ctx, id, meta); err != nil {
			_ = h.Store.Delete(c.Request().Context(), id)
			if h.Obs != nil {
				h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
			}
			return response.Fail(c, http.StatusInternalServerError, 50003, "project metadata write failed")
		}
	}

	if mode == model.ProjectModeEnterprise {
		if err := h.Stages.CreateInitial(ctx, id); err != nil {
			_ = h.Metas.Delete(ctx, id)
			_ = h.Store.Delete(ctx, id)
			if h.Obs != nil {
				h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
			}
			return response.Fail(c, http.StatusInternalServerError, 50005, "create project stages failed")
		}
		if err := h.Harness.Import(ctx, id); err != nil {
			_ = h.Stages.DeleteByProject(ctx, id)
			_ = h.Metas.Delete(ctx, id)
			_ = h.Store.Delete(ctx, id)
			if h.Obs != nil {
				h.Obs.Log(ctx, slog.LevelError, "project.create", "project_id", id, "status", "error", "error", err.Error())
			}
			return response.Fail(c, http.StatusBadGateway, 50203, "harness import failed")
		}
	}

	if h.Obs != nil {
		h.Obs.Log(ctx, slog.LevelInfo, "project.create", "project_id", id, "state", project.State, "mode", project.Mode)
	}

	return response.Created(c, project)
}

type updateAgentRolesRequest struct {
	AgentRoleIDs []int64 `json:"agentRoleIds"`
}

func (h *ProjectHandler) UpdateAgentRoles(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req updateAgentRolesRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	project, err := h.Store.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	roleIDs, hasTeam, err := h.normalizeAgentRoles(c.Request().Context(), req.AgentRoleIDs)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40007, "invalid agent roles")
	}
	if project.Mode == model.ProjectModeEnterprise {
		if !hasTeam {
			return response.Fail(c, http.StatusBadRequest, 40008, "enterprise project requires team role")
		}
		roleIDs, err = h.enterpriseRoleIDs(c.Request().Context())
		if err != nil {
			return response.Fail(c, http.StatusBadRequest, 40010, "enterprise roles unavailable")
		}
	}
	if err := h.Store.SetAgentRoles(c.Request().Context(), project.ID, roleIDs); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50011, "update agent roles failed")
	}
	return response.OK(c, map[string]any{"agentRoleIds": roleIDs})
}

func (h *ProjectHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if session.CurrentWorkspaceID == 0 {
		return response.OK(c, map[string]any{"projects": []*model.Project{}})
	}
	projects := h.Store.ListByWorkspaces(c.Request().Context(), []int64{session.CurrentWorkspaceID})
	visible := make([]*model.Project, 0, len(projects))
	for _, project := range projects {
		if project.State == model.ProjectStateDisabled {
			continue
		}
		h.attachCoverURL(c.Request().Context(), project)
		visible = append(visible, project)
	}
	return response.OK(c, map[string]any{"projects": visible})
}

func (h *ProjectHandler) Get(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, err := h.Store.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) || project.State == model.ProjectStateDisabled {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	h.attachCoverURL(c.Request().Context(), project)
	return response.OK(c, project)
}

func (h *ProjectHandler) attachCoverURL(ctx context.Context, project *model.Project) {
	if project == nil || project.CoverS3Path == "" || h.Covers == nil {
		return
	}
	expiry := h.CoverExpiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	coverURL, err := h.Covers.PresignGet(ctx, project.CoverS3Path, expiry)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelWarn, "project.cover", "project_id", project.ID, "status", "presign_error", "error", err.Error())
		}
		return
	}
	project.CoverURL = coverURL
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id := c.Param("id")
	project, err := h.Store.Get(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	if err := h.Store.Delete(c.Request().Context(), id); err != nil {
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

package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type AdminHandler struct {
	Repo          admin.Repository
	ProjectsStore store.ProjectStore
}

func NewAdminHandler(repo admin.Repository, projects store.ProjectStore) *AdminHandler {
	return &AdminHandler{Repo: repo, ProjectsStore: projects}
}

type adminProject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	State       string    `json:"state"`
	Owner       string    `json:"owner"`
	OwnerUserID int64     `json:"ownerUserId"`
	WorkspaceID int64     `json:"workspaceId"`
	CreatedAt   time.Time `json:"createdAt"`
}

func (h *AdminHandler) Overview(c echo.Context) error {
	ctx := c.Request().Context()
	overview, err := h.Repo.Overview(ctx, len(h.ProjectsStore.List(ctx)))
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, overview)
}

func (h *AdminHandler) Users(c echo.Context) error {
	users, err := h.Repo.Users(c.Request().Context())
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, users)
}

func (h *AdminHandler) Sandboxes(c echo.Context) error {
	sandboxes, err := h.Repo.Sandboxes(c.Request().Context())
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, sandboxes)
}

func (h *AdminHandler) Quotas(c echo.Context) error {
	quotas, err := h.Repo.Quotas(c.Request().Context())
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, quotas)
}

func (h *AdminHandler) Audit(c echo.Context) error {
	events, err := h.Repo.Audit(c.Request().Context())
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, events)
}

func (h *AdminHandler) Projects(c echo.Context) error {
	projects := h.ProjectsStore.List(c.Request().Context())
	out := make([]adminProject, 0, len(projects))
	for _, project := range projects {
		out = append(out, adminProject{
			ID:          project.ID,
			Name:        project.Name,
			State:       project.State,
			Owner:       strconv.FormatInt(project.OwnerUserID, 10),
			OwnerUserID: project.OwnerUserID,
			WorkspaceID: project.WorkspaceID,
			CreatedAt:   project.CreatedAt,
		})
	}
	return response.OK(c, map[string]any{"projects": out})
}

func adminRepoError(c echo.Context, _ error) error {
	return response.Fail(c, http.StatusInternalServerError, 50001, "admin data unavailable")
}

package handler

import (
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
	return response.OK(c, h.Repo.Overview(len(h.ProjectsStore.List())))
}

func (h *AdminHandler) Users(c echo.Context) error {
	return response.OK(c, h.Repo.Users())
}

func (h *AdminHandler) Sandboxes(c echo.Context) error {
	return response.OK(c, h.Repo.Sandboxes())
}

func (h *AdminHandler) Quotas(c echo.Context) error {
	return response.OK(c, h.Repo.Quotas())
}

func (h *AdminHandler) Audit(c echo.Context) error {
	return response.OK(c, h.Repo.Audit())
}

func (h *AdminHandler) Projects(c echo.Context) error {
	projects := h.ProjectsStore.List()
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

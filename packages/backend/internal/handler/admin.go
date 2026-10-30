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
	page, err := h.Repo.Users(c.Request().Context(), adminListQuery(c))
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, map[string]any{
		"users": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *AdminHandler) Sandboxes(c echo.Context) error {
	page, err := h.Repo.Sandboxes(c.Request().Context(), adminListQuery(c))
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, map[string]any{
		"sandboxes": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *AdminHandler) Quotas(c echo.Context) error {
	quotas, err := h.Repo.Quotas(c.Request().Context())
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, quotas)
}

func (h *AdminHandler) Audit(c echo.Context) error {
	page, err := h.Repo.Audit(c.Request().Context(), adminListQuery(c))
	if err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, map[string]any{
		"audit": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *AdminHandler) Projects(c echo.Context) error {
	query := admin.NormalizeQuery(adminListQuery(c))
	projects := h.ProjectsStore.List(c.Request().Context())
	filtered := make([]adminProject, 0, len(projects))
	for _, project := range projects {
		owner := strconv.FormatInt(project.OwnerUserID, 10)
		if !admin.MatchesKeyword(query.Keyword, project.ID, project.Name, owner) {
			continue
		}
		filtered = append(filtered, adminProject{
			ID:          project.ID,
			Name:        project.Name,
			State:       project.State,
			Owner:       owner,
			OwnerUserID: project.OwnerUserID,
			WorkspaceID: project.WorkspaceID,
			CreatedAt:   project.CreatedAt,
		})
	}
	page := admin.Paginate(filtered, query)
	return response.OK(c, map[string]any{
		"projects": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func adminListQuery(c echo.Context) admin.ListQuery {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	return admin.ListQuery{Keyword: c.QueryParam("keyword"), Page: page, PageSize: pageSize}
}

func adminRepoError(c echo.Context, _ error) error {
	return response.Fail(c, http.StatusInternalServerError, 50001, "admin data unavailable")
}

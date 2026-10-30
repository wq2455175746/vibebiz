package handler

import (
	"errors"
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

type userStatusRequest struct {
	Status *int8 `json:"status"`
}

func (h *AdminHandler) UpdateUserStatus(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user id")
	}
	var req userStatusRequest
	if err := c.Bind(&req); err != nil || req.Status == nil || (*req.Status != 0 && *req.Status != 1) {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid status")
	}
	if err := h.Repo.UpdateUserStatus(c.Request().Context(), id, *req.Status); err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, map[string]any{"id": strconv.FormatInt(id, 10), "status": *req.Status})
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

type quotaUpdateRequest struct {
	LLMTokensQuota      *int64 `json:"llmTokensQuota"`
	SandboxMinutesQuota *int64 `json:"sandboxMinutesQuota"`
	StorageQuotaMB      *int64 `json:"storageQuotaMb"`
}

func (h *AdminHandler) UpdateQuota(c echo.Context) error {
	userID, err := strconv.ParseInt(c.Param("userId"), 10, 64)
	if err != nil || userID <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid user id")
	}
	var req quotaUpdateRequest
	if err := c.Bind(&req); err != nil || req.LLMTokensQuota == nil || req.SandboxMinutesQuota == nil || req.StorageQuotaMB == nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid quota")
	}
	if *req.LLMTokensQuota < 0 || *req.SandboxMinutesQuota < 0 || *req.StorageQuotaMB < 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid quota")
	}
	if err := h.Repo.UpdateQuota(c.Request().Context(), userID, admin.QuotaUpdateInput{
		LLMTokensQuota:      *req.LLMTokensQuota,
		SandboxMinutesQuota: *req.SandboxMinutesQuota,
		StorageQuotaMB:      *req.StorageQuotaMB,
	}); err != nil {
		return adminRepoError(c, err)
	}
	return response.OK(c, map[string]any{"userId": strconv.FormatInt(userID, 10)})
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

func adminRepoError(c echo.Context, err error) error {
	if errors.Is(err, admin.ErrNotFound) {
		return response.Fail(c, http.StatusNotFound, 40401, "admin record not found")
	}
	return response.Fail(c, http.StatusInternalServerError, 50001, "admin data unavailable")
}

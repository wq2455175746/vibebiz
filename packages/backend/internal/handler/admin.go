package handler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
	"vibebiz/internal/notification"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type AdminSandbox interface {
	ListFiles(ctx context.Context, id string) ([]client.FileEntry, error)
	PreviewURL(ctx context.Context, id string) (string, error)
}

type AdminHandler struct {
	Repo          admin.Repository
	ProjectsStore store.ProjectStore
	Sandbox       AdminSandbox
	Covers        CoverURLSigner
	CoverExpiry   time.Duration
	Notifications *notification.Service
	Metas         auth.ProjectMetaStore
	Now           func() time.Time
}

func NewAdminHandler(repo admin.Repository, projects store.ProjectStore) *AdminHandler {
	return &AdminHandler{Repo: repo, ProjectsStore: projects}
}

type adminProject struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	State        string    `json:"state"`
	Owner        string    `json:"owner"`
	OwnerUserID  int64     `json:"ownerUserId"`
	WorkspaceID  int64     `json:"workspaceId"`
	CoverURL     string    `json:"coverUrl"`
	ReviewStatus string    `json:"reviewStatus"`
	ReviewNote   string    `json:"reviewNote"`
	CreatedAt    time.Time `json:"createdAt"`
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
		coverURL := ""
		if h.Covers != nil && project.CoverS3Path != "" {
			if signed, err := h.Covers.PresignGet(c.Request().Context(), project.CoverS3Path, h.CoverExpiry); err == nil {
				coverURL = signed
			}
		}
		filtered = append(filtered, adminProject{
			ID:           project.ID,
			Name:         project.Name,
			State:        project.State,
			Owner:        owner,
			OwnerUserID:  project.OwnerUserID,
			WorkspaceID:  project.WorkspaceID,
			CoverURL:     coverURL,
			ReviewStatus: project.ReviewStatus,
			ReviewNote:   project.ReviewNote,
			CreatedAt:    project.CreatedAt,
		})
	}
	page := admin.Paginate(filtered, query)
	return response.OK(c, map[string]any{
		"projects": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *AdminHandler) ProjectFiles(c echo.Context) error {
	if h.Sandbox == nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
	}
	files, err := h.Sandbox.ListFiles(c.Request().Context(), c.Param("id"))
	if err != nil {
		return response.Fail(c, http.StatusBadGateway, 50201, "list project files failed")
	}
	return response.OK(c, map[string]any{"files": files})
}

func (h *AdminHandler) ProjectLive(c echo.Context) error {
	if h.Sandbox == nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
	}
	projectID := c.Param("id")
	previewURL, err := h.Sandbox.PreviewURL(c.Request().Context(), projectID)
	if err != nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
	}
	target, err := url.Parse(previewURL)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50003, "invalid preview target")
	}
	wildcard := c.Param("*")
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.URL.Path = target.Path + wildcard
			req.Host = target.Host
		},
		Transport:     previewTransport,
		FlushInterval: -1,
	}
	proxy.ServeHTTP(c.Response(), c.Request())
	return nil
}

type reviewProjectRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (h *AdminHandler) ReviewProject(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req reviewProjectRequest
	if err := c.Bind(&req); err != nil || (req.Status != model.ReviewStatusApproved && req.Status != model.ReviewStatusRejected) {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid review status")
	}
	project, err := h.ProjectsStore.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	now := h.now()
	project.ReviewStatus = req.Status
	project.ReviewNote = req.Note
	project.ReviewedBy = session.Username
	project.ReviewedAt = &now
	if req.Status == model.ReviewStatusRejected {
		project.State = model.ProjectStateDisabled
	} else if project.State == model.ProjectStateDisabled {
		project.State = model.ProjectStateReady
	}
	if err := h.ProjectsStore.Update(c.Request().Context(), project); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "update project failed")
	}
	if h.Metas != nil {
		if meta, err := h.Metas.Get(c.Request().Context(), project.ID); err == nil {
			if req.Status == model.ReviewStatusRejected {
				meta.Status = model.ProjectStateDisabled
			} else {
				meta.Status = ""
			}
			_ = h.Metas.Set(c.Request().Context(), project.ID, meta)
		}
	}
	if req.Status == model.ReviewStatusRejected && h.Notifications != nil && project.OwnerUserID > 0 {
		h.notifyRejected(c.Request().Context(), project)
	}
	return response.OK(c, map[string]any{
		"id": project.ID, "reviewStatus": project.ReviewStatus, "state": project.State,
	})
}

func (h *AdminHandler) notifyRejected(ctx context.Context, project *model.Project) {
	content := fmt.Sprintf("项目「%s」审核未通过，已被禁用。", project.Name)
	if project.ReviewNote != "" {
		content += "原因：" + project.ReviewNote
	}
	view, err := h.Notifications.Create(ctx, notification.CreateInput{
		Title: "项目审核未通过", Content: content, Type: daomodel.NotificationInbox,
		ExpireAt: h.now().Add(30 * 24 * time.Hour), RecipientUserID: project.OwnerUserID,
	}, notification.Creator{Name: "system"})
	if err != nil {
		return
	}
	_, _ = h.Notifications.Publish(ctx, view.ID)
}

func (h *AdminHandler) now() time.Time {
	if h.Now != nil {
		return h.Now().UTC()
	}
	return time.Now().UTC()
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

package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/agentrole"
	"vibebiz/internal/auth"
	"vibebiz/internal/response"
)

type RoleService interface {
	List(ctx context.Context) ([]agentrole.Role, error)
	ListPage(ctx context.Context, query agentrole.ListQuery) (agentrole.Page, error)
	ListActive(ctx context.Context) ([]agentrole.Role, error)
	GetActive(ctx context.Context, id int64) (agentrole.Role, error)
	Create(ctx context.Context, input agentrole.CreateInput, createdBy string) (agentrole.Role, error)
	Update(ctx context.Context, id int64, input agentrole.UpdateInput, updatedBy string) (agentrole.Role, error)
	PresignAvatar(ctx context.Context, filename, contentType string, size int64) (agentrole.Presign, error)
	UploadAvatar(ctx context.Context, contentType string, size int64, body io.Reader) (string, error)
}

type RoleHandler struct {
	Service RoleService
}

func NewRoleHandler(service RoleService) *RoleHandler {
	return &RoleHandler{Service: service}
}

type roleRequest struct {
	RoleCode     string `json:"roleCode"`
	RoleNameEN   string `json:"roleNameEn"`
	RoleNameZH   string `json:"roleNameZh"`
	Description  string `json:"description"`
	SystemPrompt string `json:"systemPrompt"`
	AvatarKey    string `json:"avatarKey"`
	SortOrder    int    `json:"sortOrder"`
	Status       *int8  `json:"status"`
}

type roleAvatarPresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
}

func (h *RoleHandler) List(c echo.Context) error {
	page, err := h.Service.ListPage(c.Request().Context(), roleListQuery(c))
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, map[string]any{
		"roles": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func roleListQuery(c echo.Context) agentrole.ListQuery {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	return agentrole.ListQuery{Keyword: c.QueryParam("keyword"), Page: page, PageSize: pageSize}
}

type publicRoleResponse struct {
	ID          int64  `json:"id"`
	RoleCode    string `json:"roleCode"`
	RoleNameEN  string `json:"roleNameEn"`
	RoleNameZH  string `json:"roleNameZh"`
	Description string `json:"description"`
	AvatarURL   string `json:"avatarUrl"`
}

func (h *RoleHandler) PublicList(c echo.Context) error {
	roles, err := h.Service.ListActive(c.Request().Context())
	if err != nil {
		return roleError(c, err)
	}
	out := make([]publicRoleResponse, 0, len(roles))
	for _, role := range roles {
		out = append(out, publicRoleResponse{
			ID: role.ID, RoleCode: role.RoleCode, RoleNameEN: role.RoleNameEN,
			RoleNameZH: role.RoleNameZH, Description: role.Description, AvatarURL: role.AvatarURL,
		})
	}
	return response.OK(c, map[string]any{"roles": out})
}

func (h *RoleHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req roleRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	role, err := h.Service.Create(c.Request().Context(), agentrole.CreateInput{
		RoleCode: req.RoleCode, RoleNameEN: req.RoleNameEN, RoleNameZH: req.RoleNameZH,
		Description: req.Description, SystemPrompt: req.SystemPrompt, AvatarKey: req.AvatarKey,
		Status: status, SortOrder: req.SortOrder,
	}, session.Username)
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, role)
}

func (h *RoleHandler) Update(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid role id")
	}
	var req roleRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	status := int8(1)
	if req.Status != nil {
		status = *req.Status
	}
	role, err := h.Service.Update(c.Request().Context(), id, agentrole.UpdateInput{
		Description: req.Description, SystemPrompt: req.SystemPrompt, Status: status,
	}, session.Username)
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, role)
}

func (h *RoleHandler) PresignAvatar(c echo.Context) error {
	var req roleAvatarPresignRequest
	if err := c.Bind(&req); err != nil || req.Filename == "" || req.ContentType == "" || req.Size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	result, err := h.Service.PresignAvatar(c.Request().Context(), req.Filename, req.ContentType, req.Size)
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, result)
}

func (h *RoleHandler) UploadAvatar(c echo.Context) error {
	contentType := strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType))
	size := c.Request().ContentLength
	if contentType == "" || size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid avatar")
	}
	key, err := h.Service.UploadAvatar(c.Request().Context(), contentType, size, io.LimitReader(c.Request().Body, size))
	if err != nil {
		return roleError(c, err)
	}
	return response.OK(c, map[string]any{"key": key})
}

func roleError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, agentrole.ErrInvalid), errors.Is(err, agentrole.ErrAvatarInvalid), errors.Is(err, agentrole.ErrAvatarNotFound):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid role")
	case errors.Is(err, agentrole.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "role not found")
	case errors.Is(err, agentrole.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "role code already exists")
	case errors.Is(err, agentrole.ErrStorageUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "role avatar storage not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "role operation failed")
	}
}

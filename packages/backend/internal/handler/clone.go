package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/client"
	"vibebiz/internal/clone"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type CloneService interface {
	Create(ctx context.Context, workspaceID, userID int64, input clone.CreateInput) (clone.View, error)
	List(ctx context.Context, workspaceIDs []int64) ([]clone.View, error)
	Get(ctx context.Context, workspaceIDs []int64, projectID string) (clone.View, error)
	Delete(ctx context.Context, workspaceIDs []int64, projectID string) error
	ListAssets(ctx context.Context, workspaceIDs []int64, projectID string) ([]clone.AssetView, error)
	AddAsset(ctx context.Context, workspaceID, userID int64, projectID, resourceID, role string) (clone.AssetView, error)
	DeleteAsset(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) error
	SetCover(ctx context.Context, workspaceID, userID int64, projectID, resourceID string) (clone.AssetView, error)
}

type CloneTestRunner interface {
	RunCloneTests(ctx context.Context, projectID string) (client.CloneTestResult, error)
}

type CloneHandler struct {
	Service CloneService
	Tests   CloneTestRunner
	Obs     *observability.Provider
}

func NewCloneHandler(service CloneService, obs *observability.Provider) *CloneHandler {
	return &CloneHandler{Service: service, Obs: obs}
}

type cloneAssetsInput struct {
	CoverResourceID    string   `json:"coverResourceId"`
	GalleryResourceIDs []string `json:"galleryResourceIds"`
	UnitTestResourceID string   `json:"unitTestResourceId"`
	DocsResourceID     string   `json:"docsResourceId"`
}

type createCloneRequest struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	SourceURL   string           `json:"sourceUrl"`
	Assets      cloneAssetsInput `json:"assets"`
}

type addCloneAssetRequest struct {
	ResourceID string `json:"resourceId"`
	Role       string `json:"role"`
}

func (h *CloneHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req createCloneRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	view, err := h.Service.Create(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, clone.CreateInput{
		Name: req.Name, Description: req.Description, SourceURL: req.SourceURL,
		CoverResourceID: req.Assets.CoverResourceID, GalleryResourceIDs: req.Assets.GalleryResourceIDs,
		UnitTestResourceID: req.Assets.UnitTestResourceID, DocsResourceID: req.Assets.DocsResourceID,
	})
	if err != nil {
		return h.fail(c, "clone.create", "", err)
	}
	return response.Created(c, view)
}

func (h *CloneHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	items, err := h.Service.List(c.Request().Context(), []int64{session.CurrentWorkspaceID})
	if err != nil {
		return h.fail(c, "clone.list", "", err)
	}
	return response.OK(c, map[string]any{"clones": items})
}

func (h *CloneHandler) Get(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	view, err := h.Service.Get(c.Request().Context(), session.WorkspaceIDs, c.Param("id"))
	if err != nil {
		return h.fail(c, "clone.get", c.Param("id"), err)
	}
	return response.OK(c, view)
}

func (h *CloneHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if err := h.Service.Delete(c.Request().Context(), session.WorkspaceIDs, c.Param("id")); err != nil {
		return h.fail(c, "clone.delete", c.Param("id"), err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *CloneHandler) ListAssets(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	assets, err := h.Service.ListAssets(c.Request().Context(), session.WorkspaceIDs, c.Param("id"))
	if err != nil {
		return h.fail(c, "clone.assets.list", c.Param("id"), err)
	}
	return response.OK(c, map[string]any{"assets": assets})
}

func (h *CloneHandler) AddAsset(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req addCloneAssetRequest
	if err := c.Bind(&req); err != nil || req.ResourceID == "" || req.Role == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "resourceId and role are required")
	}
	asset, err := h.Service.AddAsset(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, c.Param("id"), req.ResourceID, req.Role)
	if err != nil {
		return h.fail(c, "clone.assets.add", c.Param("id"), err)
	}
	return response.OK(c, asset)
}

func (h *CloneHandler) DeleteAsset(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if err := h.Service.DeleteAsset(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, c.Param("id"), c.Param("resourceId")); err != nil {
		return h.fail(c, "clone.assets.delete", c.Param("id"), err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *CloneHandler) SetCover(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	asset, err := h.Service.SetCover(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, c.Param("id"), c.Param("resourceId"))
	if err != nil {
		return h.fail(c, "clone.assets.cover", c.Param("id"), err)
	}
	return response.OK(c, asset)
}

func (h *CloneHandler) RunTests(c echo.Context) error {
	if _, ok := auth.SessionFrom(c); !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	if h.Tests == nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50303, "clone test runner unavailable")
	}
	result, err := h.Tests.RunCloneTests(c.Request().Context(), c.Param("id"))
	if err != nil {
		return failFromSandbox(c, err, "run clone tests failed")
	}
	return response.OK(c, result)
}

func (h *CloneHandler) fail(c echo.Context, action, projectID string, err error) error {
	if h.Obs != nil {
		h.Obs.Log(c.Request().Context(), slog.LevelError, action,
			"project_id", projectID, "status", "error", "error", err.Error())
	}
	switch {
	case errors.Is(err, clone.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid clone request")
	case errors.Is(err, clone.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "clone project not found")
	case errors.Is(err, clone.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "clone project conflict")
	case errors.Is(err, clone.ErrRolesUnavailable):
		return response.Fail(c, http.StatusBadRequest, 40010, "clone roles unavailable")
	case errors.Is(err, clone.ErrSandboxUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50302, "sandbox unavailable")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50005, "clone operation failed")
	}
}

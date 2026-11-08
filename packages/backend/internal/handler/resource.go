package handler

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/observability"
	"vibebiz/internal/resource"
	"vibebiz/internal/response"
	"vibebiz/internal/usage"
)

type ResourceHandler struct {
	Service *resource.Service
	Obs     *observability.Provider
}

func NewResourceHandler(service *resource.Service, obs *observability.Provider) *ResourceHandler {
	return &ResourceHandler{Service: service, Obs: obs}
}

type resourcePresignRequest struct {
	Filename    string `json:"filename"`
	ContentType string `json:"contentType"`
	Size        int64  `json:"size"`
	Purpose     string `json:"purpose"`
}

type resourceAdoptRequest struct {
	ResourceID string `json:"resourceId"`
}

func (h *ResourceHandler) Presign(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req resourcePresignRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	result, err := h.Service.Presign(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, resource.PresignInput{
		Filename: req.Filename, ContentType: req.ContentType, Size: req.Size, Purpose: req.Purpose,
	})
	if err != nil {
		h.log(c, "resource.presign", "", err)
		return resourceError(c, err)
	}
	return response.OK(c, result)
}

func (h *ResourceHandler) DeleteStaging(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	resourceID := c.Param("resourceId")
	if err := h.Service.DeleteStaging(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, resourceID); err != nil {
		h.log(c, "resource.delete_staging", resourceID, err)
		return resourceError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *ResourceHandler) UploadStaging(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	resourceID := c.Param("resourceId")
	limited := http.MaxBytesReader(c.Response(), c.Request().Body, resource.MaxChannelBuildFileBytes)
	tmp, err := os.CreateTemp("", "vibebiz-resource-upload-*")
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "resource upload failed")
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()
	size, err := io.Copy(tmp, limited)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, "resource file is too large")
		}
		return response.Fail(c, http.StatusBadRequest, 40006, "invalid request body")
	}
	if size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40006, "invalid request body")
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "resource upload failed")
	}
	if err := h.Service.UploadStaging(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, resourceID, tmp, size, c.Request().Header.Get(echo.HeaderContentType)); err != nil {
		h.log(c, "resource.upload", resourceID, err)
		return resourceError(c, err)
	}
	return response.OK(c, map[string]any{"uploaded": true})
}

func (h *ResourceHandler) Adopt(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req resourceAdoptRequest
	if err := c.Bind(&req); err != nil || req.ResourceID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "resourceId is required")
	}
	projectID := c.Param("id")
	item, err := h.Service.Adopt(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, projectID, req.ResourceID)
	if err != nil {
		h.log(c, "resource.adopt", req.ResourceID, err)
		return resourceError(c, err)
	}
	return response.OK(c, item)
}

func (h *ResourceHandler) List(c echo.Context) error {
	result, err := h.Service.List(c.Request().Context(), c.Param("id"))
	if err != nil {
		h.log(c, "resource.list", "", err)
		return resourceError(c, err)
	}
	return response.OK(c, result)
}

func (h *ResourceHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	projectID, resourceID := c.Param("id"), c.Param("resourceId")
	if err := h.Service.Delete(c.Request().Context(), session.CurrentWorkspaceID, session.UserID, projectID, resourceID); err != nil {
		h.log(c, "resource.delete", resourceID, err)
		return resourceError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *ResourceHandler) log(c echo.Context, action, resourceID string, err error) {
	if h.Obs == nil {
		return
	}
	h.Obs.Log(c.Request().Context(), slog.LevelError, action,
		"project_id", c.Param("id"), "resource_id", resourceID, "status", "error", "error", err.Error())
}

func resourceError(c echo.Context, err error) error {
	var exceeded *usage.QuotaExceededError
	if errors.As(err, &exceeded) {
		return response.FailWithResult(c, http.StatusTooManyRequests, quotaCode(exceeded.QuotaType), "daily quota exceeded",
			map[string]any{"quotaType": exceeded.QuotaType, "used": exceeded.Used, "limit": exceeded.Limit})
	}
	switch {
	case errors.Is(err, resource.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40404, "resource not found")
	case errors.Is(err, resource.ErrInvalidFile):
		return response.Fail(c, http.StatusBadRequest, 40008, "unsupported resource file")
	case errors.Is(err, resource.ErrTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, "resource file is too large")
	case errors.Is(err, resource.ErrProjectFull):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41302, "project resource quota exceeded")
	case errors.Is(err, resource.ErrUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50303, "resource storage unavailable")
	case errors.Is(err, resource.ErrNotUploadable):
		return response.Fail(c, http.StatusConflict, 40902, "resource is not uploadable")
	default:
		return response.Fail(c, http.StatusBadGateway, 50203, "resource operation failed")
	}
}

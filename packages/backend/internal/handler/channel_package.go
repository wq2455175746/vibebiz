package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/channelpkg"
	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
	"vibebiz/internal/dao"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type ChannelPackageHandler struct {
	Service  *channelpkg.Service
	Projects store.ProjectStore
}

func NewChannelPackageHandler(service *channelpkg.Service, projects store.ProjectStore) *ChannelPackageHandler {
	return &ChannelPackageHandler{Service: service, Projects: projects}
}

type channelPackageRequest struct {
	Channel    string `json:"channel"`
	SourceType string `json:"sourceType"`
	ResourceID string `json:"resourceId"`
}

type channelPackageResponse struct {
	ID            string   `json:"id"`
	Channel       string   `json:"channel"`
	SourceType    string   `json:"sourceType"`
	Status        string   `json:"status"`
	Version       string   `json:"version"`
	FileSizeBytes int64    `json:"fileSizeBytes"`
	Level         string   `json:"level"`
	Warnings      []string `json:"warnings"`
	ErrorMessage  string   `json:"errorMessage"`
	CreatedAt     string   `json:"createdAt"`
	DownloadURL   string   `json:"downloadUrl"`
	GuideURL      string   `json:"guideUrl"`
}

func (h *ChannelPackageHandler) Generate(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, ok := h.loadProject(c, session)
	if !ok {
		return nil
	}
	var req channelPackageRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if h.Service == nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
	item, err := h.Service.Generate(c.Request().Context(), project, channelpkg.GenerateInput{
		Channel: req.Channel, SourceType: req.SourceType, ResourceID: req.ResourceID,
	})
	if err != nil {
		return channelPackageError(c, err)
	}
	return response.OK(c, map[string]any{"package": toChannelPackageResponse(item)})
}

func (h *ChannelPackageHandler) List(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, ok := h.loadProject(c, session)
	if !ok {
		return nil
	}
	if h.Service == nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
	items, err := h.Service.List(c.Request().Context(), project.ID)
	if err != nil {
		return channelPackageError(c, err)
	}
	out := make([]channelPackageResponse, 0, len(items))
	for i := range items {
		out = append(out, toChannelPackageResponse(&items[i]))
	}
	return response.OK(c, map[string]any{"packages": out})
}

func (h *ChannelPackageHandler) Download(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, ok := h.loadProject(c, session)
	if !ok {
		return nil
	}
	if h.Service == nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
	item, err := h.Service.Get(c.Request().Context(), c.Param("packageId"))
	if err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40409, "channel package not found")
		}
		return channelPackageError(c, err)
	}
	if item.ProjectPublicID != project.ID || item.Status != daomodel.ChannelPackageStatusReady {
		return response.Fail(c, http.StatusNotFound, 40409, "channel package not found")
	}
	target, err := confinedPackagePath(h.Service.PackagesDir(), project.ID, item)
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40409, "channel package not found")
	}
	file, err := os.Open(target)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return response.Fail(c, http.StatusNotFound, 40409, "channel package not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
	defer func() { _ = file.Close() }()
	version := item.Version
	if version == "" {
		version = item.PublicID
	}
	filename := fmt.Sprintf("%s-%s-%s.zip", project.ID, item.Channel, version)
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Stream(http.StatusOK, "application/zip", file)
}

func (h *ChannelPackageHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, ok := h.loadProject(c, session)
	if !ok {
		return nil
	}
	if h.Service == nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
	packageID := c.Param("packageId")
	if err := h.Service.Delete(c.Request().Context(), project.ID, packageID); err != nil {
		if errors.Is(err, dao.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40409, "channel package not found")
		}
		return channelPackageError(c, err)
	}
	return response.OK(c, map[string]string{"id": packageID})
}

func (h *ChannelPackageHandler) Guides(c echo.Context) error {
	return response.OK(c, map[string]any{"guides": guides.List()})
}

func (h *ChannelPackageHandler) loadProject(c echo.Context, session *auth.Session) (*model.Project, bool) {
	if h.Projects == nil {
		_ = response.Fail(c, http.StatusNotFound, 40401, "project not found")
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
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) || project.IsHidden {
		_ = response.Fail(c, http.StatusNotFound, 40401, "project not found")
		return nil, false
	}
	return project, true
}

func toChannelPackageResponse(item *daomodel.ProjectChannelPackage) channelPackageResponse {
	var manifest core.Manifest
	if len(item.Manifest) > 0 {
		_ = json.Unmarshal(item.Manifest, &manifest)
	}
	warnings := manifest.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return channelPackageResponse{
		ID:            item.PublicID,
		Channel:       item.Channel,
		SourceType:    item.SourceType,
		Status:        item.Status,
		Version:       item.Version,
		FileSizeBytes: item.FileSizeBytes,
		Level:         manifest.Level,
		Warnings:      warnings,
		ErrorMessage:  item.ErrorMessage,
		CreatedAt:     item.CreatedAt.UTC().Format(time.RFC3339),
		DownloadURL:   "/cp/api/v1/projects/" + item.ProjectPublicID + "/channel-packages/" + item.PublicID + "/download",
		GuideURL:      "/settings/channel-guides?channel=" + item.Channel,
	}
}

func confinedPackagePath(packagesDir, projectID string, item *daomodel.ProjectChannelPackage) (string, error) {
	if packagesDir == "" || strings.TrimSpace(item.FilePath) == "" {
		return "", channelpkg.ErrInvalidSource
	}
	relative := filepath.FromSlash(item.FilePath)
	if filepath.IsAbs(relative) || filepath.VolumeName(relative) != "" {
		return "", channelpkg.ErrInvalidSource
	}
	projectRoot := filepath.Join(packagesDir, projectID)
	full := filepath.Join(packagesDir, relative)
	inside, err := filepath.Rel(projectRoot, full)
	if err != nil || inside == "." || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) || filepath.IsAbs(inside) {
		return "", channelpkg.ErrInvalidSource
	}
	return full, nil
}

func channelPackageError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, channelpkg.ErrInvalidChannel), errors.Is(err, channelpkg.ErrInvalidSourceType):
		return response.Fail(c, http.StatusBadRequest, 40001, err.Error())
	case errors.Is(err, channelpkg.ErrUnsupported):
		return response.Fail(c, http.StatusConflict, 40901, err.Error())
	case errors.Is(err, channelpkg.ErrSourceNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "channel source not found")
	case errors.Is(err, channelpkg.ErrPackageTooLarge), errors.Is(err, core.ErrArchiveTooLarge), errors.Is(err, core.ErrUnsafeArchive):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, err.Error())
	case errors.Is(err, channelpkg.ErrBusy):
		return response.Fail(c, http.StatusTooManyRequests, 42901, err.Error())
	default:
		return response.Fail(c, http.StatusInternalServerError, 50014, "channel package failed")
	}
}

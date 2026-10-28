package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type FileClient interface {
	ListFiles(ctx context.Context, id string) ([]client.FileEntry, error)
	ReadFile(ctx context.Context, id, path string) (*client.FileContent, error)
	WriteFile(ctx context.Context, id, path, content, baseVersion string) (*client.WriteResult, error)
}

type FileHandler struct {
	Sandbox FileClient
	Obs     *observability.Provider
}

func NewFileHandler(sandbox FileClient, obs *observability.Provider) *FileHandler {
	return &FileHandler{Sandbox: sandbox, Obs: obs}
}

func (h *FileHandler) List(c echo.Context) error {
	projectID := c.Param("id")
	ctx := c.Request().Context()
	files, err := h.Sandbox.ListFiles(ctx, projectID)
	if err != nil {
		if h.Obs != nil {
			h.Obs.Log(ctx, slog.LevelError, "file.list", "project_id", projectID, "status", "error", "error", err.Error())
		}
		return failFromSandbox(c, err, "list files failed")
	}
	return response.OK(c, map[string]any{"files": files})
}

func (h *FileHandler) Read(c echo.Context) error {
	path := strings.TrimSpace(c.QueryParam("path"))
	if path == "" {
		return response.Fail(c, http.StatusBadRequest, 40003, "path is required")
	}
	projectID := c.Param("id")
	ctx := c.Request().Context()
	file, err := h.Sandbox.ReadFile(ctx, projectID, path)
	if h.Obs != nil {
		if err != nil {
			h.Obs.Log(ctx, slog.LevelError, "file.read", "project_id", projectID, "path", path, "status", "error", "error", err.Error())
		} else {
			h.Obs.Log(ctx, slog.LevelInfo, "file.read", "project_id", projectID, "path", path, "status", "ok", "version", file.Version)
		}
	}
	if err != nil {
		return failFromSandbox(c, err, "read file failed")
	}
	return response.OK(c, file)
}

type writeFileRequest struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	BaseVersion string `json:"baseVersion"`
}

func (h *FileHandler) Write(c echo.Context) error {
	var req writeFileRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40004, "invalid request body")
	}
	if strings.TrimSpace(req.Path) == "" {
		return response.Fail(c, http.StatusBadRequest, 40005, "path is required")
	}

	projectID := c.Param("id")
	ctx := c.Request().Context()
	result, err := h.Sandbox.WriteFile(ctx, projectID, req.Path, req.Content, req.BaseVersion)
	if h.Obs != nil {
		if err != nil {
			h.Obs.Log(ctx, slog.LevelError, "file.write", "project_id", projectID, "path", req.Path, "status", "error", "error", err.Error())
		} else {
			h.Obs.Log(ctx, slog.LevelInfo, "file.write", "project_id", projectID, "path", req.Path, "status", "ok", "version", result.Version)
		}
	}
	if err != nil {
		return failFromSandbox(c, err, "write file failed")
	}
	return response.OK(c, result)
}

func failFromSandbox(c echo.Context, err error, fallback string) error {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		status := apiErr.Status
		if status == 0 {
			status = http.StatusBadGateway
		}
		if status >= http.StatusInternalServerError {
			return response.Fail(c, http.StatusBadGateway, 50202, fallback)
		}
		code := apiErr.Code
		if code == 0 {
			code = status
		}
		return response.Fail(c, status, code, apiErr.Message)
	}
	return response.Fail(c, http.StatusBadGateway, 50202, fallback)
}

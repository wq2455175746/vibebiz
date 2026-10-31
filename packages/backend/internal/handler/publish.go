package handler

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/publish"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type PublishHandler struct {
	Service       *publish.Service
	Projects      store.ProjectStore
	PublicBaseURL string
}

func NewPublishHandler(service *publish.Service, projects store.ProjectStore, publicBaseURL string) *PublishHandler {
	return &PublishHandler{Service: service, Projects: projects, PublicBaseURL: publicBaseURL}
}

type publishRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

type publishResponse struct {
	URL         string `json:"url"`
	Slug        string `json:"slug"`
	PublishedAt string `json:"publishedAt"`
}

func (h *PublishHandler) Publish(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	project, err := h.Projects.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50002, "get project failed")
	}
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	var req publishRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if strings.TrimSpace(req.Title) == "" {
		req.Title = project.Name
	}
	result, err := h.Service.Publish(c.Request().Context(), project.ID, publish.Input{
		Title: req.Title, Description: req.Description, Tags: req.Tags,
	})
	if err != nil {
		return publishError(c, err)
	}
	return response.OK(c, publishResponse{
		URL:         h.publicBase(c) + "/p/" + result.Slug + "/",
		Slug:        result.Slug,
		PublishedAt: result.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *PublishHandler) Preview(c echo.Context) error {
	path, err := h.Service.FilePath(c.Param("slug"), c.Param("*"))
	if err != nil {
		return publishError(c, err)
	}
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set(
		"Content-Security-Policy",
		"sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads",
	)
	return c.File(path)
}

func (h *PublishHandler) publicBase(c echo.Context) string {
	if base := strings.TrimRight(strings.TrimSpace(h.PublicBaseURL), "/"); base != "" {
		return base
	}
	proto := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Proto"))
	if proto == "" {
		proto = "http"
		if c.Request().TLS != nil {
			proto = "https"
		}
	}
	host := strings.TrimSpace(c.Request().Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = c.Request().Host
	}
	return proto + "://" + host
}

func publishError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, publish.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40015, "publish source has no index.html or app.html")
	case errors.Is(err, publish.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40407, "published file not found")
	case errors.Is(err, publish.ErrTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, "published app is too large")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50010, "publish failed")
	}
}

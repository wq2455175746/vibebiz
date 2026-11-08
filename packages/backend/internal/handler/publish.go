package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/leaderboard"
	"vibebiz/internal/model"
	"vibebiz/internal/publish"
	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

type PublishHandler struct {
	Service       *publish.Service
	Projects      store.ProjectStore
	PublicBaseURL string
	Covers        CoverURLSigner
	CoverExpiry   time.Duration
	Leaderboard   *leaderboard.Service
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

type publishedProjectResponse struct {
	ProjectID   string   `json:"projectId"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	CoverURL    string   `json:"coverUrl,omitempty"`
	PublishedAt string   `json:"publishedAt"`
	URL         string   `json:"url"`
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
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) || project.IsHidden {
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
	if h.Leaderboard != nil {
		_ = h.Leaderboard.UpsertMeta(c.Request().Context(), project, req.Title)
	}
	return response.OK(c, publishResponse{
		URL:         h.publicBase(c) + "/p/" + result.Slug + "/",
		Slug:        result.Slug,
		PublishedAt: result.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *PublishHandler) ListPublished(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	items, err := h.Service.List(c.Request().Context())
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50011, "list published projects failed")
	}
	allowed := make(map[string]*model.Project)
	for _, project := range h.Projects.ListByWorkspaces(c.Request().Context(), session.WorkspaceIDs) {
		if project.IsHidden {
			continue
		}
		allowed[project.ID] = project
	}
	out := make([]publishedProjectResponse, 0, len(items))
	for _, item := range items {
		project, exists := allowed[item.ProjectID]
		if !exists {
			continue
		}
		name := item.Title
		if name == "" {
			name = project.Name
		}
		tags := item.Tags
		if tags == nil {
			tags = []string{}
		}
		out = append(out, publishedProjectResponse{
			ProjectID:   item.ProjectID,
			Name:        name,
			Description: item.Description,
			Tags:        tags,
			CoverURL:    h.coverURL(c.Request().Context(), project),
			PublishedAt: item.PublishedAt.UTC().Format(time.RFC3339),
			URL:         h.publicBase(c) + "/p/" + item.ProjectID + "/",
		})
	}
	return response.OK(c, map[string]any{"projects": out})
}

func (h *PublishHandler) coverURL(ctx context.Context, project *model.Project) string {
	if project == nil {
		return ""
	}
	if project.CoverS3Path == "" || h.Covers == nil {
		return project.CoverURL
	}
	expiry := h.CoverExpiry
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	signed, err := h.Covers.PresignGet(ctx, project.CoverS3Path, expiry)
	if err != nil {
		return project.CoverURL
	}
	return signed
}

func (h *PublishHandler) DeletePublished(c echo.Context) error {
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
	if !containsInt64(session.WorkspaceIDs, project.WorkspaceID) || project.IsHidden {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	if err := h.Service.Delete(c.Request().Context(), project.ID); err != nil {
		switch {
		case errors.Is(err, publish.ErrNotFound):
			return response.Fail(c, http.StatusNotFound, 40408, "published project not found")
		case errors.Is(err, publish.ErrInvalid):
			return response.Fail(c, http.StatusBadRequest, 40015, "invalid published project")
		default:
			return response.Fail(c, http.StatusInternalServerError, 50012, "delete published project failed")
		}
	}
	if h.Leaderboard != nil {
		_ = h.Leaderboard.Delete(c.Request().Context(), project.ID)
	}
	return response.OK(c, map[string]string{"id": project.ID})
}

func (h *PublishHandler) Preview(c echo.Context) error {
	if project, err := h.Projects.Get(c.Request().Context(), c.Param("slug")); err == nil && project.IsHidden {
		return response.Fail(c, http.StatusNotFound, 40401, "project not found")
	}
	path, err := h.Service.FilePath(c.Param("slug"), c.Param("*"))
	if err != nil {
		return publishError(c, err)
	}
	if h.Leaderboard != nil && isEntryDocument(c.Param("*")) {
		_ = h.Leaderboard.RecordView(c.Request().Context(), c.Param("slug"))
	}
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("X-Content-Type-Options", "nosniff")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set(
		"Content-Security-Policy",
		"sandbox allow-scripts allow-forms allow-modals allow-popups allow-downloads",
	)
	return c.File(path)
}

func isEntryDocument(wildcard string) bool {
	path := strings.Trim(strings.TrimSpace(wildcard), "/")
	return path == "" || path == "index.html"
}

func (h *PublishHandler) TrackClick(c echo.Context) error {
	slug := c.Param("slug")
	if _, err := h.Service.FilePath(slug, ""); err != nil {
		return publishError(c, err)
	}
	if h.Leaderboard != nil {
		_ = h.Leaderboard.RecordClick(c.Request().Context(), slug)
	}
	return c.NoContent(http.StatusNoContent)
}

func (h *PublishHandler) PublishedStats(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	ctx := c.Request().Context()
	published, err := h.Service.List(ctx)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50011, "list published projects failed")
	}
	allowed := make(map[string]*model.Project)
	for _, project := range h.Projects.ListByWorkspaces(ctx, session.WorkspaceIDs) {
		if project.IsHidden {
			continue
		}
		allowed[project.ID] = project
	}
	inputs := make([]leaderboard.StatInput, 0, len(published))
	for _, item := range published {
		project, exists := allowed[item.ProjectID]
		if !exists {
			continue
		}
		name := item.Title
		if name == "" {
			name = project.Name
		}
		inputs = append(inputs, leaderboard.StatInput{ProjectID: item.ProjectID, Name: name})
	}
	stats := make([]leaderboard.ProjectStat, 0, len(inputs))
	if h.Leaderboard != nil {
		stats, err = h.Leaderboard.Stats(ctx, inputs)
		if err != nil {
			return response.Fail(c, http.StatusInternalServerError, 50015, "load published stats failed")
		}
	} else {
		for _, input := range inputs {
			stats = append(stats, leaderboard.ProjectStat{ProjectID: input.ProjectID, Name: input.Name})
		}
	}
	var totals leaderboard.ProjectStat
	for _, item := range stats {
		totals.Exposure += item.Exposure
		totals.Clicks += item.Clicks
	}
	return response.OK(c, map[string]any{
		"exposure": totals.Exposure, "clicks": totals.Clicks,
		"dwellSeconds": 0, "spend": 0, "projects": stats,
	})
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

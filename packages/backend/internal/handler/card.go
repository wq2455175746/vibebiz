package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/businesscard"
	"vibebiz/internal/response"
)

type CardService interface {
	GetMine(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	UpdateProfile(ctx context.Context, userID int64, workspaceIDs []int64, headline string, tags []string) (*businesscard.Card, error)
	AddProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	RemoveProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	SetFeaturedProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	UploadFeatured(ctx context.Context, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*businesscard.Card, error)
	ClearFeatured(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	Publish(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	Discover(ctx context.Context, page, pageSize int) (*businesscard.DiscoverResult, error)
	GetPublic(ctx context.Context, publicID string) (*businesscard.Card, error)
	PublicProject(ctx context.Context, projectID string) (*businesscard.PublicProject, error)
	AdminList(ctx context.Context, keyword string, page, pageSize int) (*businesscard.AdminListResult, error)
	AdminSetHidden(ctx context.Context, publicID string, hidden bool) (*businesscard.AdminRow, error)
	AdminDelete(ctx context.Context, publicID string) error
}

type CardHandler struct {
	Service       CardService
	PublicBaseURL string
}

func NewCardHandler(service CardService, publicBaseURL string) *CardHandler {
	return &CardHandler{Service: service, PublicBaseURL: publicBaseURL}
}

func (h *CardHandler) GetMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	card, err := h.Service.GetMine(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) UpdateMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req struct {
		Headline string   `json:"headline"`
		Tags     []string `json:"tags"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.UpdateProfile(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.Headline, req.Tags)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) AddItem(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req struct {
		ProjectID string `json:"projectId"`
	}
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.ProjectID) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.AddProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.ProjectID)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) RemoveItem(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	card, err := h.Service.RemoveProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, c.Param("projectId"))
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) SetFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req struct {
		Kind      string `json:"kind"`
		ProjectID string `json:"projectId"`
	}
	if err := c.Bind(&req); err != nil || req.Kind != "project" || strings.TrimSpace(req.ProjectID) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.SetFeaturedProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.ProjectID)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) UploadFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	contentType := strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType))
	name := c.Request().Header.Get("X-File-Name")
	if decoded, err := url.QueryUnescape(name); err == nil {
		name = decoded
	}
	if strings.TrimSpace(name) == "" {
		name = "featured"
	}
	size := c.Request().ContentLength
	if size <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.UploadFeatured(c.Request().Context(), session.UserID, session.WorkspaceIDs, contentType, name, size, c.Request().Body)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) ClearFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	card, err := h.Service.ClearFeatured(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) Publish(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	card, err := h.Service.Publish(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil {
		return cardError(c, err)
	}
	return h.okCard(c, card)
}

func (h *CardHandler) Discover(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.Discover(c.Request().Context(), page, pageSize)
	if err != nil {
		return cardError(c, err)
	}
	if result != nil {
		for index := range result.Cards {
			h.normalizeItems(c, result.Cards[index].Items)
		}
	}
	return response.OK(c, result)
}

func (h *CardHandler) PublicCard(c echo.Context) error {
	card, err := h.Service.GetPublic(c.Request().Context(), c.Param("id"))
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40410, "card not found")
	}
	return h.okCard(c, card)
}

func (h *CardHandler) PublicProject(c echo.Context) error {
	project, err := h.Service.PublicProject(c.Request().Context(), c.Param("projectId"))
	if err != nil {
		return response.Fail(c, http.StatusNotFound, 40411, "project not found")
	}
	if project != nil {
		items := []businesscard.CardItem{project.Item}
		h.normalizeItems(c, items)
		project.Item = items[0]
	}
	return response.OK(c, project)
}

func (h *CardHandler) AdminList(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.AdminList(c.Request().Context(), c.QueryParam("keyword"), page, pageSize)
	if err != nil {
		return cardError(c, err)
	}
	return response.OK(c, result)
}

func (h *CardHandler) AdminHide(c echo.Context) error {
	var req struct {
		Hidden bool `json:"hidden"`
	}
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	row, err := h.Service.AdminSetHidden(c.Request().Context(), c.Param("id"), req.Hidden)
	if err != nil {
		return cardError(c, err)
	}
	return response.OK(c, map[string]any{"id": row.ID, "hidden": row.Hidden})
}

func (h *CardHandler) AdminDelete(c echo.Context) error {
	if err := h.Service.AdminDelete(c.Request().Context(), c.Param("id")); err != nil {
		return cardError(c, err)
	}
	return response.OK(c, map[string]string{"id": c.Param("id")})
}

func (h *CardHandler) okCard(c echo.Context, card *businesscard.Card) error {
	if card != nil {
		h.normalizeItems(c, card.Items)
	}
	return response.OK(c, card)
}

func (h *CardHandler) normalizeItems(c echo.Context, items []businesscard.CardItem) {
	base := leaderboardPublicBase(c, h.PublicBaseURL)
	for index := range items {
		if strings.HasPrefix(items[index].ShareURL, "/") {
			items[index].ShareURL = base + items[index].ShareURL
		}
	}
}

func cardError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, businesscard.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "card not found")
	case errors.Is(err, businesscard.ErrInvalidProject):
		return response.Fail(c, http.StatusBadRequest, 40016, "project cannot be added to card")
	case errors.Is(err, businesscard.ErrItemLimit), errors.Is(err, businesscard.ErrItemDuplicate),
		errors.Is(err, businesscard.ErrFeaturedNotFound), errors.Is(err, businesscard.ErrNoItems),
		errors.Is(err, businesscard.ErrInvalidInput), errors.Is(err, businesscard.ErrInvalidFile):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid card request")
	case errors.Is(err, businesscard.ErrFileTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41302, "featured file is too large")
	case errors.Is(err, businesscard.ErrStorageUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "card storage is not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "business card request failed")
	}
}

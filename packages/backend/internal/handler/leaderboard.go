package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/leaderboard"
	"vibebiz/internal/response"
)

type LeaderboardHandler struct {
	Service       *leaderboard.Service
	PublicBaseURL string
}

func NewLeaderboardHandler(service *leaderboard.Service, publicBaseURL string) *LeaderboardHandler {
	return &LeaderboardHandler{Service: service, PublicBaseURL: publicBaseURL}
}

type publicLeaderboardItem struct {
	Rank      int    `json:"rank"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Author    string `json:"author"`
	Visits    int64  `json:"visits"`
	CoverURL  string `json:"coverUrl"`
	URL       string `json:"url"`
}

type adminLeaderboardItem struct {
	Rank        int    `json:"rank"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Author      string `json:"author"`
	Visits      int64  `json:"visits"`
	Exposure    int64  `json:"exposure"`
	Clicks      int64  `json:"clicks"`
	Hidden      bool   `json:"hidden"`
	PublishedAt string `json:"publishedAt"`
}

func (h *LeaderboardHandler) ListPublic(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	items, err := h.Service.ListPublic(c.Request().Context(), limit)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50013, "load leaderboard failed")
	}
	base := leaderboardPublicBase(c, h.PublicBaseURL)
	out := make([]publicLeaderboardItem, 0, len(items))
	for _, item := range items {
		out = append(out, publicLeaderboardItem{
			Rank: item.Rank, ProjectID: item.ProjectID, Name: item.Name,
			Author: item.Author, Visits: item.Visits, CoverURL: item.CoverURL,
			URL: base + "/p/" + item.ProjectID + "/",
		})
	}
	return response.OK(c, map[string]any{"items": out})
}

func (h *LeaderboardHandler) ListAdmin(c echo.Context) error {
	rows, err := h.Service.ListAdmin(c.Request().Context())
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50013, "load leaderboard failed")
	}
	items := make([]adminLeaderboardItem, 0, len(rows))
	for index, row := range rows {
		items = append(items, adminLeaderboardItem{
			Rank: index + 1, ProjectID: row.ProjectID, Name: row.Name, Author: row.Author,
			Visits: row.Visits, Exposure: row.Exposure, Clicks: row.Clicks,
			Hidden: row.Hidden, PublishedAt: row.PublishedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	query := adminListQuery(c)
	filtered := make([]adminLeaderboardItem, 0, len(items))
	for _, item := range items {
		if admin.MatchesKeyword(query.Keyword, item.Name, item.Author, item.ProjectID) {
			filtered = append(filtered, item)
		}
	}
	page := admin.Paginate(filtered, query)
	return response.OK(c, map[string]any{
		"items": page.Items, "total": page.Total, "page": page.Page, "pageSize": page.PageSize,
	})
}

func (h *LeaderboardHandler) Hide(c echo.Context) error {
	projectID := strings.TrimSpace(c.Param("projectId"))
	if projectID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid project id")
	}
	var req struct {
		Hidden *bool `json:"hidden"`
	}
	if err := c.Bind(&req); err != nil || req.Hidden == nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if err := h.Service.Hide(c.Request().Context(), projectID, *req.Hidden); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "update leaderboard failed")
	}
	return response.OK(c, map[string]any{"projectId": projectID, "hidden": *req.Hidden})
}

func (h *LeaderboardHandler) Delete(c echo.Context) error {
	projectID := strings.TrimSpace(c.Param("projectId"))
	if projectID == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid project id")
	}
	if err := h.Service.Delete(c.Request().Context(), projectID); err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50014, "delete leaderboard failed")
	}
	return response.OK(c, map[string]any{"projectId": projectID})
}

func leaderboardPublicBase(c echo.Context, configured string) string {
	if base := strings.TrimRight(strings.TrimSpace(configured), "/"); base != "" {
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

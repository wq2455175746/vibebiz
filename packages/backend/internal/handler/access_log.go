package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/response"
	"vibebiz/internal/usertrace"
)

type AccessLogHandler struct {
	Repo usertrace.Queryer
}

func NewAccessLogHandler(repo usertrace.Queryer) *AccessLogHandler {
	return &AccessLogHandler{Repo: repo}
}

func (h *AccessLogHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	query := usertrace.Query{
		Keyword:   c.QueryParam("keyword"),
		EventType: c.QueryParam("eventType"),
		Page:      page,
		PageSize:  pageSize,
		From:      parseAccessLogTime(c.QueryParam("from"), false),
		To:        parseAccessLogTime(c.QueryParam("to"), true),
	}
	result, err := h.Repo.List(c.Request().Context(), query)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "access logs unavailable")
	}
	return response.OK(c, map[string]any{
		"logs": result.Items, "total": result.Total, "page": result.Page, "pageSize": result.PageSize,
	})
}

func (h *AccessLogHandler) IPStats(c echo.Context) error {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	userID, _ := strconv.ParseInt(c.QueryParam("userId"), 10, 64)
	if userID < 0 {
		userID = 0
	}
	stats, err := h.Repo.IPStats(c.Request().Context(), usertrace.IPStatsQuery{
		From:   parseAccessLogTime(c.QueryParam("from"), false),
		To:     parseAccessLogTime(c.QueryParam("to"), true),
		UserID: userID,
		Limit:  limit,
	})
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50001, "ip stats unavailable")
	}
	return response.OK(c, stats)
}

func parseAccessLogTime(raw string, endOfDay bool) *time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if parsed, err := time.Parse(time.RFC3339, raw); err == nil {
		value := parsed.UTC()
		return &value
	}
	parsed, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return nil
	}
	if endOfDay {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return &parsed
}

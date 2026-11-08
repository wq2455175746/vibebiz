package handler

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/channelguide"
	"vibebiz/internal/response"
)

type ChannelGuideHandler struct {
	Service *channelguide.Service
}

func NewChannelGuideHandler(service *channelguide.Service) *ChannelGuideHandler {
	return &ChannelGuideHandler{Service: service}
}

type channelGuideRequest struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	Enabled      *bool    `json:"enabled"`
	SortOrder    *int     `json:"sortOrder"`
	Builtin      *bool    `json:"builtin"`
}

func (r channelGuideRequest) input() channelguide.Input {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	sortOrder := 100
	if r.SortOrder != nil {
		sortOrder = *r.SortOrder
	}
	return channelguide.Input{
		Channel: r.Channel, Title: r.Title, Summary: r.Summary, Markdown: r.Markdown,
		Steps: r.Steps, Requirements: r.Requirements, Enabled: enabled, SortOrder: sortOrder,
	}
}

type channelGuideListResponse struct {
	Channel       string `json:"channel"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Enabled       bool   `json:"enabled"`
	Builtin       bool   `json:"builtin"`
	SortOrder     int    `json:"sortOrder"`
	MarkdownBytes int    `json:"markdownBytes"`
	UpdatedAt     string `json:"updatedAt"`
}

type channelGuideDetailResponse struct {
	channelGuideListResponse
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
}

type publicChannelGuideResponse struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	UpdatedAt    string   `json:"updatedAt"`
	Enabled      bool     `json:"enabled"`
	Builtin      bool     `json:"builtin"`
	Packageable  bool     `json:"packageable"`
	SortOrder    int      `json:"sortOrder"`
}

func (h *ChannelGuideHandler) PublicList(c echo.Context) error {
	items, err := h.Service.List(c.Request().Context(), false, "")
	if err != nil {
		return channelGuideError(c, err)
	}
	out := make([]publicChannelGuideResponse, 0, len(items))
	for _, item := range items {
		out = append(out, publicChannelGuideResponse{
			Channel: item.Channel, Title: item.Title, Summary: item.Summary, Markdown: item.Markdown,
			Steps: item.Steps, Requirements: item.Requirements,
			UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02"),
			Enabled:   item.Enabled, Builtin: item.Builtin, Packageable: item.Packageable, SortOrder: item.SortOrder,
		})
	}
	return response.OK(c, map[string]any{"guides": out})
}

func (h *ChannelGuideHandler) List(c echo.Context) error {
	status := c.QueryParam("status")
	includeDisabled := status != "enabled"
	items, err := h.Service.List(c.Request().Context(), includeDisabled, c.QueryParam("keyword"))
	if err != nil {
		return channelGuideError(c, err)
	}
	out := make([]channelGuideListResponse, 0, len(items))
	for _, item := range items {
		if status == "disabled" && item.Enabled {
			continue
		}
		out = append(out, toChannelGuideListResponse(item))
	}
	return response.OK(c, map[string]any{"guides": out})
}

func (h *ChannelGuideHandler) Get(c echo.Context) error {
	item, err := h.Service.Get(c.Request().Context(), c.Param("channel"))
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Create(c echo.Context) error {
	var req channelGuideRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	item, err := h.Service.Create(c.Request().Context(), req.input())
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.Created(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Update(c echo.Context) error {
	channel := c.Param("channel")
	var req channelGuideRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if req.Channel != "" && req.Channel != channel {
		return response.Fail(c, http.StatusBadRequest, 40001, "channel cannot be changed")
	}
	if req.Builtin != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "builtin cannot be changed")
	}
	item, err := h.Service.Update(c.Request().Context(), channel, req.input())
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Delete(c echo.Context) error {
	if err := h.Service.Delete(c.Request().Context(), c.Param("channel")); err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *ChannelGuideHandler) UploadMarkdown(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, channelguide.MaxMarkdownBytes+64<<10)
	file, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return response.Fail(c, http.StatusRequestEntityTooLarge, 41303, "channel guide markdown is too large")
		}
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	if !allowedMarkdownContentType(file.Header.Get("Content-Type")) {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file type")
	}
	src, err := file.Open()
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(io.LimitReader(src, channelguide.MaxMarkdownBytes+1))
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	item, err := h.Service.UploadMarkdown(c.Request().Context(), c.Param("channel"), file.Filename, data)
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func allowedMarkdownContentType(value string) bool {
	if semicolon := strings.IndexByte(value, ';'); semicolon >= 0 {
		value = value[:semicolon]
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "text/markdown", "text/plain", "application/octet-stream":
		return true
	default:
		return false
	}
}

func toChannelGuideListResponse(item channelguide.Item) channelGuideListResponse {
	return channelGuideListResponse{
		Channel: item.Channel, Title: item.Title, Summary: item.Summary, Enabled: item.Enabled,
		Builtin: item.Builtin, SortOrder: item.SortOrder, MarkdownBytes: len(item.Markdown),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toChannelGuideDetailResponse(item channelguide.Item) channelGuideDetailResponse {
	return channelGuideDetailResponse{
		channelGuideListResponse: toChannelGuideListResponse(item),
		Markdown:                 item.Markdown, Steps: item.Steps, Requirements: item.Requirements,
	}
}

func channelGuideError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, channelguide.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid channel guide")
	case errors.Is(err, channelguide.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40411, "channel guide not found")
	case errors.Is(err, channelguide.ErrConflict), errors.Is(err, channelguide.ErrBuiltinDelete):
		return response.Fail(c, http.StatusConflict, 40905, "channel guide conflict")
	case errors.Is(err, channelguide.ErrTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41303, "channel guide markdown is too large")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "channel guide request failed")
	}
}

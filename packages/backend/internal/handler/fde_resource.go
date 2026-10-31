package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/fderesource"
	"vibebiz/internal/response"
)

type FdeResourceHandler struct {
	Service *fderesource.Service
}

func NewFdeResourceHandler(service *fderesource.Service) *FdeResourceHandler {
	return &FdeResourceHandler{Service: service}
}

type fdeResourceRequest struct {
	Name    string   `json:"name"`
	Contact string   `json:"contact"`
	Skills  []string `json:"skills"`
	Region  string   `json:"region"`
	Note    string   `json:"note"`
	Enabled *bool    `json:"enabled"`
}

func (r fdeResourceRequest) input() fderesource.Input {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return fderesource.Input{
		Name: r.Name, Contact: r.Contact, Skills: r.Skills, Region: r.Region, Note: r.Note,
		Enabled: enabled,
	}
}

func (h *FdeResourceHandler) AdminList(c echo.Context) error {
	items, err := h.Service.List(c.Request().Context(), false)
	if err != nil {
		return fdeResourceError(c, err)
	}
	return response.OK(c, map[string]any{"resources": items})
}

func (h *FdeResourceHandler) UserList(c echo.Context) error {
	items, err := h.Service.List(c.Request().Context(), true)
	if err != nil {
		return fdeResourceError(c, err)
	}
	return response.OK(c, map[string]any{"resources": items})
}

func (h *FdeResourceHandler) Create(c echo.Context) error {
	var req fdeResourceRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	item, err := h.Service.Create(c.Request().Context(), req.input())
	if err != nil {
		return fdeResourceError(c, err)
	}
	return response.Created(c, item)
}

func (h *FdeResourceHandler) Update(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid fde resource id")
	}
	var req fdeResourceRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	item, err := h.Service.Update(c.Request().Context(), id, req.input())
	if err != nil {
		return fdeResourceError(c, err)
	}
	return response.OK(c, item)
}

func fdeResourceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, fderesource.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40014, "invalid fde resource")
	case errors.Is(err, fderesource.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40406, "fde resource not found")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50009, "fde resource request failed")
	}
}

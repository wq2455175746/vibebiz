package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/businesscard"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type CardLikeService interface {
	Set(ctx context.Context, cardPublicID string, userID int64, liked bool) (businesscard.LikeState, error)
}

type CardLikeHandler struct {
	Service CardLikeService
	Metrics *observability.Metrics
}

func NewCardLikeHandler(service CardLikeService) *CardLikeHandler {
	return &CardLikeHandler{Service: service}
}

func (h *CardLikeHandler) Set(c echo.Context) error { return h.apply(c, true) }

func (h *CardLikeHandler) Unset(c echo.Context) error { return h.apply(c, false) }

func (h *CardLikeHandler) apply(c echo.Context, liked bool) error {
	action := "unlike"
	if liked {
		action = "like"
	}
	session, ok := auth.SessionFrom(c)
	if !ok {
		h.record(action, "unauthorized")
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	state, err := h.Service.Set(c.Request().Context(), c.Param("id"), session.UserID, liked)
	if err != nil {
		if errors.Is(err, businesscard.ErrNotFound) {
			h.record(action, "not_found")
			return response.Fail(c, http.StatusNotFound, 40410, "card not found")
		}
		h.record(action, "unavailable")
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "like service unavailable")
	}
	h.record(action, "ok")
	return response.OK(c, map[string]any{"liked": state.Liked, "likeCount": state.Count})
}

func (h *CardLikeHandler) record(action, status string) {
	if h.Metrics != nil {
		h.Metrics.RecordCardLike(action, status)
	}
}

package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/businesscard"
	"vibebiz/internal/observability"
	"vibebiz/internal/response"
)

type CardCommentService interface {
	ListRoots(ctx context.Context, cardPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error)
	ListReplies(ctx context.Context, cardPublicID, rootPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error)
	Create(ctx context.Context, cardPublicID string, userID int64, parentPublicID, content string) (*businesscard.Comment, error)
	Update(ctx context.Context, cardPublicID, commentPublicID string, userID int64, content string) (*businesscard.Comment, error)
	Delete(ctx context.Context, cardPublicID, commentPublicID string, userID int64) (businesscard.CommentDeleteResult, error)
}

type CardCommentHandler struct {
	Service     CardCommentService
	Covers      CoverURLSigner
	CoverExpiry time.Duration
	Metrics     *observability.Metrics
}

func NewCardCommentHandler(service CardCommentService) *CardCommentHandler {
	return &CardCommentHandler{Service: service}
}

func (h *CardCommentHandler) ListRoots(c echo.Context) error {
	page, err := h.Service.ListRoots(c.Request().Context(), c.Param("id"), commentUserID(c), c.QueryParam("cursor"), commentLimit(c))
	if err != nil {
		return commentError(c, err)
	}
	return h.okPage(c, page)
}

func (h *CardCommentHandler) ListReplies(c echo.Context) error {
	page, err := h.Service.ListReplies(c.Request().Context(), c.Param("id"), c.Param("commentId"), commentUserID(c), c.QueryParam("cursor"), commentLimit(c))
	if err != nil {
		return commentError(c, err)
	}
	return h.okPage(c, page)
}

func (h *CardCommentHandler) Create(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		h.record("create", "unauthorized")
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req struct {
		Content  string `json:"content"`
		ParentID string `json:"parentId"`
	}
	if err := c.Bind(&req); err != nil {
		h.record("create", "bad_request")
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	comment, err := h.Service.Create(c.Request().Context(), c.Param("id"), session.UserID, req.ParentID, req.Content)
	if err != nil {
		h.record("create", commentStatus(err))
		return commentError(c, err)
	}
	h.record("create", "ok")
	return h.okComment(c, comment)
}

func (h *CardCommentHandler) Update(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		h.record("update", "unauthorized")
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	var req struct {
		Content string `json:"content"`
	}
	if err := c.Bind(&req); err != nil {
		h.record("update", "bad_request")
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	comment, err := h.Service.Update(c.Request().Context(), c.Param("id"), c.Param("commentId"), session.UserID, req.Content)
	if err != nil {
		h.record("update", commentStatus(err))
		return commentError(c, err)
	}
	h.record("update", "ok")
	return h.okComment(c, comment)
}

func (h *CardCommentHandler) Delete(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok {
		h.record("delete", "unauthorized")
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	result, err := h.Service.Delete(c.Request().Context(), c.Param("id"), c.Param("commentId"), session.UserID)
	if err != nil {
		h.record("delete", commentStatus(err))
		return commentError(c, err)
	}
	h.record("delete", "ok")
	return response.OK(c, result)
}

func (h *CardCommentHandler) record(action, status string) {
	if h.Metrics != nil {
		h.Metrics.RecordCardComment(action, status)
	}
}

func (h *CardCommentHandler) okPage(c echo.Context, page *businesscard.CommentPage) error {
	if page != nil {
		h.signComments(c.Request().Context(), page.Comments)
	}
	return response.OK(c, page)
}

func (h *CardCommentHandler) okComment(c echo.Context, comment *businesscard.Comment) error {
	if comment != nil {
		comments := []businesscard.Comment{*comment}
		h.signComments(c.Request().Context(), comments)
		*comment = comments[0]
	}
	return response.OK(c, comment)
}

func (h *CardCommentHandler) signComments(ctx context.Context, comments []businesscard.Comment) {
	for i := range comments {
		key := comments[i].Author.AvatarURL
		if key != "" && !strings.HasPrefix(key, "http://") && !strings.HasPrefix(key, "https://") {
			if h.Covers == nil {
				comments[i].Author.AvatarURL = ""
			} else if signed, err := h.Covers.PresignGet(ctx, key, h.CoverExpiry); err == nil {
				comments[i].Author.AvatarURL = signed
			} else {
				comments[i].Author.AvatarURL = ""
			}
		}
		h.signComments(ctx, comments[i].Replies)
	}
}

func commentUserID(c echo.Context) int64 {
	if session, ok := auth.SessionFrom(c); ok {
		return session.UserID
	}
	return 0
}

func commentLimit(c echo.Context) int {
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	return limit
}

func commentStatus(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, businesscard.ErrCommentInvalid):
		return "invalid"
	case errors.Is(err, businesscard.ErrCommentForbidden):
		return "forbidden"
	case errors.Is(err, businesscard.ErrCommentNotFound):
		return "not_found"
	case errors.Is(err, businesscard.ErrCommentRateLimit):
		return "rate_limited"
	default:
		return "error"
	}
}

func commentError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, businesscard.ErrCommentInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid comment")
	case errors.Is(err, businesscard.ErrCommentForbidden):
		return response.Fail(c, http.StatusForbidden, 40301, "comment forbidden")
	case errors.Is(err, businesscard.ErrCommentNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "comment not found")
	case errors.Is(err, businesscard.ErrCommentRateLimit):
		return response.Fail(c, http.StatusTooManyRequests, 42901, "too many comments")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "comment request failed")
	}
}

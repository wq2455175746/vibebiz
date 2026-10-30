package handler

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/response"
	"vibebiz/internal/store"
)

const maxCoverBytes = 5 * 1024 * 1024

var pngSignature = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

type CoverStore interface {
	SetCover(ctx context.Context, id, key string) error
}

type CoverS3 interface {
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

type CoverHandler struct {
	Store  CoverStore
	S3     CoverS3
	Expiry time.Duration
}

func NewCoverHandler(store CoverStore, s3 CoverS3, expiry time.Duration) *CoverHandler {
	if expiry <= 0 {
		expiry = 15 * time.Minute
	}
	return &CoverHandler{Store: store, S3: s3, Expiry: expiry}
}

func (h *CoverHandler) Upload(c echo.Context) error {
	if h.S3 == nil {
		return response.Fail(c, http.StatusServiceUnavailable, 50303, "cover storage is not configured")
	}
	projectID := c.Param("id")
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, maxCoverBytes+1))
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40008, "invalid cover image")
	}
	if len(body) > maxCoverBytes {
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41301, "cover image is too large")
	}
	if !bytes.HasPrefix(body, pngSignature) {
		return response.Fail(c, http.StatusBadRequest, 40008, "cover image must be a png")
	}

	key := "projects/" + url.PathEscape(projectID) + "/cover.png"
	if err := h.S3.Put(c.Request().Context(), key, "image/png", int64(len(body)), bytes.NewReader(body)); err != nil {
		return response.Fail(c, http.StatusBadGateway, 50203, "upload cover failed")
	}
	if err := h.Store.SetCover(c.Request().Context(), projectID, key); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return response.Fail(c, http.StatusNotFound, 40401, "project not found")
		}
		return response.Fail(c, http.StatusInternalServerError, 50012, "save cover failed")
	}
	coverURL, err := h.S3.PresignGet(c.Request().Context(), key, h.Expiry)
	if err != nil {
		return response.Fail(c, http.StatusInternalServerError, 50013, "sign cover url failed")
	}
	return response.OK(c, map[string]any{"coverS3Path": key, "coverUrl": coverURL})
}

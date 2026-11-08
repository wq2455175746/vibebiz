package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/channelguide"
)

func newChannelGuideHandler(t *testing.T) *ChannelGuideHandler {
	t.Helper()
	svc := channelguide.NewService(channelguide.NewMemoryRepository())
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewChannelGuideHandler(svc)
}

func TestChannelGuideHandlerPublicList(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.GET("/guides", h.PublicList)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "\"updatedAt\":\"0001") {
		t.Fatalf("updatedAt missing: %s", rec.Body.String())
	}
}

func TestChannelGuideHandlerCreateAndDeleteBuiltin(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides", h.Create)
	e.DELETE("/guides/:channel", h.Delete)

	body := `{"channel":"taptap_mobile","title":"TapTap","summary":"","markdown":"# TapTap","steps":["a"],"requirements":["b"],"enabled":true,"sortOrder":40}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/guides", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "taptap_mobile") {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/guides/wechat_minigame", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "40905") {
		t.Fatalf("delete builtin status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuideHandlerUploadMarkdown(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides/:channel/markdown", h.UploadMarkdown)

	body := "--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"guide.md\"\r\nContent-Type: text/markdown\r\n\r\n# Uploaded\r\n--boundary--\r\n"
	req := httptest.NewRequest(http.MethodPost, "/guides/wechat_minigame/markdown", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "# Uploaded") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

package handler

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
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

func TestChannelGuideHandlerSortOrderDefaultsAndZero(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides", h.Create)

	cases := []struct {
		name string
		body string
		want string
	}{
		{name: "omitted defaults to 100", body: `{"channel":"custom_default","title":"Default","markdown":"# x"}`, want: `"sortOrder":100`},
		{name: "explicit zero preserved", body: `{"channel":"custom_zero","title":"Zero","markdown":"# x","sortOrder":0}`, want: `"sortOrder":0`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/guides", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			e.ServeHTTP(rec, req)
			if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChannelGuideHandlerCreateIgnoresBuiltin(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides", h.Create)

	body := `{"channel":"custom_builtin","title":"Custom","markdown":"# x","builtin":true}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/guides", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), `"builtin":false`) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuideHandlerUpdateRejectsBuiltin(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.PUT("/guides/:channel", h.Update)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/guides/wechat_minigame", strings.NewReader(`{"builtin":false}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "40001") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "builtin cannot be changed") {
		t.Fatalf("body = %s", rec.Body.String())
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

func createDisabledGuide(t *testing.T, h *ChannelGuideHandler) {
	t.Helper()
	_, err := h.Service.Create(context.Background(), channelguide.Input{
		Channel: "taptap_mobile", Title: "TapTap", Markdown: "# TapTap", Enabled: false, SortOrder: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideHandlerListStatusFilter(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	createDisabledGuide(t, h)
	e.GET("/guides", h.List)

	cases := []struct {
		name        string
		query       string
		wantGuide   bool
		wantAnother bool
	}{
		{name: "enabled excludes disabled", query: "?status=enabled", wantGuide: false, wantAnother: true},
		{name: "disabled includes only disabled", query: "?status=disabled", wantGuide: true, wantAnother: false},
		{name: "all includes both", query: "?status=all", wantGuide: true, wantAnother: true},
		{name: "empty includes both", query: "", wantGuide: true, wantAnother: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides"+tc.query, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
			}
			if got := strings.Contains(rec.Body.String(), "taptap_mobile"); got != tc.wantGuide {
				t.Fatalf("contains disabled guide = %v, want %v: %s", got, tc.wantGuide, rec.Body.String())
			}
			if got := strings.Contains(rec.Body.String(), "wechat_minigame"); got != tc.wantAnother {
				t.Fatalf("contains enabled guide = %v, want %v: %s", got, tc.wantAnother, rec.Body.String())
			}
		})
	}
}

func TestChannelGuideHandlerPublicListExcludesDisabled(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	createDisabledGuide(t, h)
	e.GET("/guides", h.PublicList)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "taptap_mobile") {
		t.Fatalf("public list contains disabled guide: %s", rec.Body.String())
	}
}

func TestChannelGuideHandlerUploadOversizedBody(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides/:channel/markdown", h.UploadMarkdown)

	data := bytes.Repeat([]byte("a"), channelguide.MaxMarkdownBytes+(128<<10))
	contentType, body := multipartGuideBody(t, "guide.md", "text/markdown", data)
	req := httptest.NewRequest(http.MethodPost, "/guides/wechat_minigame/markdown", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "41303") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuideHandlerUploadRejectsUnsupportedContentType(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides/:channel/markdown", h.UploadMarkdown)

	contentType, body := multipartGuideBody(t, "guide.md", "image/png", []byte("# Uploaded"))
	req := httptest.NewRequest(http.MethodPost, "/guides/wechat_minigame/markdown", bytes.NewReader(body))
	req.Header.Set(echo.HeaderContentType, contentType)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "40001") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func multipartGuideBody(t *testing.T, filename, contentType string, data []byte) (string, []byte) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, filename))
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	part, err := w.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return w.FormDataContentType(), buf.Bytes()
}

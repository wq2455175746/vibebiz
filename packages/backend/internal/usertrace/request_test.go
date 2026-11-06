package usertrace

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestCaptureRequestSanitizesQueryAndDevice(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects?keyword=abc&token=secret&password=p", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0 Safari/537.36")
	req.Header.Set("X-Device-Id", "dev_123-ABC")
	req.Header.Set("Referer", "https://vibebiz-dev.com/")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	info := CaptureRequest(c)
	if info.Query == "" || strings.Contains(info.Query, "secret") || strings.Contains(info.Query, "password=p") {
		t.Fatalf("query = %q", info.Query)
	}
	if info.DeviceID != "dev_123-ABC" || info.DeviceType != "desktop" || info.OS != "Windows" || info.Browser != "Chrome" {
		t.Fatalf("info = %+v", info)
	}
	if info.Referer != "https://vibebiz-dev.com/" {
		t.Fatalf("referer = %q", info.Referer)
	}
}

func TestCaptureRequestDropsInvalidDeviceID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil)
	req.Header.Set("X-Device-Id", "bad id with spaces")
	c := e.NewContext(req, httptest.NewRecorder())

	if info := CaptureRequest(c); info.DeviceID != "" {
		t.Fatalf("device id = %q, want empty", info.DeviceID)
	}
}

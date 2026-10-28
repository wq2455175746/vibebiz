package auth

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func newEchoContext(req *http.Request, rec *httptest.ResponseRecorder) echo.Context {
	return echo.New().NewContext(req, rec)
}

func TestCookieSetAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	c := newEchoContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	m := CookieManager{Name: "vibebiz_user", Secure: true, TTL: time.Hour, Now: time.Now}

	if err := m.Set(c, CookieIdentity{SID: "sid-1", Username: "u", Email: "u@example.com"}); err != nil {
		t.Fatalf("set: %v", err)
	}

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != "vibebiz_user" {
		t.Fatalf("name = %q", cookie.Name)
	}
	if !cookie.HttpOnly {
		t.Fatal("cookie should be HttpOnly")
	}
	if cookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("same site = %v, want Lax", cookie.SameSite)
	}
	if cookie.Path != "/" {
		t.Fatalf("path = %q, want /", cookie.Path)
	}
	if !cookie.Secure {
		t.Fatal("cookie should be secure")
	}
	if cookie.MaxAge != 3600 {
		t.Fatalf("max age = %d, want 3600", cookie.MaxAge)
	}
	if cookie.Domain != "" {
		t.Fatalf("domain = %q, want empty", cookie.Domain)
	}
}

func TestCookieReadRoundTrip(t *testing.T) {
	rec := httptest.NewRecorder()
	c := newEchoContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	m := CookieManager{Name: "vibebiz_ops", TTL: time.Hour, Now: time.Now}
	want := CookieIdentity{SID: "sid-9", Username: "ops", Email: "ops@example.com"}

	if err := m.Set(c, want); err != nil {
		t.Fatalf("set: %v", err)
	}
	cookie := rec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(cookie)
	got, err := m.Read(newEchoContext(req, httptest.NewRecorder()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != want {
		t.Fatalf("identity = %+v, want %+v", got, want)
	}
}

func TestCookieReadMissing(t *testing.T) {
	m := CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now}
	_, err := m.Read(newEchoContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder()))
	if !errors.Is(err, ErrNoCookie) {
		t.Fatalf("err = %v, want ErrNoCookie", err)
	}
}

func TestCookieReadTampered(t *testing.T) {
	m := CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now}
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "vibebiz_user", Value: "!!!not-base64!!!"})
	if _, err := m.Read(newEchoContext(req, httptest.NewRecorder())); err == nil {
		t.Fatal("expected error for tampered cookie")
	}
}

func TestCookieClear(t *testing.T) {
	rec := httptest.NewRecorder()
	c := newEchoContext(httptest.NewRequest(http.MethodPost, "/", nil), rec)
	m := CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now}

	m.Clear(c)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	if cookies[0].MaxAge >= 0 {
		t.Fatalf("max age = %d, want negative", cookies[0].MaxAge)
	}
}

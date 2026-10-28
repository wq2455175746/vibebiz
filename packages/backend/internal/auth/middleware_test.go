package auth

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

type testEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func newAuthenticator(t *testing.T) (*Authenticator, *fakeSessionStore) {
	t.Helper()
	sessions := newFakeSessionStore()
	svc := &Service{Sessions: sessions, Now: time.Now}
	return &Authenticator{
		Service:    svc,
		UserCookie: CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now},
		OpsCookie:  CookieManager{Name: "vibebiz_ops", TTL: time.Hour, Now: time.Now},
	}, sessions
}

func cookieFor(t *testing.T, manager CookieManager, sid string) *http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	c := newEchoContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
	if err := manager.Set(c, CookieIdentity{SID: sid, Username: "u", Email: "u@example.com"}); err != nil {
		t.Fatalf("set cookie: %v", err)
	}
	return rec.Result().Cookies()[0]
}

func serveWithCookie(e *echo.Echo, target string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decodeTestEnvelope(t *testing.T, rec *httptest.ResponseRecorder) testEnvelope {
	t.Helper()
	var env testEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return env
}

func TestRequireUserRejectsMissingCookie(t *testing.T) {
	authn, _ := newAuthenticator(t)
	e := echo.New()
	e.GET("/p", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireUser)

	rec := serveWithCookie(e, "/p", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if env := decodeTestEnvelope(t, rec); env.Code != 40101 {
		t.Fatalf("code = %d, want 40101", env.Code)
	}
}

func TestRequireUserRejectsInvalidCookie(t *testing.T) {
	authn, _ := newAuthenticator(t)
	e := echo.New()
	e.GET("/p", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireUser)

	rec := serveWithCookie(e, "/p", &http.Cookie{Name: "vibebiz_user", Value: "not base64!"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireUserUnknownSession(t *testing.T) {
	authn, _ := newAuthenticator(t)
	e := echo.New()
	e.GET("/p", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireUser)

	rec := serveWithCookie(e, "/p", cookieFor(t, authn.UserCookie, "missing"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireUserRedisError(t *testing.T) {
	authn, sessions := newAuthenticator(t)
	sessions.err = errors.New("redis down")
	e := echo.New()
	e.GET("/p", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireUser)

	rec := serveWithCookie(e, "/p", cookieFor(t, authn.UserCookie, "sid"))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if env := decodeTestEnvelope(t, rec); env.Code != 50301 {
		t.Fatalf("code = %d, want 50301", env.Code)
	}
}

func TestRequireUserWrongRealm(t *testing.T) {
	authn, sessions := newAuthenticator(t)
	sessions.items[sessionKey(RealmUser, "sid")] = &Session{ID: "sid", Realm: RealmOps, UserID: 1}
	e := echo.New()
	e.GET("/p", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireUser)

	rec := serveWithCookie(e, "/p", cookieFor(t, authn.UserCookie, "sid"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestRequireUserAcceptsValidSession(t *testing.T) {
	authn, sessions := newAuthenticator(t)
	sessions.items[sessionKey(RealmUser, "sid")] = &Session{ID: "sid", Realm: RealmUser, UserID: 5, Username: "u"}
	e := echo.New()
	e.GET("/p", func(c echo.Context) error {
		session, ok := SessionFrom(c)
		if !ok || session.UserID != 5 {
			t.Fatalf("session from context = %+v, ok = %v", session, ok)
		}
		return c.NoContent(http.StatusOK)
	}, authn.RequireUser)

	rec := serveWithCookie(e, "/p", cookieFor(t, authn.UserCookie, "sid"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRequireOpsAcceptsOnlyOpsSession(t *testing.T) {
	authn, sessions := newAuthenticator(t)
	sessions.items[sessionKey(RealmOps, "ops-sid")] = &Session{ID: "ops-sid", Realm: RealmOps, UserID: 9, Role: "super_admin"}
	sessions.items[sessionKey(RealmUser, "user-sid")] = &Session{ID: "user-sid", Realm: RealmUser, UserID: 5}

	e := echo.New()
	e.GET("/admin", func(c echo.Context) error { return c.NoContent(http.StatusOK) }, authn.RequireOps)

	rec := serveWithCookie(e, "/admin", cookieFor(t, authn.OpsCookie, "ops-sid"))
	if rec.Code != http.StatusOK {
		t.Fatalf("ops status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithCookie(e, "/admin", cookieFor(t, authn.OpsCookie, "user-sid"))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user session on ops status = %d, want 401", rec.Code)
	}
}

func TestSessionFromEmpty(t *testing.T) {
	c := newEchoContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
	if _, ok := SessionFrom(c); ok {
		t.Fatal("expected no session")
	}
}

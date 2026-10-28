package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/dao/model"
)

type authUserFinder struct {
	user         *model.User
	workspaceIDs []int64
}

func (f *authUserFinder) FindUserByIdentifier(_ context.Context, _ string) (*model.User, error) {
	return f.user, nil
}

func (f *authUserFinder) WorkspaceIDs(_ context.Context, _ int64) ([]int64, error) {
	return f.workspaceIDs, nil
}

func (f *authUserFinder) TouchUserLogin(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}

type authOpsFinder struct {
	user *model.OpsUser
}

func (f *authOpsFinder) FindOpsByIdentifier(_ context.Context, _ string) (*model.OpsUser, error) {
	return f.user, nil
}

func (f *authOpsFinder) TouchOpsLogin(_ context.Context, _ int64, _ string, _ time.Time) error {
	return nil
}

type authSessionStore struct {
	items map[string]*auth.Session
}

func newAuthSessionStore() *authSessionStore {
	return &authSessionStore{items: make(map[string]*auth.Session)}
}

func (f *authSessionStore) key(realm auth.Realm, id string) string {
	return string(realm) + ":" + id
}

func (f *authSessionStore) Create(_ context.Context, session *auth.Session, _ time.Duration) error {
	copied := *session
	f.items[f.key(session.Realm, session.ID)] = &copied
	return nil
}

func (f *authSessionStore) Get(_ context.Context, realm auth.Realm, id string) (*auth.Session, error) {
	session, ok := f.items[f.key(realm, id)]
	if !ok {
		return nil, auth.ErrSessionNotFound
	}
	copied := *session
	return &copied, nil
}

func (f *authSessionStore) Delete(_ context.Context, realm auth.Realm, id string) error {
	delete(f.items, f.key(realm, id))
	return nil
}

func newAuthHandlerForTest(t *testing.T, user *model.User, workspaces []int64, ops *model.OpsUser) (*AuthHandler, *authSessionStore) {
	t.Helper()
	hash, err := auth.HashPassword("88888888")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if user != nil {
		user.PasswordHash = &hash
	}
	if ops != nil {
		ops.PasswordHash = &hash
	}
	sessions := newAuthSessionStore()
	service := &auth.Service{
		Users:    &authUserFinder{user: user, workspaceIDs: workspaces},
		Ops:      &authOpsFinder{user: ops},
		Sessions: sessions,
		UserTTL:  168 * time.Hour,
		OpsTTL:   12 * time.Hour,
		Now:      func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	handler := &AuthHandler{
		Service:    service,
		UserCookie: auth.CookieManager{Name: "vibebiz_user", TTL: 168 * time.Hour, Now: time.Now},
		OpsCookie:  auth.CookieManager{Name: "vibebiz_ops", TTL: 12 * time.Hour, Now: time.Now},
	}
	return handler, sessions
}

type authEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

func decodeAuthEnvelope(t *testing.T, rec *httptest.ResponseRecorder) authEnvelope {
	t.Helper()
	var env authEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return env
}

func authRouter(h *AuthHandler) *echo.Echo {
	e := echo.New()
	e.POST("/cp/api/v1/auth/login", h.LoginUser)
	e.GET("/cp/api/v1/auth/me", h.MeUser)
	e.POST("/cp/api/v1/auth/logout", h.LogoutUser)
	e.POST("/cp/api/v1/admin/auth/login", h.LoginOps)
	e.GET("/cp/api/v1/admin/auth/me", h.MeOps)
	e.POST("/cp/api/v1/admin/auth/logout", h.LogoutOps)
	return e
}

func postLogin(e *echo.Echo, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func getWithCookie(e *echo.Echo, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func loginUser(t *testing.T, h *AuthHandler) (*http.Cookie, auth.Session) {
	t.Helper()
	e := authRouter(h)
	rec := postLogin(e, "/cp/api/v1/auth/login", `{"identifier":"wanghg11","password":"88888888"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	return cookies[0], auth.Session{}
}

func TestLoginUserSuccess(t *testing.T) {
	h, sessions := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "wanghg11", Email: "wang@example.com", Nickname: "Wang", Status: 1}, []int64{3}, nil)
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/auth/login", `{"identifier":"  wanghg11  ","password":"88888888"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeAuthEnvelope(t, rec)
	var user struct {
		ID           int64   `json:"id"`
		Username     string  `json:"username"`
		Email        string  `json:"email"`
		Nickname     string  `json:"nickname"`
		WorkspaceIDs []int64 `json:"workspaceIds"`
	}
	if err := json.Unmarshal(env.Result, &user); err != nil {
		t.Fatalf("decode user: %v", err)
	}
	if user.ID != 1 || user.Username != "wanghg11" || user.Email != "wang@example.com" || len(user.WorkspaceIDs) != 1 || user.WorkspaceIDs[0] != 3 {
		t.Fatalf("user = %+v", user)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "vibebiz_user" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v", cookies)
	}
	if len(sessions.items) != 1 {
		t.Fatalf("sessions = %+v", sessions.items)
	}
}

func TestLoginUserBadCredentials(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, []int64{1}, nil)
	rec := postLogin(authRouter(h), "/cp/api/v1/auth/login", `{"identifier":"u","password":"wrong"}`, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40102 {
		t.Fatalf("code = %d, want 40102", env.Code)
	}
}

func TestLoginUserDisabled(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 0}, []int64{1}, nil)
	rec := postLogin(authRouter(h), "/cp/api/v1/auth/login", `{"identifier":"u","password":"88888888"}`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40301 {
		t.Fatalf("code = %d, want 40301", env.Code)
	}
}

func TestLoginUserNoWorkspace(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, nil, nil)
	rec := postLogin(authRouter(h), "/cp/api/v1/auth/login", `{"identifier":"u","password":"88888888"}`, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40302 {
		t.Fatalf("code = %d, want 40302", env.Code)
	}
}

func TestLoginUserInvalidBody(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Status: 1}, []int64{1}, nil)
	for _, body := range []string{`not json`, `{"identifier":"u"}`, `{"password":"88888888"}`} {
		rec := postLogin(authRouter(h), "/cp/api/v1/auth/login", body, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("body %q status = %d, want 400", body, rec.Code)
		}
	}
}

func TestMeUser(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "wanghg11", Email: "wang@example.com", Status: 1}, []int64{3}, nil)
	cookie, _ := loginUser(t, h)

	rec := getWithCookie(authRouter(h), "/cp/api/v1/auth/me", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeAuthEnvelope(t, rec)
	var user struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
	}
	if err := json.Unmarshal(env.Result, &user); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if user.ID != 1 || user.Username != "wanghg11" || user.Email != "wang@example.com" {
		t.Fatalf("user = %+v", user)
	}
}

func TestMeUserUnauthorized(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Status: 1}, []int64{1}, nil)
	rec := getWithCookie(authRouter(h), "/cp/api/v1/auth/me", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40101 {
		t.Fatalf("code = %d, want 40101", env.Code)
	}
}

func TestLogoutUser(t *testing.T) {
	h, sessions := newAuthHandlerForTest(t, &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}, []int64{1}, nil)
	cookie, _ := loginUser(t, h)
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/auth/logout", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "vibebiz_user" || cookies[0].MaxAge >= 0 {
		t.Fatalf("clear cookie = %+v", cookies)
	}
	if len(sessions.items) != 0 {
		t.Fatalf("sessions not deleted: %+v", sessions.items)
	}
	if rec := getWithCookie(e, "/cp/api/v1/auth/me", cookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("me after logout status = %d, want 401", rec.Code)
	}
	if rec := postLogin(e, "/cp/api/v1/auth/logout", "", nil); rec.Code != http.StatusOK {
		t.Fatalf("logout without cookie status = %d, want 200", rec.Code)
	}
}

func TestLoginOps(t *testing.T) {
	h, _ := newAuthHandlerForTest(t, nil, nil, &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Email: "admin@example.com", RoleCode: "super_admin", Status: 1})
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/admin/auth/login", `{"identifier":"admin","password":"88888888"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeAuthEnvelope(t, rec)
	var ops struct {
		ID       int64  `json:"id"`
		Username string `json:"username"`
		Email    string `json:"email"`
		RoleCode string `json:"roleCode"`
	}
	if err := json.Unmarshal(env.Result, &ops); err != nil {
		t.Fatalf("decode ops: %v", err)
	}
	if ops.ID != 9 || ops.RoleCode != "super_admin" || ops.Email != "admin@example.com" {
		t.Fatalf("ops = %+v", ops)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "vibebiz_ops" || !cookies[0].HttpOnly {
		t.Fatalf("cookies = %+v", cookies)
	}

	rec = getWithCookie(e, "/cp/api/v1/admin/auth/me", cookies[0])
	if rec.Code != http.StatusOK {
		t.Fatalf("me status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCookieRealmsCannotCross(t *testing.T) {
	user := &model.User{Base: model.Base{ID: 1}, Username: "u", Email: "u@example.com", Status: 1}
	ops := &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Email: "admin@example.com", RoleCode: "super_admin", Status: 1}
	h, _ := newAuthHandlerForTest(t, user, []int64{1}, ops)
	e := authRouter(h)

	userCookie, _ := loginUser(t, h)
	if rec := getWithCookie(e, "/cp/api/v1/admin/auth/me", userCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("user cookie on ops me status = %d, want 401", rec.Code)
	}

	rec := postLogin(e, "/cp/api/v1/admin/auth/login", `{"identifier":"admin","password":"88888888"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("ops login status = %d", rec.Code)
	}
	opsCookie := rec.Result().Cookies()[0]
	if rec := getWithCookie(e, "/cp/api/v1/auth/me", opsCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ops cookie on user me status = %d, want 401", rec.Code)
	}
}

func TestLogoutOps(t *testing.T) {
	h, sessions := newAuthHandlerForTest(t, nil, nil, &model.OpsUser{Base: model.Base{ID: 9}, Username: "admin", Email: "admin@example.com", RoleCode: "super_admin", Status: 1})
	e := authRouter(h)

	rec := postLogin(e, "/cp/api/v1/admin/auth/login", `{"identifier":"admin","password":"88888888"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}
	cookie := rec.Result().Cookies()[0]

	rec = postLogin(e, "/cp/api/v1/admin/auth/logout", "", cookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout status = %d", rec.Code)
	}
	if len(sessions.items) != 0 {
		t.Fatalf("ops session not deleted")
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "vibebiz_ops" || cookies[0].MaxAge >= 0 {
		t.Fatalf("clear cookie = %+v", cookies)
	}
}

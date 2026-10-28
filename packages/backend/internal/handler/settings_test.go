package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/account"
	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
)

type settingsPresigner struct {
	headSize   int64
	headType   string
	headErr    error
	putKey     string
	uploadKey  string
	uploadType string
	uploadSize int64
	uploadBody string
	uploadErr  error
}

func (f *settingsPresigner) PresignPut(_ context.Context, key, contentType string, _ int64, _ time.Duration) (string, map[string]string, error) {
	f.putKey = key
	return "https://s3.example/put/" + key, map[string]string{"Content-Type": contentType}, nil
}

func (f *settingsPresigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example/get/" + key, nil
}

func (f *settingsPresigner) Put(_ context.Context, key, contentType string, size int64, body io.Reader) error {
	if f.uploadErr != nil {
		return f.uploadErr
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	f.uploadKey = key
	f.uploadType = contentType
	f.uploadSize = size
	f.uploadBody = string(data)
	return nil
}

func (f *settingsPresigner) Head(_ context.Context, _ string) (int64, string, error) {
	return f.headSize, f.headType, f.headErr
}

func newSettingsTestService(t *testing.T, repo *fakeAccountRepo, presigner account.Presigner) *account.Service {
	t.Helper()
	service := account.NewService(repo, nil, presigner, 15*time.Minute)
	service.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	service.NewID = func() string { return "fixed" }
	return service
}

func settingsFixture(t *testing.T) (*fakeAccountRepo, *settingsPresigner, *account.Service) {
	t.Helper()
	repo := newFakeAccountRepo()
	repo.profile = &dao.UserProfile{ID: 7, Username: "wanghg11", Email: "wang@example.com", AvatarS3Path: "avatar/7/a.png"}
	repo.usersByEmail["wang@example.com"] = repo.profile
	repo.usersByEmail["vistor@example.com"] = &dao.UserProfile{ID: 8, Username: "vistor", Email: "vistor@example.com"}
	repo.memberships = []dao.WorkspaceMembership{
		{ID: 5, Name: "personal", OwnerUserID: 7, Role: "owner"},
		{ID: 9, Name: "team", OwnerUserID: 8, Role: "editor"},
	}
	repo.workspaces[5] = &dao.WorkspaceDetail{ID: 5, Name: "personal", Description: "mine", OwnerUserID: 7, MemberCount: 1}
	repo.workspaces[9] = &dao.WorkspaceDetail{ID: 9, Name: "team", OwnerUserID: 8, MemberCount: 2}
	repo.members[5] = []dao.WorkspaceMember{{UserID: 7, Username: "wanghg11", Email: "wang@example.com", Role: "owner", Status: 1}}
	repo.members[9] = []dao.WorkspaceMember{{UserID: 8, Username: "vistor", Email: "vistor@example.com", Role: "owner", Status: 1}}
	presigner := &settingsPresigner{headSize: 128, headType: "image/png"}
	return repo, presigner, newSettingsTestService(t, repo, presigner)
}

func settingsRouter(h *SettingsHandler) *echo.Echo {
	e := echo.New()
	e.GET("/me/profile", h.GetProfile)
	e.PUT("/me/profile", h.UpdateProfile)
	e.POST("/me/avatar/presign", h.PresignAvatar)
	e.PUT("/me/avatar", h.ConfirmAvatar)
	e.POST("/me/avatar/upload", h.UploadAvatar)
	e.GET("/workspaces/current", h.GetWorkspace)
	e.PUT("/workspaces/current", h.UpdateWorkspace)
	e.POST("/workspaces/current/avatar/presign", h.PresignWorkspaceAvatar)
	e.PUT("/workspaces/current/avatar", h.ConfirmWorkspaceAvatar)
	e.POST("/workspaces/current/avatar/upload", h.UploadWorkspaceAvatar)
	e.GET("/workspaces/current/members", h.Members)
	e.POST("/workspaces/current/members", h.InviteMember)
	e.GET("/workspaces/current/folders", h.Folders)
	e.POST("/workspaces/current/folders", h.CreateFolder)
	return e
}

func settingsRequest(e *echo.Echo, method, target, body string, session *auth.Session) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func uploadRequest(e *echo.Echo, target, contentType, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func sessionMiddleware(session *auth.Session) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if session != nil {
				c.Set("auth.session", session)
			}
			return next(c)
		}
	}
}

func settingsEcho(h *SettingsHandler, session *auth.Session) *echo.Echo {
	e := settingsRouter(h)
	e.Use(sessionMiddleware(session))
	return e
}

func decodeSettingsEnvelope(t *testing.T, rec *httptest.ResponseRecorder) authEnvelope {
	t.Helper()
	return decodeAuthEnvelope(t, rec)
}

func TestSettingsGetProfile(t *testing.T) {
	_, _, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})
	e := settingsEcho(h, sessionForTest(5))

	rec := settingsRequest(e, http.MethodGet, "/me/profile", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var me account.Me
	if err := json.Unmarshal(decodeSettingsEnvelope(t, rec).Result, &me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if me.ID != 7 || me.CurrentWorkspaceID != 5 || me.AvatarURL != "https://s3.example/get/avatar/7/a.png" {
		t.Fatalf("me = %+v", me)
	}
}

func TestSettingsUpdateProfile(t *testing.T) {
	repo, _, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{Name: "vibebiz_user", TTL: time.Hour, Now: time.Now})
	e := settingsEcho(h, sessionForTest(5))

	rec := settingsRequest(e, http.MethodPut, "/me/profile", `{"username":"renamed"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.profile.Username != "renamed" {
		t.Fatalf("profile username = %q", repo.profile.Username)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 1 || cookies[0].Name != "vibebiz_user" {
		t.Fatalf("cookies = %+v", cookies)
	}

	repo.usernameTaken = true
	if rec := settingsRequest(e, http.MethodPut, "/me/profile", `{"username":"taken"}`, nil); rec.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d", rec.Code)
	}
	if rec := settingsRequest(e, http.MethodPut, "/me/profile", `{"username":"x"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d", rec.Code)
	}
}

func TestSettingsAvatarFlow(t *testing.T) {
	repo, presigner, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})
	e := settingsEcho(h, sessionForTest(5))

	rec := settingsRequest(e, http.MethodPost, "/me/avatar/presign", `{"filename":"a.png","contentType":"image/png","size":128}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("presign status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if presigner.putKey != "avatar/7/fixed.png" {
		t.Fatalf("put key = %q", presigner.putKey)
	}
	if rec := settingsRequest(e, http.MethodPost, "/me/avatar/presign", `{"filename":"a.gif","contentType":"image/gif","size":128}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d", rec.Code)
	}
	if rec := settingsRequest(e, http.MethodPost, "/me/avatar/presign", `{"filename":"a.png","contentType":"image/png","size":0}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid size status = %d", rec.Code)
	}
	if rec := settingsRequest(e, http.MethodPut, "/me/avatar", `{"key":"avatar/8/fixed.png"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign key status = %d", rec.Code)
	}
	presigner.headErr = errors.New("missing")
	if rec := settingsRequest(e, http.MethodPut, "/me/avatar", `{"key":"avatar/7/fixed.png"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing upload status = %d", rec.Code)
	}
	presigner.headErr = nil
	rec = settingsRequest(e, http.MethodPut, "/me/avatar", `{"key":"avatar/7/fixed.png"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.profile.AvatarS3Path != "avatar/7/fixed.png" {
		t.Fatalf("stored key = %q", repo.profile.AvatarS3Path)
	}
}

func TestSettingsAvatarUploadFallback(t *testing.T) {
	repo, presigner, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})
	e := settingsEcho(h, sessionForTest(5))

	rec := uploadRequest(e, "/me/avatar/upload", "image/png", "pngdata")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if presigner.uploadKey != "avatar/7/fixed.png" || presigner.uploadType != "image/png" || presigner.uploadSize != int64(len("pngdata")) || presigner.uploadBody != "pngdata" {
		t.Fatalf("upload = %q/%q/%d/%q", presigner.uploadKey, presigner.uploadType, presigner.uploadSize, presigner.uploadBody)
	}
	if repo.profile.AvatarS3Path != "avatar/7/fixed.png" {
		t.Fatalf("stored key = %q", repo.profile.AvatarS3Path)
	}
	var me account.Me
	if err := json.Unmarshal(decodeSettingsEnvelope(t, rec).Result, &me); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if me.ID != 7 {
		t.Fatalf("me = %+v", me)
	}

	if rec := uploadRequest(e, "/me/avatar/upload", "image/gif", "pngdata"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d", rec.Code)
	}
	if rec := uploadRequest(e, "/me/avatar/upload", "", "pngdata"); rec.Code != http.StatusBadRequest {
		t.Fatalf("missing type status = %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPost, "/me/avatar/upload", nil)
	req.Header.Set(echo.HeaderContentType, "image/png")
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body status = %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPost, "/me/avatar/upload", strings.NewReader("x"))
	req.Header.Set(echo.HeaderContentType, "image/png")
	req.ContentLength = account.MaxAvatarBytes + 1
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("oversize status = %d", rec.Code)
	}

	presigner.uploadErr = errors.New("s3 down")
	if rec := uploadRequest(e, "/me/avatar/upload", "image/png", "pngdata"); rec.Code != http.StatusInternalServerError {
		t.Fatalf("s3 error status = %d", rec.Code)
	}
}

func TestSettingsWorkspaceAvatarUploadFallback(t *testing.T) {
	repo, presigner, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})
	owner := settingsEcho(h, sessionForTest(5))

	rec := uploadRequest(owner, "/workspaces/current/avatar/upload", "image/png", "pngdata")
	if rec.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if presigner.uploadKey != "workspace-avatar/5/fixed.png" || presigner.uploadBody != "pngdata" {
		t.Fatalf("upload = %q/%q", presigner.uploadKey, presigner.uploadBody)
	}
	if repo.workspaces[5].AvatarS3Path != "workspace-avatar/5/fixed.png" {
		t.Fatalf("stored key = %q", repo.workspaces[5].AvatarS3Path)
	}
	if rec := uploadRequest(owner, "/workspaces/current/avatar/upload", "image/gif", "pngdata"); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid type status = %d", rec.Code)
	}
	if rec := uploadRequest(settingsEcho(h, sessionForTest(9)), "/workspaces/current/avatar/upload", "image/png", "pngdata"); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner status = %d", rec.Code)
	}
}

func TestSettingsWorkspace(t *testing.T) {
	repo, presigner, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})

	rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodGet, "/workspaces/current", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	var workspace account.Workspace
	if err := json.Unmarshal(decodeSettingsEnvelope(t, rec).Result, &workspace); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !workspace.IsOwner || workspace.Name != "personal" || workspace.MemberCount != 1 {
		t.Fatalf("workspace = %+v", workspace)
	}

	if rec := settingsRequest(settingsEcho(h, sessionForTest(9)), http.MethodPut, "/workspaces/current", `{"name":"nope","description":""}`, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner status = %d", rec.Code)
	}
	rec = settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPut, "/workspaces/current", `{"name":"renamed","description":"desc"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.workspaces[5].Name != "renamed" || repo.workspaces[5].Description != "desc" {
		t.Fatalf("workspace not updated: %+v", repo.workspaces[5])
	}

	rec = settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPost, "/workspaces/current/avatar/presign", `{"filename":"a.png","contentType":"image/png","size":128}`, nil)
	if rec.Code != http.StatusOK || presigner.putKey != "workspace-avatar/5/fixed.png" {
		t.Fatalf("workspace presign status=%d key=%q", rec.Code, presigner.putKey)
	}
	if rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPut, "/workspaces/current/avatar", `{"key":"workspace-avatar/9/fixed.png"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("foreign workspace key status = %d", rec.Code)
	}
	rec = settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPut, "/workspaces/current/avatar", `{"key":"workspace-avatar/5/fixed.png"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("workspace confirm status = %d", rec.Code)
	}
	if repo.workspaces[5].AvatarS3Path != "workspace-avatar/5/fixed.png" {
		t.Fatalf("stored key = %q", repo.workspaces[5].AvatarS3Path)
	}
}

func TestSettingsMembers(t *testing.T) {
	repo, _, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})

	rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodGet, "/workspaces/current/members", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("members status = %d", rec.Code)
	}
	var payload struct {
		Members []account.Member `json:"members"`
	}
	if err := json.Unmarshal(decodeSettingsEnvelope(t, rec).Result, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Members) != 1 || !payload.Members[0].IsOwner {
		t.Fatalf("members = %+v", payload.Members)
	}

	if rec := settingsRequest(settingsEcho(h, sessionForTest(9)), http.MethodPost, "/workspaces/current/members", `{"email":"vistor@example.com"}`, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("non-owner invite status = %d", rec.Code)
	}
	if rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPost, "/workspaces/current/members", `{"email":"missing@example.com"}`, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("missing invite status = %d", rec.Code)
	}
	if rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPost, "/workspaces/current/members", `{"email":"wang@example.com"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("self invite status = %d", rec.Code)
	}
	rec = settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPost, "/workspaces/current/members", `{"email":"vistor@example.com"}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("invite status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(repo.members[5]) != 2 {
		t.Fatalf("members after invite = %+v", repo.members[5])
	}
	repo.members[5] = append(repo.members[5], dao.WorkspaceMember{UserID: 8, Role: "editor", Status: 1})
	if rec := settingsRequest(settingsEcho(h, sessionForTest(5)), http.MethodPost, "/workspaces/current/members", `{"email":"vistor@example.com"}`, nil); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate invite status = %d", rec.Code)
	}
}

func TestSettingsRequiresSession(t *testing.T) {
	_, _, service := settingsFixture(t)
	e := settingsEcho(NewSettingsHandler(service, auth.CookieManager{}), nil)
	for _, target := range []string{"/me/profile", "/workspaces/current", "/workspaces/current/members"} {
		if rec := settingsRequest(e, http.MethodGet, target, "", nil); rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", target, rec.Code)
		}
	}
}

func TestSettingsFolders(t *testing.T) {
	_, _, service := settingsFixture(t)
	h := NewSettingsHandler(service, auth.CookieManager{})
	e := settingsEcho(h, sessionForTest(5))

	rec := settingsRequest(e, http.MethodGet, "/workspaces/current/folders", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = settingsRequest(e, http.MethodPost, "/workspaces/current/folders", `{"name":"设计稿"}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var folder account.Folder
	if err := json.Unmarshal(decodeSettingsEnvelope(t, rec).Result, &folder); err != nil {
		t.Fatalf("decode folder: %v", err)
	}
	if folder.Name != "设计稿" || folder.ParentID != 0 {
		t.Fatalf("folder = %+v", folder)
	}
	if rec := settingsRequest(e, http.MethodPost, "/workspaces/current/folders", `{"name":"设计稿"}`, nil); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d", rec.Code)
	}
	if rec := settingsRequest(e, http.MethodPost, "/workspaces/current/folders", `{"name":"../bad"}`, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d", rec.Code)
	}
}

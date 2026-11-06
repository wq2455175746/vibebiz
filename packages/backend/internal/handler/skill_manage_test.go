package handler

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/client"
)

type fakeStoredSkillClient struct {
	scope      string
	userID     int64
	overwrite  bool
	uploaded   []byte
	deleted    string
	listResult []client.SkillInfo
}

func (f *fakeStoredSkillClient) ListStoredSkills(context.Context, string, int64) ([]client.SkillInfo, error) {
	return f.listResult, nil
}

func (f *fakeStoredSkillClient) UploadSkill(_ context.Context, scope string, userID int64, _ string, source io.Reader, _ int64, overwrite bool) (client.SkillInfo, error) {
	f.scope = scope
	f.userID = userID
	f.overwrite = overwrite
	f.uploaded, _ = io.ReadAll(source)
	return client.SkillInfo{Name: "demo", Source: scope, Description: "demo"}, nil
}

func (f *fakeStoredSkillClient) DeleteSkill(_ context.Context, _ string, _ int64, name string) error {
	f.deleted = name
	return nil
}

func skillUploadRequest(target, fieldScope, fieldUser, overwrite string) *http.Request {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("scope", fieldScope)
	if fieldUser != "" {
		_ = writer.WriteField("userId", fieldUser)
	}
	_ = writer.WriteField("overwrite", overwrite)
	part, _ := writer.CreateFormFile("file", "demo.zip")
	_, _ = part.Write([]byte("zipdata"))
	_ = writer.Close()
	req := httptest.NewRequest(http.MethodPost, target, &body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	return req
}

func TestSkillManagerUserUploadUsesSessionID(t *testing.T) {
	fake := &fakeStoredSkillClient{}
	h := NewSkillManager(fake, admin.NewMemoryRepository(), nil)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/me/skills", h.UploadMine, &auth.Session{
		UserID: 7, Realm: auth.RealmUser,
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, skillUploadRequest("/me/skills", "public", "99", "true"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if fake.scope != "user" || fake.userID != 7 || !fake.overwrite || string(fake.uploaded) != "zipdata" {
		t.Fatalf("upload = %+v", fake)
	}
}

func TestSkillManagerAdminUploadValidatesUser(t *testing.T) {
	fake := &fakeStoredSkillClient{}
	h := NewSkillManager(fake, admin.NewMemoryRepository(), nil)
	e := echo.New()
	e.POST("/admin/skills", h.AdminUpload)

	rec := httptest.NewRecorder()
	req := skillUploadRequest("/admin/skills", "user", "99", "false")
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing user status = %d body = %s", rec.Code, rec.Body.String())
	}
	if fake.userID != 0 {
		t.Fatalf("upload should not run: %+v", fake)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, skillUploadRequest("/admin/skills", "user", "1", "false"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if fake.scope != "user" || fake.userID != 1 {
		t.Fatalf("upload = %+v", fake)
	}
}

func TestSkillManagerAdminDelete(t *testing.T) {
	fake := &fakeStoredSkillClient{}
	h := NewSkillManager(fake, admin.NewMemoryRepository(), nil)
	e := echo.New()
	e.DELETE("/admin/skills/:name", h.AdminDelete)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/admin/skills/demo?scope=public", nil))
	if rec.Code != http.StatusOK || fake.deleted != "demo" {
		t.Fatalf("status = %d fake = %+v", rec.Code, fake)
	}
}

func TestSkillManagerListMine(t *testing.T) {
	fake := &fakeStoredSkillClient{listResult: []client.SkillInfo{{Name: "demo", Source: "public"}}}
	h := NewSkillManager(fake, admin.NewMemoryRepository(), nil)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/me/skills", h.ListMine, &auth.Session{
		UserID: 7, Realm: auth.RealmUser,
	})

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/me/skills", nil))
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte(`"name":"demo"`)) {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

package handler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/agentrole"
)

type fakeRoleService struct {
	roles       []agentrole.Role
	role        agentrole.Role
	input       agentrole.CreateInput
	by          string
	updateID    int64
	updateInput agentrole.UpdateInput
	updateBy    string
	err         error
	key         string
}

func (f *fakeRoleService) List(context.Context) ([]agentrole.Role, error) {
	return f.roles, f.err
}

func (f *fakeRoleService) ListPage(_ context.Context, query agentrole.ListQuery) (agentrole.Page, error) {
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 10
	}
	return agentrole.Page{Items: f.roles, Total: int64(len(f.roles)), Page: page, PageSize: pageSize}, f.err
}

func (f *fakeRoleService) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, f.err
}

func (f *fakeRoleService) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}

func (f *fakeRoleService) Create(_ context.Context, input agentrole.CreateInput, by string) (agentrole.Role, error) {
	f.input = input
	f.by = by
	return f.role, f.err
}

func (f *fakeRoleService) Update(_ context.Context, id int64, input agentrole.UpdateInput, by string) (agentrole.Role, error) {
	f.updateID = id
	f.updateInput = input
	f.updateBy = by
	return f.role, f.err
}

func (f *fakeRoleService) PresignAvatar(context.Context, string, string, int64) (agentrole.Presign, error) {
	return agentrole.Presign{UploadURL: "https://upload", Key: "roles/avatar/x.png", Headers: map[string]string{"Content-Type": "image/png"}}, f.err
}

func (f *fakeRoleService) UploadAvatar(context.Context, string, int64, io.Reader) (string, error) {
	return f.key, f.err
}

func TestRoleList(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{roles: []agentrole.Role{{ID: 1, RoleCode: "alex", RoleNameZH: "全栈工程师"}}})
	e := echo.New()
	e.GET("/cp/api/v1/admin/roles", withModelSession(h.List))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/roles", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Roles []agentrole.Role `json:"roles"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Roles) != 1 {
		t.Fatalf("roles = %+v", env.Result.Roles)
	}
}

func TestRoleCreateMapsErrors(t *testing.T) {
	cases := []struct {
		err  error
		code int
	}{
		{agentrole.ErrInvalid, http.StatusBadRequest},
		{agentrole.ErrConflict, http.StatusConflict},
		{agentrole.ErrStorageUnavailable, http.StatusServiceUnavailable},
		{agentrole.ErrAvatarNotFound, http.StatusBadRequest},
	}
	for _, tc := range cases {
		h := NewRoleHandler(&fakeRoleService{err: tc.err})
		e := echo.New()
		e.POST("/cp/api/v1/admin/roles", withModelSession(h.Create))
		body := `{"roleCode":"frank","roleNameEn":"Frank","roleNameZh":"测试","systemPrompt":"x"}`
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)
		if rec.Code != tc.code {
			t.Fatalf("err %v status = %d, want %d", tc.err, rec.Code, tc.code)
		}
	}
}

func TestRoleCreatePassesInput(t *testing.T) {
	service := &fakeRoleService{role: agentrole.Role{ID: 2, RoleCode: "frank"}}
	h := NewRoleHandler(service)
	e := echo.New()
	e.POST("/cp/api/v1/admin/roles", withModelSession(h.Create))
	body := `{"roleCode":"frank","roleNameEn":"Frank","roleNameZh":"测试","description":"d","systemPrompt":"p","avatarKey":"roles/avatar/x.png","sortOrder":60,"status":1}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.input.RoleCode != "frank" || service.input.AvatarKey != "roles/avatar/x.png" || service.by != "admin" {
		t.Fatalf("input = %+v by = %q", service.input, service.by)
	}
}

func TestRoleUpdatePassesInput(t *testing.T) {
	service := &fakeRoleService{role: agentrole.Role{ID: 7, RoleCode: "frank"}}
	h := NewRoleHandler(service)
	e := echo.New()
	e.PUT("/cp/api/v1/admin/roles/:id", withModelSession(h.Update))
	body := `{"description":"d","systemPrompt":"p","status":0}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/roles/7", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if service.updateID != 7 || service.updateInput.SystemPrompt != "p" || service.updateInput.Status != 0 || service.updateBy != "admin" {
		t.Fatalf("id = %d input = %+v by = %q", service.updateID, service.updateInput, service.updateBy)
	}
}

func TestRoleUpdateMapsNotFound(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{err: agentrole.ErrNotFound})
	e := echo.New()
	e.PUT("/cp/api/v1/admin/roles/:id", withModelSession(h.Update))
	body := `{"description":"d","systemPrompt":"p","status":1}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/admin/roles/7", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRolePresignAvatar(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{})
	e := echo.New()
	e.POST("/cp/api/v1/admin/roles/avatar/presign", withModelSession(h.PresignAvatar))
	body := `{"filename":"frank.png","contentType":"image/png","size":1024}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles/avatar/presign", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestRolePublicListOmitsSystemPrompt(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{roles: []agentrole.Role{{
		ID: 6, RoleCode: "team", RoleNameEN: "Team", RoleNameZH: "团队",
		Description: "all", SystemPrompt: "secret", Status: 1,
	}}})
	e := echo.New()
	e.GET("/cp/api/v1/agent-roles", h.PublicList)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/agent-roles", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") || !strings.Contains(rec.Body.String(), "team") {
		t.Fatalf("body = %s", rec.Body.String())
	}
}

func TestRoleUploadAvatar(t *testing.T) {
	h := NewRoleHandler(&fakeRoleService{key: "roles/avatar/x.png"})
	e := echo.New()
	e.POST("/cp/api/v1/admin/roles/avatar/upload", withModelSession(h.UploadAvatar))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/roles/avatar/upload", strings.NewReader("png"))
	req.Header.Set(echo.HeaderContentType, "image/png")
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

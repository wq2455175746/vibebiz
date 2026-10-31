package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/clone"
)

type fakeCloneService struct {
	input     clone.CreateInput
	workspace int64
	userID    int64
	view      clone.View
	assets    []clone.AssetView
	asset     clone.AssetView
	err       error
	deleted   string
}

func (f *fakeCloneService) Create(_ context.Context, workspaceID, userID int64, input clone.CreateInput) (clone.View, error) {
	f.workspace, f.userID, f.input = workspaceID, userID, input
	return f.view, f.err
}
func (f *fakeCloneService) List(context.Context, []int64) ([]clone.View, error) {
	return []clone.View{f.view}, f.err
}
func (f *fakeCloneService) Get(context.Context, []int64, string) (clone.View, error) {
	return f.view, f.err
}
func (f *fakeCloneService) Delete(_ context.Context, _ []int64, id string) error {
	f.deleted = id
	return f.err
}
func (f *fakeCloneService) ListAssets(context.Context, []int64, string) ([]clone.AssetView, error) {
	return f.assets, f.err
}
func (f *fakeCloneService) AddAsset(_ context.Context, workspaceID, userID int64, _ string, resourceID, role string) (clone.AssetView, error) {
	f.workspace, f.userID = workspaceID, userID
	f.input = clone.CreateInput{CoverResourceID: resourceID}
	f.asset.Role = role
	return f.asset, f.err
}
func (f *fakeCloneService) DeleteAsset(context.Context, int64, int64, string, string) error {
	return f.err
}
func (f *fakeCloneService) SetCover(context.Context, int64, int64, string, string) (clone.AssetView, error) {
	return f.asset, f.err
}

func TestCloneCreateBindsAssets(t *testing.T) {
	svc := &fakeCloneService{view: clone.View{ID: "clone-1", Name: "Acme", Mode: "clone"}}
	h := NewCloneHandler(svc, nil)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/clones", h.Create, sessionForTest(3, 4))

	body := `{"name":"Acme","description":"clone acme","sourceUrl":"https://acme.example.com","assets":{"coverResourceId":"cover","galleryResourceIds":["g1"],"unitTestResourceId":"zip","docsResourceId":"doc"}}`
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/clones", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.workspace != 3 || svc.userID != 7 || svc.input.CoverResourceID != "cover" || len(svc.input.GalleryResourceIDs) != 1 {
		t.Fatalf("input = %+v workspace=%d user=%d", svc.input, svc.workspace, svc.userID)
	}
	var env struct {
		Code   int        `json:"code"`
		Result clone.View `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Code != 0 || env.Result.ID != "clone-1" {
		t.Fatalf("env = %+v err=%v", env, err)
	}
}

func TestCloneErrorsMapToEnvelope(t *testing.T) {
	cases := []struct {
		err    error
		status int
	}{
		{clone.ErrInvalid, http.StatusBadRequest},
		{clone.ErrNotFound, http.StatusNotFound},
		{clone.ErrConflict, http.StatusConflict},
		{clone.ErrRolesUnavailable, http.StatusBadRequest},
		{clone.ErrSandboxUnavailable, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		svc := &fakeCloneService{err: tc.err}
		h := NewCloneHandler(svc, nil)
		e := echo.New()
		routeWithSession(e, http.MethodGet, "/cp/api/v1/clones/:id", h.Get, sessionForTest(3))
		req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/clones/clone-1", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != tc.status {
			t.Errorf("err %v status = %d, want %d", tc.err, rec.Code, tc.status)
		}
	}
}

func TestCloneAddAssetUsesSessionWorkspace(t *testing.T) {
	svc := &fakeCloneService{asset: clone.AssetView{ID: "img-1", Role: clone.RoleGallery}}
	h := NewCloneHandler(svc, nil)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/dp/api/v1/projects/:id/clone/assets", h.AddAsset, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/clone-1/clone/assets", strings.NewReader(`{"resourceId":"img-1","role":"gallery"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.workspace != 3 || svc.userID != 7 || svc.input.CoverResourceID != "img-1" {
		t.Fatalf("workspace=%d user=%d input=%+v", svc.workspace, svc.userID, svc.input)
	}
}

var _ = errors.Is

type fakeCloneTests struct {
	result client.CloneTestResult
	err    error
}

func (f *fakeCloneTests) RunCloneTests(context.Context, string) (client.CloneTestResult, error) {
	return f.result, f.err
}

func TestCloneRunTests(t *testing.T) {
	h := NewCloneHandler(&fakeCloneService{}, nil)
	h.Tests = &fakeCloneTests{result: client.CloneTestResult{Status: "passed", ExitCode: 0, ReportPath: "docs/clone/test-report.md"}}
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/dp/api/v1/projects/:id/clone/tests/run", h.RunTests, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/dp/api/v1/projects/clone-1/clone/tests/run", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result client.CloneTestResult `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || env.Result.Status != "passed" {
		t.Fatalf("env = %+v err=%v", env, err)
	}
}

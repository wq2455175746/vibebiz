package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/model"
	"vibebiz/internal/sandboxregistry"
	"vibebiz/internal/store"
)

type fakeInitializer struct {
	sandboxID int64
	err       error
	calledID  string
}

func (f *fakeInitializer) InitProject(_ context.Context, id string) (int64, error) {
	f.calledID = id
	return f.sandboxID, f.err
}

type fakeMetaStore struct {
	items     map[string]auth.ProjectMeta
	setErr    error
	getErr    error
	deleteErr error
	deleted   []string
}

func newFakeMetaStore() *fakeMetaStore {
	return &fakeMetaStore{items: make(map[string]auth.ProjectMeta)}
}

func (f *fakeMetaStore) Set(_ context.Context, projectID string, meta auth.ProjectMeta) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.items[projectID] = meta
	return nil
}

func (f *fakeMetaStore) Get(_ context.Context, projectID string) (auth.ProjectMeta, error) {
	if f.getErr != nil {
		return auth.ProjectMeta{}, f.getErr
	}
	meta, ok := f.items[projectID]
	if !ok {
		return auth.ProjectMeta{}, auth.ErrProjectMetaNotFound
	}
	return meta, nil
}

func (f *fakeMetaStore) Delete(_ context.Context, projectID string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deleted = append(f.deleted, projectID)
	delete(f.items, projectID)
	return nil
}

func newProjectHandlerForTest(projectStore store.ProjectStore, init *fakeInitializer, metas auth.ProjectMetaStore) *ProjectHandler {
	h := NewProjectHandler(projectStore, init, nil, metas)
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	return h
}

func sessionForTest(workspaceIDs ...int64) *auth.Session {
	session := &auth.Session{ID: "sid", Realm: auth.RealmUser, UserID: 7, Username: "wanghg11", WorkspaceIDs: workspaceIDs}
	if len(workspaceIDs) > 0 {
		session.CurrentWorkspaceID = workspaceIDs[0]
	}
	return session
}

func routeWithSession(e *echo.Echo, method, path string, h echo.HandlerFunc, session *auth.Session) {
	e.Add(method, path, func(c echo.Context) error {
		c.Set("auth.session", session)
		return h(c)
	})
}

func decodeProjectEnvelope(t *testing.T, rec *httptest.ResponseRecorder) model.Project {
	t.Helper()
	var env struct {
		Code   int           `json:"code"`
		Result model.Project `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return env.Result
}

func TestCreateProjectUsesSessionWorkspaceAndOwner(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	init := &fakeInitializer{sandboxID: 42}
	metas := newFakeMetaStore()
	h := newProjectHandlerForTest(projectStore, init, metas)

	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3, 4))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"make a snake game"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	project := decodeProjectEnvelope(t, rec)
	if project.WorkspaceID != 3 || project.OwnerUserID != 7 {
		t.Fatalf("project ownership = %d/%d, want 3/7", project.WorkspaceID, project.OwnerUserID)
	}
	if init.calledID != project.ID {
		t.Fatalf("sandbox called with %q, project id %q", init.calledID, project.ID)
	}
	if project.SandboxID == nil || *project.SandboxID != 42 {
		t.Fatalf("sandbox id = %+v, want 42", project.SandboxID)
	}
	if meta, ok := metas.items[project.ID]; !ok || meta.WorkspaceID != 3 || meta.OwnerUserID != 7 {
		t.Fatalf("meta = %+v, ok = %v", meta, ok)
	}
}

func TestCreateProjectUsesCurrentWorkspace(t *testing.T) {
	metas := newFakeMetaStore()
	h := newProjectHandlerForTest(store.NewMemoryProjectStore(), &fakeInitializer{}, metas)

	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3, 4))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello","workspaceId":4}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if project := decodeProjectEnvelope(t, rec); project.WorkspaceID != 3 {
		t.Fatalf("workspace = %d, want current workspace 3", project.WorkspaceID)
	}
}

func TestCreateProjectRejectsForeignCurrentWorkspace(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore())

	session := sessionForTest(3)
	session.CurrentWorkspaceID = 99
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, session)

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should not be stored for foreign workspace")
	}
}

func TestCreateProjectRequiresPrompt(t *testing.T) {
	h := newProjectHandlerForTest(store.NewMemoryProjectStore(), &fakeInitializer{}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"  "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateProjectSandboxFailure(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{err: errors.New("boom")}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(env.Message, "boom") {
		t.Fatalf("message leaked internal error: %q", env.Message)
	}
	if env.Message != "workspace init failed" {
		t.Fatalf("message = %q, want generic message", env.Message)
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should not be stored when init fails")
	}
}

func TestCreateProjectNoSandboxReturns503(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{err: sandboxregistry.ErrNoHealthySandbox}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should not be stored without a sandbox")
	}
}

func TestCreateProjectMetaFailureRollsBack(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	metas := newFakeMetaStore()
	metas.setErr = errors.New("redis down")
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, metas)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should be removed when meta write fails")
	}
}

func TestListProjectsUsesCurrentWorkspace(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	now := time.Now()
	projectStore.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 1, CreatedAt: now.Add(-time.Minute)})
	projectStore.Create(context.Background(), &model.Project{ID: "p2", WorkspaceID: 2, CreatedAt: now})
	projectStore.Create(context.Background(), &model.Project{ID: "p3", WorkspaceID: 3, CreatedAt: now})

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects", h.List, sessionForTest(1, 2))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Result struct {
			Projects []model.Project `json:"projects"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Result.Projects) != 1 || env.Result.Projects[0].ID != "p1" {
		t.Fatalf("projects = %+v", env.Result.Projects)
	}
}

func TestGetProjectCrossWorkspaceNotFound(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	projectStore.Create(context.Background(), &model.Project{ID: "p2", WorkspaceID: 2})

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id", h.Get, sessionForTest(1))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p2", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestGetProjectOwnWorkspace(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	projectStore.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 1})

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id", h.Get, sessionForTest(1))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteProjectCrossWorkspaceNotFound(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	projectStore.Create(context.Background(), &model.Project{ID: "p2", WorkspaceID: 2})

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore())
	e := echo.New()
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/projects/:id", h.Delete, sessionForTest(1))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/projects/p2", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if _, err := projectStore.Get(context.Background(), "p2"); err != nil {
		t.Fatal("project should still exist")
	}
}

func TestDeleteProjectDeletesStoreAndMeta(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	projectStore.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 1})
	metas := newFakeMetaStore()
	metas.items["p1"] = auth.ProjectMeta{WorkspaceID: 1, OwnerUserID: 7}

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, metas)
	e := echo.New()
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/projects/:id", h.Delete, sessionForTest(1))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/projects/p1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if _, err := projectStore.Get(context.Background(), "p1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("project still exists: %v", err)
	}
	if len(metas.deleted) != 1 || metas.deleted[0] != "p1" {
		t.Fatalf("meta not deleted: %v", metas.deleted)
	}
}

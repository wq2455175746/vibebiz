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

	"vibebiz/internal/agentrole"
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

type fakeAgentRoles struct {
	roles []agentrole.Role
}

func (f *fakeAgentRoles) ListActive(context.Context) ([]agentrole.Role, error) {
	return f.roles, nil
}

func (f *fakeAgentRoles) GetActive(_ context.Context, id int64) (agentrole.Role, error) {
	for _, role := range f.roles {
		if role.ID == id && role.Status == 1 {
			return role, nil
		}
	}
	return agentrole.Role{}, agentrole.ErrNotFound
}

func newProjectHandlerForTest(projectStore store.ProjectStore, init *fakeInitializer, metas auth.ProjectMetaStore, roles AgentRoleStore) *ProjectHandler {
	h := NewProjectHandler(projectStore, init, nil, metas, roles)
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
	h := newProjectHandlerForTest(projectStore, init, metas, &fakeAgentRoles{})

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
	h := newProjectHandlerForTest(store.NewMemoryProjectStore(), &fakeInitializer{}, metas, &fakeAgentRoles{})

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
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})

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
	h := newProjectHandlerForTest(store.NewMemoryProjectStore(), &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
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
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{err: errors.New("boom")}, newFakeMetaStore(), &fakeAgentRoles{})
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
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{err: sandboxregistry.ErrNoHealthySandbox}, newFakeMetaStore(), &fakeAgentRoles{})
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
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, metas, &fakeAgentRoles{})
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

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
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

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
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

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
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

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
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

	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, metas, &fakeAgentRoles{})
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

func TestCreateProjectStoresAgentRoles(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
		{ID: 4, RoleCode: "emma", RoleNameZH: "产品经理", Status: 1},
	}}
	h := NewProjectHandler(projectStore, &fakeInitializer{sandboxID: 42}, nil, newFakeMetaStore(), roles)
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello","agentRoleIds":[2,4]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := projectStore.Get(context.Background(), decodeProjectEnvelope(t, rec).ID)
	if len(got.AgentRoleIDs) != 2 || *got.DefaultAgentRoleID != 2 {
		t.Fatalf("roles = %+v", got)
	}
}

func TestUpdateProjectAgentRolesNormalizesTeam(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	_ = projectStore.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3})
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 6, RoleCode: "team", RoleNameZH: "团队", Status: 1},
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
	}}
	h := NewProjectHandler(projectStore, &fakeInitializer{}, nil, newFakeMetaStore(), roles)
	e := echo.New()
	routeWithSession(e, http.MethodPut, "/cp/api/v1/projects/:id/agent-roles", h.UpdateAgentRoles, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/projects/p1/agent-roles", strings.NewReader(`{"agentRoleIds":[2,6]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	got, _ := projectStore.Get(context.Background(), "p1")
	if len(got.AgentRoleIDs) != 1 || got.AgentRoleIDs[0] != 6 {
		t.Fatalf("team not normalized: %+v", got.AgentRoleIDs)
	}
}

type fakeCoverURLSigner struct{}

func (fakeCoverURLSigner) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example/" + key, nil
}

func TestListProjectsAttachesCoverURL(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	_ = projectStore.Create(context.Background(), &model.Project{
		ID: "p1", WorkspaceID: 3, CreatedAt: time.Now(),
		CoverS3Path: "projects/p1/cover.png",
	})
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
	h.Covers = fakeCoverURLSigner{}
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects", h.List, sessionForTest(3))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Projects []model.Project `json:"projects"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Projects) != 1 || env.Result.Projects[0].CoverURL != "https://s3.example/projects/p1/cover.png" {
		t.Fatalf("projects = %+v", env.Result.Projects)
	}
}

type fakeStageInitializer struct {
	created []string
	deleted []string
	err     error
}

func (f *fakeStageInitializer) CreateInitial(_ context.Context, projectID string) error {
	f.created = append(f.created, projectID)
	return f.err
}

func (f *fakeStageInitializer) DeleteByProject(_ context.Context, projectID string) error {
	f.deleted = append(f.deleted, projectID)
	return nil
}

func enterpriseRolesForTest() []agentrole.Role {
	return []agentrole.Role{
		{ID: 6, RoleCode: "team", RoleNameZH: "团队", Status: 1},
		{ID: 4, RoleCode: "emma", RoleNameZH: "产品经理", Status: 1},
		{ID: 2, RoleCode: "bob", RoleNameZH: "系统架构师", Status: 1},
		{ID: 7, RoleCode: "senior", RoleNameZH: "高级工程师", Status: 1},
		{ID: 8, RoleCode: "qa", RoleNameZH: "测试开发工程师", Status: 1},
		{ID: 1, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
	}
}

type fakeHarnessImporter struct {
	id    string
	err   error
	calls int
}

func (f *fakeHarnessImporter) Import(_ context.Context, projectID string) error {
	f.calls++
	f.id = projectID
	return f.err
}

func TestCreateEnterpriseProjectImportsHarness(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	roles := &fakeAgentRoles{roles: enterpriseRolesForTest()}
	harness := &fakeHarnessImporter{}
	stages := &fakeStageInitializer{}
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{sandboxID: 42}, newFakeMetaStore(), roles)
	h.Harness = harness
	h.Stages = stages
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"enterprise","mode":"enterprise","agentRoleIds":[6]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	project := decodeProjectEnvelope(t, rec)
	if project.Mode != model.ProjectModeEnterprise {
		t.Fatalf("mode = %q", project.Mode)
	}
	if harness.calls != 1 || harness.id != project.ID {
		t.Fatalf("harness calls=%d id=%q, project=%q", harness.calls, harness.id, project.ID)
	}
	if len(stages.created) != 1 || stages.created[0] != project.ID {
		t.Fatalf("stages created = %v", stages.created)
	}
	stored, err := projectStore.Get(context.Background(), project.ID)
	if err != nil || stored.Mode != model.ProjectModeEnterprise {
		t.Fatalf("stored = %+v err=%v", stored, err)
	}
	if len(stored.AgentRoleIDs) != 6 || stored.AgentRoleIDs[0] != 6 {
		t.Fatalf("enterprise role ids = %v", stored.AgentRoleIDs)
	}
}

func TestCreateEnterpriseProjectRequiresTeamRole(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
	}}
	harness := &fakeHarnessImporter{}
	init := &fakeInitializer{sandboxID: 42}
	h := newProjectHandlerForTest(projectStore, init, newFakeMetaStore(), roles)
	h.Harness = harness
	h.Stages = &fakeStageInitializer{}
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"enterprise","mode":"enterprise","agentRoleIds":[2]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if init.calledID != "" || harness.calls != 0 {
		t.Fatalf("should not initialize or import: init=%q harness=%d", init.calledID, harness.calls)
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should not be stored")
	}
}

func TestCreateStandardProjectDoesNotImportHarness(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	harness := &fakeHarnessImporter{}
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{sandboxID: 42}, newFakeMetaStore(), &fakeAgentRoles{})
	h.Harness = harness
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"small app"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if project := decodeProjectEnvelope(t, rec); project.Mode != model.ProjectModeStandard {
		t.Fatalf("mode = %q", project.Mode)
	}
	if harness.calls != 0 {
		t.Fatalf("harness calls = %d", harness.calls)
	}
}

func TestCreateEnterpriseProjectHarnessFailureRollsBack(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	metas := newFakeMetaStore()
	roles := &fakeAgentRoles{roles: enterpriseRolesForTest()}
	harness := &fakeHarnessImporter{err: errors.New("s3 down")}
	stages := &fakeStageInitializer{}
	h := newProjectHandlerForTest(projectStore, &fakeInitializer{sandboxID: 42}, metas, roles)
	h.Harness = harness
	h.Stages = stages
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"enterprise","mode":"enterprise","agentRoleIds":[6]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(projectStore.List(context.Background())) != 0 {
		t.Fatal("project should be rolled back")
	}
	if len(metas.deleted) != 1 {
		t.Fatalf("meta should be deleted: %v", metas.deleted)
	}
	if len(stages.deleted) != 1 {
		t.Fatalf("stages should be deleted: %v", stages.deleted)
	}
}

func TestCreateEnterpriseProjectWithoutHarnessImporterFails(t *testing.T) {
	roles := &fakeAgentRoles{roles: enterpriseRolesForTest()}
	h := newProjectHandlerForTest(store.NewMemoryProjectStore(), &fakeInitializer{sandboxID: 42}, newFakeMetaStore(), roles)
	h.Stages = &fakeStageInitializer{}
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects", h.Create, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"enterprise","mode":"enterprise","agentRoleIds":[6]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateEnterpriseProjectAgentRolesRequiresTeam(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	_ = projectStore.Create(context.Background(), &model.Project{
		ID: "p1", WorkspaceID: 3, Mode: model.ProjectModeEnterprise,
	})
	roles := &fakeAgentRoles{roles: []agentrole.Role{
		{ID: 2, RoleCode: "alex", RoleNameZH: "全栈工程师", Status: 1},
	}}
	h := NewProjectHandler(projectStore, &fakeInitializer{}, nil, newFakeMetaStore(), roles)
	e := echo.New()
	routeWithSession(e, http.MethodPut, "/cp/api/v1/projects/:id/agent-roles", h.UpdateAgentRoles, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/projects/p1/agent-roles", strings.NewReader(`{"agentRoleIds":[2]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateProjectPreviewPath(t *testing.T) {
	cases := []struct {
		name       string
		body       string
		wantStatus int
		wantPath   string
	}{
		{name: "valid nested path", body: `{"previewPath":"dist/index.html"}`, wantStatus: http.StatusOK, wantPath: "dist/index.html"},
		{name: "clear path", body: `{"previewPath":""}`, wantStatus: http.StatusOK, wantPath: ""},
		{name: "parent traversal", body: `{"previewPath":"../secret.html"}`, wantStatus: http.StatusBadRequest},
		{name: "absolute path", body: `{"previewPath":"/etc/passwd"}`, wantStatus: http.StatusBadRequest},
		{name: "nested traversal", body: `{"previewPath":"docs/../../secret.html"}`, wantStatus: http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			projectStore := store.NewMemoryProjectStore()
			project := &model.Project{ID: "p1", WorkspaceID: 3, Name: "demo", State: model.ProjectStateReady}
			if err := projectStore.Create(context.Background(), project); err != nil {
				t.Fatal(err)
			}
			h := newProjectHandlerForTest(projectStore, &fakeInitializer{}, newFakeMetaStore(), &fakeAgentRoles{})
			e := echo.New()
			routeWithSession(e, http.MethodPut, "/cp/api/v1/projects/:id/entry", h.UpdatePreviewPath, sessionForTest(3))

			req := httptest.NewRequest(http.MethodPut, "/cp/api/v1/projects/p1/entry", strings.NewReader(tc.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if tc.wantStatus != http.StatusOK {
				return
			}
			saved, err := projectStore.Get(context.Background(), "p1")
			if err != nil {
				t.Fatal(err)
			}
			if saved.PreviewPath != tc.wantPath {
				t.Fatalf("preview path = %q, want %q", saved.PreviewPath, tc.wantPath)
			}
		})
	}
}

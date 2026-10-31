package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
	"vibebiz/internal/model"
	"vibebiz/internal/projectstage"
	"vibebiz/internal/store"
)

type fakeStageStore struct {
	stages      map[string]model.ProjectStage
	definitions []projectstage.Definition
	startErr    error
	completeErr error
	nextKey     string
	started     []string
	completed   []string
}

func newFakeStageStore() *fakeStageStore {
	return newFakeStageStoreForMode(model.ProjectModeEnterprise)
}

func newFakeStageStoreForMode(mode string) *fakeStageStore {
	items := make(map[string]model.ProjectStage)
	for _, stage := range projectstage.InitialStagesForMode("p1", mode) {
		items[stage.StageKey] = stage
	}
	return &fakeStageStore{stages: items, definitions: projectstage.DefinitionsForMode(mode)}
}

func (f *fakeStageStore) CreateInitial(context.Context, string) error                { return nil }
func (f *fakeStageStore) CreateInitialForMode(context.Context, string, string) error { return nil }
func (f *fakeStageStore) DeleteByProject(context.Context, string) error              { return nil }

func (f *fakeStageStore) List(_ context.Context, _ string) ([]model.ProjectStage, error) {
	out := make([]model.ProjectStage, 0, len(f.definitions))
	for _, definition := range f.definitions {
		out = append(out, f.stages[definition.Key])
	}
	return out, nil
}

func (f *fakeStageStore) Get(_ context.Context, _, stageKey string) (*model.ProjectStage, error) {
	stage, ok := f.stages[stageKey]
	if !ok {
		return nil, projectstage.ErrStageNotFound
	}
	return &stage, nil
}

func (f *fakeStageStore) Start(_ context.Context, _, stageKey string, now time.Time) (bool, error) {
	if f.startErr != nil {
		return false, f.startErr
	}
	stage := f.stages[stageKey]
	if stage.Status != model.ProjectStageActive || stage.StartedAt != nil {
		return false, nil
	}
	stage.StartedAt = &now
	f.stages[stageKey] = stage
	f.started = append(f.started, stageKey)
	return true, nil
}

func (f *fakeStageStore) Complete(_ context.Context, _, stageKey string, now time.Time) (string, error) {
	if f.completeErr != nil {
		return "", f.completeErr
	}
	stage := f.stages[stageKey]
	stage.Status = model.ProjectStageDone
	stage.CompletedAt = &now
	f.stages[stageKey] = stage
	f.completed = append(f.completed, stageKey)
	if f.nextKey != "" {
		next := f.stages[f.nextKey]
		next.Status = model.ProjectStageActive
		f.stages[f.nextKey] = next
	}
	return f.nextKey, nil
}

type fakeStageSandbox struct {
	content string
	readErr error
	active  bool
}

func (f *fakeStageSandbox) ReadFile(context.Context, string, string) (*client.FileContent, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return &client.FileContent{Content: f.content}, nil
}

func (f *fakeStageSandbox) ChatStatus(context.Context, string) (client.ChatStatus, error) {
	return client.ChatStatus{Active: f.active}, nil
}

func newStageHandlerForTest(project *model.Project, stages *fakeStageStore, sandbox *fakeStageSandbox) (*StageHandler, *echo.Echo) {
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewStageHandler(projects, stages, sandbox)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/stages", h.List, sessionForTest(project.WorkspaceID))
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/stages/:stageKey/start", h.Start, sessionForTest(project.WorkspaceID))
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/stages/:stageKey/complete", h.Complete, sessionForTest(project.WorkspaceID))
	return h, e
}

func enterpriseStageProject() *model.Project {
	return &model.Project{ID: "p1", WorkspaceID: 3, Mode: model.ProjectModeEnterprise}
}

func TestListStages(t *testing.T) {
	h, e := newStageHandlerForTest(enterpriseStageProject(), newFakeStageStore(), &fakeStageSandbox{})
	_ = h

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/stages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Mode         string `json:"mode"`
			CurrentStage string `json:"currentStage"`
			Stages       []struct {
				StageKey string `json:"stageKey"`
				Title    string `json:"title"`
				Status   string `json:"status"`
			} `json:"stages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.Mode != model.ProjectModeEnterprise || env.Result.CurrentStage != "requirements" {
		t.Fatalf("result = %+v", env.Result)
	}
	if len(env.Result.Stages) != 7 || env.Result.Stages[0].Title != "需求梳理" || env.Result.Stages[5].Status != model.ProjectStagePlaceholder {
		t.Fatalf("stages = %+v", env.Result.Stages)
	}
}

func TestListStagesRejectsStandardProject(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3, Mode: model.ProjectModeStandard}
	_, e := newStageHandlerForTest(project, newFakeStageStore(), &fakeStageSandbox{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/stages", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestStartStage(t *testing.T) {
	stages := newFakeStageStore()
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/start", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(stages.started) != 1 || stages.stages["requirements"].StartedAt == nil {
		t.Fatalf("stage not started: %+v", stages.stages["requirements"])
	}
}

func TestCompleteStageRequiresArtifact(t *testing.T) {
	stages := newFakeStageStore()
	now := time.Now().UTC()
	stage := stages.stages["requirements"]
	stage.StartedAt = &now
	stages.stages["requirements"] = stage
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "  "})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/complete", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(stages.completed) != 0 {
		t.Fatal("stage should not complete")
	}
}

func TestCompleteStageRejectsRunningAgent(t *testing.T) {
	stages := newFakeStageStore()
	now := time.Now().UTC()
	stage := stages.stages["requirements"]
	stage.StartedAt = &now
	stages.stages["requirements"] = stage
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "# done", active: true})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/complete", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompleteStageAdvances(t *testing.T) {
	stages := newFakeStageStore()
	stages.nextKey = "architecture"
	now := time.Now().UTC()
	stage := stages.stages["requirements"]
	stage.StartedAt = &now
	stages.stages["requirements"] = stage
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "# done"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/complete", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(stages.completed) != 1 || stages.completed[0] != "requirements" {
		t.Fatalf("completed = %v", stages.completed)
	}
	if stages.stages["architecture"].Status != model.ProjectStageActive {
		t.Fatalf("next stage = %+v", stages.stages["architecture"])
	}
}

func TestCompleteStageRejectsLockedStage(t *testing.T) {
	stages := newFakeStageStore()
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "# done"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/architecture/complete", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompleteStageRejectsPlaceholder(t *testing.T) {
	stages := newFakeStageStore()
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "# done"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/artifacts/complete", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompleteStageConflictFromStore(t *testing.T) {
	stages := newFakeStageStore()
	stages.completeErr = projectstage.ErrStageConflict
	now := time.Now().UTC()
	stage := stages.stages["requirements"]
	stage.StartedAt = &now
	stages.stages["requirements"] = stage
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{content: "# done"})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/complete", nil))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCompleteStageMissingFileIsBadRequest(t *testing.T) {
	stages := newFakeStageStore()
	now := time.Now().UTC()
	stage := stages.stages["requirements"]
	stage.StartedAt = &now
	stages.stages["requirements"] = stage
	_, e := newStageHandlerForTest(enterpriseStageProject(), stages, &fakeStageSandbox{readErr: &client.APIError{Status: http.StatusNotFound}})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/requirements/complete", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func cloneStageProject() *model.Project {
	return &model.Project{ID: "p1", WorkspaceID: 3, Mode: model.ProjectModeClone}
}

func TestListCloneStages(t *testing.T) {
	_, e := newStageHandlerForTest(cloneStageProject(), newFakeStageStoreForMode(model.ProjectModeClone), &fakeStageSandbox{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/stages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Mode   string `json:"mode"`
			Stages []struct {
				StageKey  string `json:"stageKey"`
				Title     string `json:"title"`
				Status    string `json:"status"`
				Workspace string `json:"workspace"`
			} `json:"stages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Result.Mode != model.ProjectModeClone || len(env.Result.Stages) != 5 {
		t.Fatalf("result = %+v", env.Result)
	}
	if env.Result.Stages[0].StageKey != "base_design" || env.Result.Stages[2].Workspace != projectstage.WorkspaceTest {
		t.Fatalf("stages = %+v", env.Result.Stages)
	}
}

func TestCloneArtifactStageCompletesWithoutArtifactFile(t *testing.T) {
	stages := newFakeStageStoreForMode(model.ProjectModeClone)
	artifact := stages.stages["artifacts"]
	artifact.Status = model.ProjectStageActive
	now := time.Now().UTC()
	artifact.StartedAt = &now
	stages.stages["artifacts"] = artifact
	_, e := newStageHandlerForTest(cloneStageProject(), stages, &fakeStageSandbox{})
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/stages/artifacts/complete", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if stages.stages["artifacts"].Status != model.ProjectStageDone {
		t.Fatalf("artifact stage = %+v", stages.stages["artifacts"])
	}
}

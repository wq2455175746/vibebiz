package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/dao/model"
	"vibebiz/internal/evolution"
)

func newEvolutionHandlerForTest(t *testing.T) (*EvolutionHandler, *evolution.MemoryStore) {
	t.Helper()
	store := evolution.NewMemoryStore()
	service := evolution.NewService(store)
	return NewEvolutionHandler(service), store
}

func decodeEvolutionEnvelope(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var env struct {
		Code   int            `json:"code"`
		Result map[string]any `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return env.Result
}

func TestEvolutionListReturnsVersions(t *testing.T) {
	h, store := newEvolutionHandlerForTest(t)
	if err := store.CreateVersion(context.Background(), &model.EvolutionVersion{
		PublicID: "v-1", UserID: "7", UserName: "wanghg11", Label: "v7.1", Status: "preview",
	}); err != nil {
		t.Fatal(err)
	}
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/evolve", h.List, sessionForTest())

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/evolve", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	result := decodeEvolutionEnvelope(t, rec)
	versions, ok := result["versions"].([]any)
	if !ok || len(versions) != 1 {
		t.Fatalf("versions = %+v", result["versions"])
	}
	if result["originVersion"] != evolution.OriginVersion {
		t.Fatalf("origin = %+v", result["originVersion"])
	}
}

func TestEvolutionSubmitProposalCreatesVersion(t *testing.T) {
	h, _ := newEvolutionHandlerForTest(t)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/evolve/proposals", h.SubmitProposal, sessionForTest())

	body := strings.NewReader(`{"content":"删除按钮点击没有反应"}`)
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/evolve/proposals", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	result := decodeEvolutionEnvelope(t, rec)
	proposal, ok := result["proposal"].(map[string]any)
	if !ok || proposal["kind"] != evolution.KindBug {
		t.Fatalf("proposal = %+v", result["proposal"])
	}
	version, ok := result["version"].(map[string]any)
	if !ok || version["status"] != evolution.VersionPreview {
		t.Fatalf("version = %+v", result["version"])
	}
}

func TestEvolutionAdminTaskFlow(t *testing.T) {
	h, store := newEvolutionHandlerForTest(t)
	service := h.Service
	_, version, err := service.SubmitProposal(context.Background(), evolution.Viewer{ID: "7", Name: "wanghg11"}, "首页卡片布局再紧凑一点")
	if err != nil || version == nil {
		t.Fatalf("submit proposal: %v", err)
	}

	e := echo.New()
	e.POST("/cp/api/v1/admin/evolve/tasks", h.AdminSubmitTask)
	e.PATCH("/cp/api/v1/admin/evolve/tasks/:id", h.AdminUpdateTask)

	body := strings.NewReader(`{"versionId":"` + version.PublicID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/admin/evolve/tasks", body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}
	result := decodeEvolutionEnvelope(t, rec)
	task, ok := result["task"].(map[string]any)
	if !ok || task["status"] != evolution.TaskQueued {
		t.Fatalf("task = %+v", result["task"])
	}
	taskID, _ := task["id"].(string)

	body = strings.NewReader(`{"status":"coding"}`)
	req = httptest.NewRequest(http.MethodPatch, "/cp/api/v1/admin/evolve/tasks/"+taskID, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body = %s", rec.Code, rec.Body.String())
	}
	stored, err := store.GetTask(context.Background(), taskID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != evolution.TaskCoding {
		t.Fatalf("stored status = %q", stored.Status)
	}
}

func TestEvolutionListRequiresSession(t *testing.T) {
	h, _ := newEvolutionHandlerForTest(t)
	e := echo.New()
	e.GET("/cp/api/v1/evolve", h.List)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/evolve", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestEvolutionEnsureDraft(t *testing.T) {
	h, _ := newEvolutionHandlerForTest(t)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/evolve/draft", h.EnsureDraft, sessionForTest())

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/evolve/draft", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	result := decodeEvolutionEnvelope(t, rec)
	version, ok := result["version"].(map[string]any)
	if !ok || version["status"] != evolution.VersionDraft {
		t.Fatalf("version = %+v", result["version"])
	}
}

package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
	"vibebiz/internal/store"
)

func newPublishHandlerForTest(t *testing.T, project *model.Project) (*echo.Echo, string) {
	t.Helper()
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	source := filepath.Join(workspaces, project.ID)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("<h1>game</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	h := NewPublishHandler(publish.NewService(workspaces, published), projects, "")
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/publish", h.Publish, sessionForTest(project.WorkspaceID))
	e.GET("/p/:slug/*", h.Preview)
	return e, published
}

func newPublishRequest(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func TestPublishProjectAndPreview(t *testing.T) {
	project := &model.Project{ID: "p1", Name: "Demo Game", WorkspaceID: 3}
	e, published := newPublishHandlerForTest(t, project)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newPublishRequest("/cp/api/v1/projects/p1/publish", `{"title":"毕业季小游戏","description":"三分钟通关","tags":["答题","校园"]}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("publish status = %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"url":"http://example.com/p/p1/"`) {
		t.Fatalf("publish body = %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(published, "p1", "index.html")); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/p1/", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "毕业季小游戏") {
		t.Fatalf("preview status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/p1/game/index.html", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<h1>game</h1>") {
		t.Fatalf("game preview status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestPublishRejectsForeignWorkspace(t *testing.T) {
	project := &model.Project{ID: "p1", Name: "Demo", WorkspaceID: 3}
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	if err := os.MkdirAll(filepath.Join(workspaces, "p1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaces, "p1", "index.html"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	h := NewPublishHandler(publish.NewService(workspaces, filepath.Join(root, "published")), projects, "")
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/publish", h.Publish, sessionForTest(4))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, newPublishRequest("/cp/api/v1/projects/p1/publish", `{"title":"x"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

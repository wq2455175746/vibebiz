package handler

import (
	"context"
	"encoding/json"
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
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want *", got)
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

func TestListPublishedFiltersWorkspaceAndBuildsShareURL(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	for _, id := range []string{"p1", "p2"} {
		if err := os.MkdirAll(filepath.Join(workspaces, id), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(workspaces, id, "index.html"), []byte("<h1>"+id+"</h1>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	service := publish.NewService(workspaces, published)
	if _, err := service.Publish(context.Background(), "p1", publish.Input{Title: "我的分享页"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Publish(context.Background(), "p2", publish.Input{Title: "别人的分享页"}); err != nil {
		t.Fatal(err)
	}
	projects := store.NewMemoryProjectStore()
	for _, project := range []*model.Project{
		{ID: "p1", Name: "Demo", WorkspaceID: 3, CoverURL: "https://cdn.example/p1.png"},
		{ID: "p2", Name: "Other", WorkspaceID: 4},
	} {
		if err := projects.Create(context.Background(), project); err != nil {
			t.Fatal(err)
		}
	}
	h := NewPublishHandler(service, projects, "https://play.example.com/")
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/me/published-projects", h.ListPublished, sessionForTest(3))
	routeWithSession(e, http.MethodGet, "/cp/api/v1/me/published-projects-other", h.ListPublished, sessionForTest(4))

	var got struct {
		Code   int `json:"code"`
		Result struct {
			Projects []struct {
				ProjectID   string `json:"projectId"`
				Name        string `json:"name"`
				CoverURL    string `json:"coverUrl"`
				PublishedAt string `json:"publishedAt"`
				URL         string `json:"url"`
			} `json:"projects"`
		} `json:"result"`
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/published-projects", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Result.Projects) != 1 {
		t.Fatalf("projects = %+v", got.Result.Projects)
	}
	item := got.Result.Projects[0]
	if item.ProjectID != "p1" || item.Name != "我的分享页" || item.CoverURL != "https://cdn.example/p1.png" {
		t.Fatalf("item = %+v", item)
	}
	if item.URL != "https://play.example.com/p/p1/" || item.PublishedAt == "" {
		t.Fatalf("item = %+v", item)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/published-projects-other", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "别人的分享页") {
		t.Fatalf("other status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDeletePublishedRemovesSharePageAndRejectsForeignWorkspace(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	project := &model.Project{ID: "p1", Name: "Demo", WorkspaceID: 3}
	source := filepath.Join(workspaces, project.ID)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "index.html"), []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), project); err != nil {
		t.Fatal(err)
	}
	service := publish.NewService(workspaces, published)
	if _, err := service.Publish(context.Background(), project.ID, publish.Input{Title: "作品"}); err != nil {
		t.Fatal(err)
	}
	h := NewPublishHandler(service, projects, "")
	e := echo.New()
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/projects/:id/publish", h.DeletePublished, sessionForTest(3))
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/foreign/projects/:id/publish", h.DeletePublished, sessionForTest(4))

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/foreign/projects/p1/publish", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(published, project.ID)); err != nil {
		t.Fatal("foreign delete must not remove published copy")
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/projects/p1/publish", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(published, project.ID)); !os.IsNotExist(err) {
		t.Fatal("published copy should be removed")
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/projects/p1/publish", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d body=%s", rec.Code, rec.Body.String())
	}
}

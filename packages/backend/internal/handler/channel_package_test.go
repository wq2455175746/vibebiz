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

	"vibebiz/internal/channelpkg"
	"vibebiz/internal/dao"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type channelPackageRepoStub struct {
	items map[string]*daomodel.ProjectChannelPackage
}

func newChannelPackageRepoStub(items ...*daomodel.ProjectChannelPackage) *channelPackageRepoStub {
	stub := &channelPackageRepoStub{items: make(map[string]*daomodel.ProjectChannelPackage)}
	for _, item := range items {
		stub.items[item.PublicID] = item
	}
	return stub
}

func (s *channelPackageRepoStub) Create(_ context.Context, item *daomodel.ProjectChannelPackage) error {
	s.items[item.PublicID] = item
	return nil
}

func (s *channelPackageRepoStub) Update(_ context.Context, item *daomodel.ProjectChannelPackage) error {
	s.items[item.PublicID] = item
	return nil
}

func (s *channelPackageRepoStub) GetByPublicID(_ context.Context, publicID string) (*daomodel.ProjectChannelPackage, error) {
	item, ok := s.items[publicID]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return item, nil
}

func (s *channelPackageRepoStub) ListByProject(_ context.Context, projectPublicID string) ([]daomodel.ProjectChannelPackage, error) {
	out := make([]daomodel.ProjectChannelPackage, 0, len(s.items))
	for _, item := range s.items {
		if item.ProjectPublicID == projectPublicID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (s *channelPackageRepoStub) Delete(_ context.Context, publicID string) error {
	if _, ok := s.items[publicID]; !ok {
		return dao.ErrNotFound
	}
	delete(s.items, publicID)
	return nil
}

func newChannelPackageTestService(dir string, items ...*daomodel.ProjectChannelPackage) *channelpkg.Service {
	return channelpkg.NewService(channelpkg.ServiceConfig{
		Repo:        newChannelPackageRepoStub(items...),
		PackagesDir: dir,
	})
}

func TestChannelPackageGenerateRequiresWorkspaceMember(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewChannelPackageHandler(nil, projects)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/channel-packages", h.Generate, sessionForTest(99))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/channel-packages", strings.NewReader(`{"channel":"wechat_minigame","sourceType":"html5"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuidesReturnsThreeChannels(t *testing.T) {
	h := NewChannelPackageHandler(nil, nil)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/channel-guides", h.Guides, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/channel-guides", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") || !strings.Contains(rec.Body.String(), "steam_desktop") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelPackageDownloadRejectsOtherWorkspace(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewChannelPackageHandler(newChannelPackageTestService(t.TempDir(), &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1", Status: daomodel.ChannelPackageStatusReady,
	}), projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages/:packageId/download", h.Download, sessionForTest(99))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages/pkg-1/download", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "40401") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelPackageDownloadRejectsRecordFromOtherProject(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewChannelPackageHandler(newChannelPackageTestService(t.TempDir(), &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p2", Status: daomodel.ChannelPackageStatusReady,
	}), projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages/:packageId/download", h.Download, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages/pkg-1/download", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "40409") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelPackageDownloadRejectsUnconfinedPath(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	h := NewChannelPackageHandler(newChannelPackageTestService(t.TempDir(), &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1", Status: daomodel.ChannelPackageStatusReady, FilePath: "../escape.zip",
	}), projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages/:packageId/download", h.Download, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages/pkg-1/download", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelPackageDownloadServesReadyPackage(t *testing.T) {
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	projects := store.NewMemoryProjectStore()
	_ = projects.Create(context.Background(), project)
	dir := t.TempDir()
	target := filepath.Join(dir, "p1", "wechat_minigame", "pkg-1.zip")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("zip-bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewChannelPackageHandler(newChannelPackageTestService(dir, &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1", Channel: "wechat_minigame", Version: "v1",
		Status: daomodel.ChannelPackageStatusReady, FilePath: "p1/wechat_minigame/pkg-1.zip",
	}), projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages/:packageId/download", h.Download, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages/pkg-1/download", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "zip-bytes" {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get(echo.HeaderContentType); got != "application/zip" {
		t.Fatalf("content-type = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("nosniff = %q", got)
	}
	if got := rec.Header().Get(echo.HeaderContentDisposition); !strings.Contains(got, "attachment") || !strings.Contains(got, ".zip") {
		t.Fatalf("content-disposition = %q", got)
	}
}

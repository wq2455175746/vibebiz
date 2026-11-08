package handler

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/channelpkg"
	"vibebiz/internal/channelpkg/core"
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

type channelPackageAdapterStub struct {
	level    string
	warnings []string
}

func (a channelPackageAdapterStub) Channel() string { return core.ChannelWeChat }

func (a channelPackageAdapterStub) Validate(core.SourceInfo) core.Report {
	return core.Report{Level: a.level, Warnings: append([]string{}, a.warnings...)}
}

func (a channelPackageAdapterStub) Build(_ context.Context, source core.SourceInfo, out io.Writer) (core.Manifest, error) {
	zw := zip.NewWriter(out)
	w, err := zw.Create("game.js")
	if err != nil {
		return core.Manifest{}, err
	}
	if _, err := w.Write([]byte("game")); err != nil {
		return core.Manifest{}, err
	}
	if err := zw.Close(); err != nil {
		return core.Manifest{}, err
	}
	return core.Manifest{
		ProjectID: source.ProjectID, Channel: core.ChannelWeChat, SourceType: source.SourceType,
		SourceVersion: source.Version, Level: a.level, Warnings: append([]string{}, a.warnings...),
	}, nil
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

func TestChannelPackageGenerateReturnsPackageDTO(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "p1")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		t.Fatal(err)
	}
	writeChannelPackageFile(t, workspace, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	writeChannelPackageFile(t, workspace, "game.js", `document.getElementById('game').getContext('2d')`)

	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}); err != nil {
		t.Fatal(err)
	}
	service := channelpkg.NewService(channelpkg.ServiceConfig{
		Repo:          newChannelPackageRepoStub(),
		WorkspacesDir: root,
		PackagesDir:   filepath.Join(root, "packages"),
		Adapters: map[string]core.Adapter{
			core.ChannelWeChat: channelPackageAdapterStub{level: core.LevelB, warnings: []string{"network_fetch"}},
		},
		Now:   func() time.Time { return time.Unix(1700000000, 0).UTC() },
		NewID: func() string { return "pkg-1" },
	})
	h := NewChannelPackageHandler(service, projects)
	e := echo.New()
	routeWithSession(e, http.MethodPost, "/cp/api/v1/projects/:id/channel-packages", h.Generate, sessionForTest(3))
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects/p1/channel-packages", strings.NewReader(`{"channel":"wechat_minigame","sourceType":"html5"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code   int `json:"code"`
		Result struct {
			Package channelPackageResponse `json:"package"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	pkg := env.Result.Package
	if pkg.ID != "pkg-1" || pkg.Status != daomodel.ChannelPackageStatusReady || pkg.Level != core.LevelB {
		t.Fatalf("package = %+v", pkg)
	}
	if len(pkg.Warnings) != 1 || pkg.Warnings[0] != "network_fetch" {
		t.Fatalf("warnings = %v", pkg.Warnings)
	}
	if pkg.DownloadURL != "/cp/api/v1/projects/p1/channel-packages/pkg-1/download" {
		t.Fatalf("downloadUrl = %s", pkg.DownloadURL)
	}
	if pkg.GuideURL != "/settings/channel-guides?channel=wechat_minigame" {
		t.Fatalf("guideUrl = %s", pkg.GuideURL)
	}
}

func TestChannelPackageListReturnsPackages(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}); err != nil {
		t.Fatal(err)
	}
	manifest, err := json.Marshal(core.Manifest{Level: core.LevelB, Warnings: []string{"network_fetch"}})
	if err != nil {
		t.Fatal(err)
	}
	service := newChannelPackageTestService(t.TempDir(), &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1", Channel: core.ChannelWeChat, SourceType: core.SourceHTML5,
		Status: daomodel.ChannelPackageStatusReady, Version: "v1", Manifest: manifest,
		CreatedAt: time.Unix(1700000000, 0).UTC(),
	})
	h := NewChannelPackageHandler(service, projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages", h.List, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result struct {
			Packages []channelPackageResponse `json:"packages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if len(env.Result.Packages) != 1 {
		t.Fatalf("packages = %+v", env.Result.Packages)
	}
	pkg := env.Result.Packages[0]
	if pkg.ID != "pkg-1" || pkg.Level != core.LevelB || len(pkg.Warnings) != 1 {
		t.Fatalf("package = %+v", pkg)
	}
}

func TestChannelPackageDeleteRemovesRecordAndFile(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "p1", "wechat_minigame", "pkg-1.zip")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	writeChannelPackageFile(t, filepath.Dir(target), "pkg-1.zip", "zip-bytes")
	repo := newChannelPackageRepoStub(&daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1", Channel: core.ChannelWeChat,
		Status: daomodel.ChannelPackageStatusReady, FilePath: "p1/wechat_minigame/pkg-1.zip",
	})
	service := channelpkg.NewService(channelpkg.ServiceConfig{Repo: repo, PackagesDir: dir})
	h := NewChannelPackageHandler(service, projects)
	e := echo.New()
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/projects/:id/channel-packages/:packageId", h.Delete, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/cp/api/v1/projects/p1/channel-packages/pkg-1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var env struct {
		Code   int `json:"code"`
		Result struct {
			Deleted bool `json:"deleted"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Code != 0 || !env.Result.Deleted {
		t.Fatalf("delete response = %s", rec.Body.String())
	}
	if _, ok := repo.items["pkg-1"]; ok {
		t.Fatal("record still present")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("file still present: %v", err)
	}
}

func TestChannelPackageHiddenProjectRejected(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3, IsHidden: true}); err != nil {
		t.Fatal(err)
	}
	h := NewChannelPackageHandler(nil, projects)
	e := echo.New()
	routeWithSession(e, http.MethodGet, "/cp/api/v1/projects/:id/channel-packages", h.List, sessionForTest(3))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/projects/p1/channel-packages", nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "40401") {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChannelPackageErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		status int
		code   int
	}{
		{name: "invalid source", err: channelpkg.ErrInvalidSource, status: http.StatusBadRequest, code: 40001},
		{name: "unsupported", err: channelpkg.ErrUnsupported, status: http.StatusConflict, code: 40901},
		{name: "busy", err: channelpkg.ErrBusy, status: http.StatusTooManyRequests, code: 42911},
		{name: "too large", err: channelpkg.ErrPackageTooLarge, status: http.StatusRequestEntityTooLarge, code: 41301},
		{name: "unsafe archive", err: fmt.Errorf("write: %w", core.ErrUnsafeArchive), status: http.StatusRequestEntityTooLarge, code: 41301},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := echo.New()
			rec := httptest.NewRecorder()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), rec)
			if err := channelPackageError(c, tc.err); err != nil {
				t.Fatal(err)
			}
			if rec.Code != tc.status {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.status, rec.Body.String())
			}
			var env struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
				t.Fatal(err)
			}
			if env.Code != tc.code {
				t.Fatalf("code = %d, want %d", env.Code, tc.code)
			}
		})
	}
}

func writeChannelPackageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

package channelpkg

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/dao"
	daomodel "vibebiz/internal/dao/model"
	"vibebiz/internal/model"
)

type fakeRepo struct {
	mu         sync.Mutex
	items      map[string]*daomodel.ProjectChannelPackage
	updates    int
	lastUpdate *daomodel.ProjectChannelPackage
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: map[string]*daomodel.ProjectChannelPackage{}}
}

func clonePackage(item *daomodel.ProjectChannelPackage) *daomodel.ProjectChannelPackage {
	if item == nil {
		return nil
	}
	clone := *item
	if item.SourceResourceID != nil {
		resourceID := *item.SourceResourceID
		clone.SourceResourceID = &resourceID
	}
	clone.Manifest = append(daomodel.JSON(nil), item.Manifest...)
	return &clone
}

func (f *fakeRepo) Create(_ context.Context, item *daomodel.ProjectChannelPackage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[item.PublicID] = clonePackage(item)
	return nil
}

func (f *fakeRepo) Update(_ context.Context, item *daomodel.ProjectChannelPackage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.items[item.PublicID] = clonePackage(item)
	f.updates++
	f.lastUpdate = clonePackage(item)
	return nil
}

func (f *fakeRepo) GetByPublicID(_ context.Context, id string) (*daomodel.ProjectChannelPackage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.items[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return item, nil
}

func (f *fakeRepo) ListByProject(_ context.Context, projectID string) ([]daomodel.ProjectChannelPackage, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []daomodel.ProjectChannelPackage
	for _, item := range f.items {
		if item.ProjectPublicID == projectID {
			out = append(out, *item)
		}
	}
	return out, nil
}

func (f *fakeRepo) Delete(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.items[id]; !ok {
		return dao.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeRepo) get(t *testing.T, id string) *daomodel.ProjectChannelPackage {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	item, ok := f.items[id]
	if !ok {
		t.Fatalf("record %q not found", id)
	}
	return clonePackage(item)
}

func (f *fakeRepo) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.items)
}

func (f *fakeRepo) updateState() (int, *daomodel.ProjectChannelPackage) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updates, clonePackage(f.lastUpdate)
}

type fakeAdapter struct {
	level    string
	blockers []string
	build    func(ctx context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error)
}

func (f fakeAdapter) Channel() string { return core.ChannelWeChat }

func (f fakeAdapter) Validate(core.SourceInfo) core.Report {
	return core.Report{Level: f.level, Blockers: append([]string{}, f.blockers...)}
}

func (f fakeAdapter) Build(ctx context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error) {
	if f.build != nil {
		return f.build(ctx, src, out)
	}
	return writeFakePackage(out, src)
}

func writeFakePackage(out io.Writer, src core.SourceInfo) (core.Manifest, error) {
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
		ProjectID:     src.ProjectID,
		Channel:       core.ChannelWeChat,
		SourceType:    src.SourceType,
		SourceVersion: src.Version,
		Level:         src.Report.Level,
	}, nil
}

func newTestService(repo Repository, root string, adapter core.Adapter, opts ...func(*ServiceConfig)) *Service {
	cfg := ServiceConfig{
		Repo:          repo,
		WorkspacesDir: root,
		PackagesDir:   filepath.Join(root, "packages"),
		Adapters:      map[string]core.Adapter{core.ChannelWeChat: adapter},
		Now:           func() time.Time { return time.Unix(1700000000, 0).UTC() },
		NewID:         func() string { return "pkg-1" },
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewService(cfg)
}

func TestGenerateWritesPackageAndRecord(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c').getContext('2d')`)
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB})
	item, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5})
	if err != nil {
		t.Fatal(err)
	}
	if item.Status != daomodel.ChannelPackageStatusReady || item.FileSizeBytes == 0 {
		t.Fatalf("item = %+v", item)
	}
	if item.Version == "" {
		t.Fatalf("version is empty: %+v", item)
	}
	wantPath := filepath.ToSlash(filepath.Join("p1", core.ChannelWeChat, "pkg-1.zip"))
	if item.FilePath != wantPath {
		t.Fatalf("file path = %q, want %q", item.FilePath, wantPath)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", filepath.FromSlash(item.FilePath))); err != nil {
		t.Fatal(err)
	}
	if len(item.Manifest) == 0 {
		t.Fatal("manifest was not stored")
	}
	var manifest core.Manifest
	if err := json.Unmarshal(item.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.SourceVersion != item.Version || manifest.Channel != core.ChannelWeChat {
		t.Fatalf("manifest = %+v", manifest)
	}
}

func TestGenerateRejectsLevelC(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<div id="app"></div>`)
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelC})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed || item.ErrorMessage == "" {
		t.Fatalf("status = %s error = %q", item.Status, item.ErrorMessage)
	}
}

func TestGenerateUsesAdapterReportForCompatibility(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<div id="app"></div><script src="app.js"></script>`)
	write(t, dir, "app.js", `document.querySelector('#app').innerHTML = '<b>x</b>'; document.querySelectorAll('div');`)
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB})
	item, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5})
	if err != nil {
		t.Fatalf("err = %v, want adapter report to allow source", err)
	}
	if item.Status != daomodel.ChannelPackageStatusReady {
		t.Fatalf("status = %s", item.Status)
	}
}

func TestGenerateRejectsAdapterBlockers(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB, blockers: []string{"dom_dependency"}})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed || item.ErrorMessage == "" {
		t.Fatalf("item = %+v", item)
	}
}

func TestGenerateRejectsOversizedHTML5Source(t *testing.T) {
	const limit int64 = 200 << 20
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas>`)
	f, err := os.Create(filepath.Join(dir, "large.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(limit + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrPackageTooLarge) {
		t.Fatalf("err = %v, want ErrPackageTooLarge", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed {
		t.Fatalf("status = %s", item.Status)
	}
}

func TestGenerateRejectsHTML5SourceWithTooManyFiles(t *testing.T) {
	const limit = 20000
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas>`)
	for i := 0; i < limit; i++ {
		name := filepath.Join(dir, fmt.Sprintf("asset-%05d.bin", i))
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrPackageTooLarge) {
		t.Fatalf("err = %v, want ErrPackageTooLarge", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed {
		t.Fatalf("status = %s", item.Status)
	}
}

func TestGenerateRejectsUnknownChannelAndSourceType(t *testing.T) {
	repo := newFakeRepo()
	svc := newTestService(repo, t.TempDir(), fakeAdapter{level: core.LevelB})
	project := &model.Project{ID: "p1", WorkspaceID: 3}
	if _, err := svc.Generate(context.Background(), project, GenerateInput{Channel: "unknown", SourceType: core.SourceHTML5}); !errors.Is(err, ErrInvalidChannel) {
		t.Fatalf("err = %v, want ErrInvalidChannel", err)
	}
	if _, err := svc.Generate(context.Background(), project, GenerateInput{Channel: core.ChannelWeChat, SourceType: "zip"}); !errors.Is(err, ErrInvalidSourceType) {
		t.Fatalf("err = %v, want ErrInvalidSourceType", err)
	}
	if repo.count() != 0 {
		t.Fatalf("invalid inputs must not create records")
	}
}

func TestGenerateReturnsBusyWhenConcurrencyFull(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	adapter := fakeAdapter{level: core.LevelB, build: func(ctx context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error) {
		once.Do(func() { close(started) })
		select {
		case <-release:
		case <-ctx.Done():
			return core.Manifest{}, ctx.Err()
		}
		return writeFakePackage(out, src)
	}}
	repo := newFakeRepo()
	svc := newTestService(repo, root, adapter, func(cfg *ServiceConfig) {
		cfg.Concurrency = 1
		cfg.Timeout = 2 * time.Second
	})
	first := make(chan error, 1)
	go func() {
		_, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5})
		first <- err
	}()
	<-started
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrBusy) {
		t.Fatalf("err = %v, want ErrBusy", err)
	}
	close(release)
	if err := <-first; err != nil {
		t.Fatal(err)
	}
}

func TestGenerateAppliesTimeout(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	adapter := fakeAdapter{level: core.LevelB, build: func(ctx context.Context, _ core.SourceInfo, _ io.Writer) (core.Manifest, error) {
		<-ctx.Done()
		return core.Manifest{}, ctx.Err()
	}}
	repo := newFakeRepo()
	svc := newTestService(repo, root, adapter, func(cfg *ServiceConfig) {
		cfg.Timeout = 30 * time.Millisecond
	})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed {
		t.Fatalf("status = %s", item.Status)
	}
}

func TestGenerateRejectsOversizedPackageAndCleansTemp(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	adapter := fakeAdapter{level: core.LevelB, build: func(_ context.Context, _ core.SourceInfo, out io.Writer) (core.Manifest, error) {
		if _, err := out.Write(bytes.Repeat([]byte("x"), 1024)); err != nil {
			return core.Manifest{}, err
		}
		return core.Manifest{}, nil
	}}
	repo := newFakeRepo()
	svc := newTestService(repo, root, adapter, func(cfg *ServiceConfig) { cfg.MaxBytes = 16 })
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); !errors.Is(err, ErrPackageTooLarge) {
		t.Fatalf("err = %v, want ErrPackageTooLarge", err)
	}
	item := repo.get(t, "pkg-1")
	if item.Status != daomodel.ChannelPackageStatusFailed || item.FilePath != "" {
		t.Fatalf("item = %+v", item)
	}
	if _, err := os.Stat(filepath.Join(root, "packages", "p1", core.ChannelWeChat, "pkg-1.zip")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("final file must not exist: %v", err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "packages"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tmp-") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}
}

func TestGenerateEngineArtifactCleansSourceTempDir(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("index.html")
	_, _ = w.Write([]byte("<canvas></canvas>"))
	_ = zw.Close()
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
	}
	root := t.TempDir()
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB}, func(cfg *ServiceConfig) {
		cfg.OpenResource = opener
	})
	item, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{
		Channel: core.ChannelWeChat, SourceType: core.SourceEngineArtifact, ResourceID: "r1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.SourceType != core.SourceEngineArtifact || item.SourceResourceID == nil || *item.SourceResourceID != "r1" {
		t.Fatalf("item = %+v", item)
	}
	entries, err := os.ReadDir(filepath.Join(root, "packages"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".channel-src-") {
			t.Fatalf("source temp dir left behind: %s", entry.Name())
		}
	}
}

func TestGenerateUpdatePreservesFullRecord(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	repo := newFakeRepo()
	now := time.Unix(1700000000, 0).UTC()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB}, func(cfg *ServiceConfig) {
		cfg.Now = func() time.Time { return now }
	})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); err != nil {
		t.Fatal(err)
	}
	updates, item := repo.updateState()
	if updates != 1 {
		t.Fatalf("update calls = %d, want 1", updates)
	}
	if item == nil {
		t.Fatal("update received no record")
	}
	if item.Status != daomodel.ChannelPackageStatusReady || item.WorkspaceID != 3 || item.ProjectPublicID != "p1" || item.SourceType != core.SourceHTML5 || item.Channel != core.ChannelWeChat {
		t.Fatalf("record lost fields on update: %+v", item)
	}
	if item.FilePath == "" || item.FileSizeBytes == 0 || len(item.Manifest) == 0 {
		t.Fatalf("update did not carry generated fields: %+v", item)
	}
	if !item.CreatedAt.Equal(now) {
		t.Fatalf("createdAt = %v, want %v", item.CreatedAt, now)
	}
}

func TestDeleteRejectsPathEscape(t *testing.T) {
	root := t.TempDir()
	packages := filepath.Join(root, "packages")
	if err := os.MkdirAll(packages, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "escape.zip")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.items["pkg-1"] = &daomodel.ProjectChannelPackage{PublicID: "pkg-1", ProjectPublicID: "p1", FilePath: "../escape.zip"}
	svc := NewService(ServiceConfig{Repo: repo, PackagesDir: packages})
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("file outside packages dir was removed: %v", err)
	}
	if repo.count() != 1 {
		t.Fatal("record must not be deleted")
	}
}

func TestDeleteRejectsAbsolutePath(t *testing.T) {
	root := t.TempDir()
	packages := filepath.Join(root, "packages")
	if err := os.MkdirAll(packages, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "escape-abs.zip")
	if err := os.WriteFile(outside, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.items["pkg-1"] = &daomodel.ProjectChannelPackage{PublicID: "pkg-1", ProjectPublicID: "p1", FilePath: outside}
	svc := NewService(ServiceConfig{Repo: repo, PackagesDir: packages})
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("absolute file was removed: %v", err)
	}
	if repo.count() != 1 {
		t.Fatal("record must not be deleted")
	}
}

func TestDeleteRejectsForeignProject(t *testing.T) {
	root := t.TempDir()
	packages := filepath.Join(root, "packages")
	file := filepath.Join(packages, "p2", core.ChannelWeChat, "pkg-1.zip")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.items["pkg-1"] = &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p2",
		FilePath: filepath.ToSlash(filepath.Join("p2", core.ChannelWeChat, "pkg-1.zip")),
	}
	svc := NewService(ServiceConfig{Repo: repo, PackagesDir: packages})
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); !errors.Is(err, dao.ErrNotFound) {
		t.Fatalf("err = %v, want dao.ErrNotFound", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("foreign package file was removed: %v", err)
	}
	if repo.count() != 1 {
		t.Fatal("foreign record must not be deleted")
	}
}

func TestDeleteRejectsAnotherProjectsFile(t *testing.T) {
	root := t.TempDir()
	packages := filepath.Join(root, "packages")
	file := filepath.Join(packages, "p2", core.ChannelWeChat, "pkg-1.zip")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.items["pkg-1"] = &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1",
		FilePath: filepath.ToSlash(filepath.Join("p2", core.ChannelWeChat, "pkg-1.zip")),
	}
	svc := NewService(ServiceConfig{Repo: repo, PackagesDir: packages})
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("another project's file was removed: %v", err)
	}
	if repo.count() != 1 {
		t.Fatal("record must not be deleted")
	}
}

func TestDeleteSucceedsWhenFileAlreadyMissing(t *testing.T) {
	root := t.TempDir()
	packages := filepath.Join(root, "packages")
	if err := os.MkdirAll(filepath.Join(packages, "p1", core.ChannelWeChat), 0o755); err != nil {
		t.Fatal(err)
	}
	repo := newFakeRepo()
	repo.items["pkg-1"] = &daomodel.ProjectChannelPackage{
		PublicID: "pkg-1", ProjectPublicID: "p1",
		FilePath: filepath.ToSlash(filepath.Join("p1", core.ChannelWeChat, "missing.zip")),
	}
	svc := NewService(ServiceConfig{Repo: repo, PackagesDir: packages})
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); err != nil {
		t.Fatal(err)
	}
	if repo.count() != 0 {
		t.Fatal("record was not deleted")
	}
}

func TestDeleteRemovesFileAndRecord(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "p1"), "index.html", `<canvas></canvas>`)
	repo := newFakeRepo()
	svc := newTestService(repo, root, fakeAdapter{level: core.LevelB})
	item, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5})
	if err != nil {
		t.Fatal(err)
	}
	full := filepath.Join(svc.PackagesDir(), filepath.FromSlash(item.FilePath))
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(full); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("package file still exists: %v", err)
	}
	if repo.count() != 0 {
		t.Fatal("record was not deleted")
	}
	if err := svc.Delete(context.Background(), "p1", "pkg-1"); !errors.Is(err, dao.ErrNotFound) {
		t.Fatalf("second delete err = %v, want dao.ErrNotFound", err)
	}
}

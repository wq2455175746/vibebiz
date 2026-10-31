package publish

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPublishCopiesWorkspaceAndGeneratesSharePage(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	projectID := "p1"
	source := filepath.Join(workspaces, projectID)
	if err := os.MkdirAll(filepath.Join(source, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(path, content string) {
		if err := os.WriteFile(filepath.Join(source, path), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("index.html", "<h1>Game</h1>")
	write("assets/game.js", "console.log('game')")
	write("docs/secret.md", "do not publish")
	write(".git/config", "secret git")
	if err := os.WriteFile(filepath.Join(source, "clip.mp4"), []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}

	service := NewService(workspaces, published)
	result, err := service.Publish(context.Background(), projectID, Input{
		Title:       "毕业季小游戏",
		Description: "三分钟通关的校园答题游戏",
		Tags:        []string{"答题", "校园"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Slug != projectID {
		t.Fatalf("slug = %q", result.Slug)
	}
	wrapper, err := os.ReadFile(filepath.Join(published, projectID, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(wrapper)
	for _, want := range []string{"毕业季小游戏", "三分钟通关的校园答题游戏", "答题", "game/index.html"} {
		if !strings.Contains(text, want) {
			t.Fatalf("share page missing %q", want)
		}
	}
	if strings.Contains(text, "do not publish") {
		t.Fatal("docs leaked into share page")
	}
	if _, err := os.Stat(filepath.Join(published, projectID, "game", "assets", "game.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(published, projectID, "game", "docs")); !os.IsNotExist(err) {
		t.Fatal("docs directory should not be published")
	}
	video, err := os.ReadFile(filepath.Join(published, projectID, "game", "clip.mp4"))
	if err != nil || string(video) != string([]byte{0, 1, 2, 3}) {
		t.Fatalf("binary asset not copied: %v %v", video, err)
	}
}

func TestPublishFallsBackToAppHTMLAndRejectsMissingEntry(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	projectID := "p2"
	source := filepath.Join(workspaces, projectID)
	if err := os.MkdirAll(source, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "app.html"), []byte("app"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(workspaces, published)
	if _, err := service.Publish(context.Background(), projectID, Input{Title: "App"}); err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile(filepath.Join(published, projectID, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(wrapper), "game/app.html") {
		t.Fatalf("app.html fallback missing: %s", wrapper)
	}
	if _, err := service.Publish(context.Background(), "missing", Input{Title: "x"}); err == nil {
		t.Fatal("expected missing entry error")
	}
}

func TestFilePathStaysInsidePublishDirectory(t *testing.T) {
	root := t.TempDir()
	published := filepath.Join(root, "published")
	service := NewService(filepath.Join(root, "workspaces"), published)
	target := filepath.Join(published, "p1")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "index.html"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := service.FilePath("p1", "")
	if err != nil || filepath.Base(path) != "index.html" {
		t.Fatalf("path = %q err = %v", path, err)
	}
	if _, err := service.FilePath("p1", "../../etc/passwd"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
	if _, err := service.FilePath("../p1", "index.html"); err == nil {
		t.Fatal("expected invalid slug to be rejected")
	}
}

func TestListPublishedReadsMetadataAndSortsNewestFirst(t *testing.T) {
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
	service := NewService(workspaces, published)
	service.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if _, err := service.Publish(context.Background(), "p1", Input{Title: "旧作品", Tags: []string{"答题"}}); err != nil {
		t.Fatal(err)
	}
	service.Now = func() time.Time { return time.Unix(1700003600, 0).UTC() }
	if _, err := service.Publish(context.Background(), "p2", Input{Title: "新作品"}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(published, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(published, "broken", "publish.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	items, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[0].ProjectID != "p2" || items[0].Title != "新作品" {
		t.Fatalf("first = %+v", items[0])
	}
	if items[1].ProjectID != "p1" || items[1].Title != "旧作品" || len(items[1].Tags) != 1 {
		t.Fatalf("second = %+v", items[1])
	}
}

func TestRepublishUpdatesMetadataAndKeepsSameSlug(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	projectID := "p1"
	if err := os.MkdirAll(filepath.Join(workspaces, projectID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaces, projectID, "index.html"), []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(workspaces, published)
	service.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	if _, err := service.Publish(context.Background(), projectID, Input{Title: "旧标题"}); err != nil {
		t.Fatal(err)
	}
	service.Now = func() time.Time { return time.Unix(1700003600, 0).UTC() }
	result, err := service.Publish(context.Background(), projectID, Input{Title: "新标题"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Slug != projectID {
		t.Fatalf("slug = %q", result.Slug)
	}
	items, err := service.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Title != "新标题" || !items[0].PublishedAt.Equal(time.Unix(1700003600, 0).UTC()) {
		t.Fatalf("items = %+v", items)
	}
}

func TestRefreshSharePagesRewritesExistingIndexes(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	projectID := "p1"
	if err := os.MkdirAll(filepath.Join(workspaces, projectID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaces, projectID, "index.html"), []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(workspaces, published)
	if _, err := service.Publish(context.Background(), projectID, Input{Title: "刷新后的标题"}); err != nil {
		t.Fatal(err)
	}
	sharePath := filepath.Join(published, projectID, "index.html")
	if err := os.WriteFile(sharePath, []byte("old share page"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := service.RefreshSharePages(context.Background()); err != nil {
		t.Fatal(err)
	}
	text, err := os.ReadFile(sharePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "刷新后的标题") || strings.Contains(string(text), "old share page") {
		t.Fatalf("share page not refreshed: %s", text)
	}
}

func TestDeletePublishedRemovesOnlyPublishedCopy(t *testing.T) {
	root := t.TempDir()
	workspaces := filepath.Join(root, "workspaces")
	published := filepath.Join(root, "published")
	projectID := "p1"
	if err := os.MkdirAll(filepath.Join(workspaces, projectID), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workspaces, projectID, "index.html"), []byte("game"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := NewService(workspaces, published)
	if _, err := service.Publish(context.Background(), projectID, Input{Title: "作品"}); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(context.Background(), projectID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(published, projectID)); !os.IsNotExist(err) {
		t.Fatal("published copy should be removed")
	}
	if _, err := os.Stat(filepath.Join(workspaces, projectID, "index.html")); err != nil {
		t.Fatal("workspace source should remain")
	}
	if err := service.Delete(context.Background(), projectID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete err = %v", err)
	}
}

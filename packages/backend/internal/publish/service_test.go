package publish

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

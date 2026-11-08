package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skipSummaryTestPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return len(parts) > 0 && (parts[0] == "docs" || parts[0] == "node_modules")
}

func TestSkipPathOnlySkipsTopLevelDocsAndNodeModules(t *testing.T) {
	cases := map[string]bool{
		".git/config":              true,
		".leak.js":                 true,
		"docs/a.md":                true,
		"node_modules/x.js":        true,
		"src/.cache/x":             true,
		"assets/docs/a.md":         false,
		"assets/node_modules/x.js": false,
		"keep.js":                  false,
	}
	for rel, want := range cases {
		if got := SkipPath(rel); got != want {
			t.Errorf("SkipPath(%q) = %v, want %v", rel, got, want)
		}
	}
}

func TestSummarizeFilesRejectsSkippedSymlink(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "secret.js")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "source")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".leak.js")); err != nil {
		t.Skip("symlink unsupported")
	}
	_, err := SummarizeFiles(context.Background(), dir, SkipPath)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("err = %v, want ErrUnsafePath", err)
	}
}

func TestSummarizeFilesSkipsPathsAndReturnsHashes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "keep.js", "hello")
	write(t, dir, ".git/config", "ignored")
	write(t, dir, ".chathistory/session.jsonl", "ignored")
	write(t, dir, "docs/a.md", "ignored")
	write(t, dir, "node_modules/x.js", "ignored")

	files, err := SummarizeFiles(context.Background(), dir, skipSummaryTestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %+v, want only keep.js", files)
	}
	if files[0].Path != "keep.js" || files[0].Size != 5 || len(files[0].SHA256) != 12 {
		t.Fatalf("summary = %+v", files[0])
	}
}

func TestSummarizeFilesCapsAt500(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 505; i++ {
		write(t, dir, filepath.ToSlash(filepath.Join("bulk", fmt.Sprintf("%03d.js", i))), "x")
	}
	files, err := SummarizeFiles(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 500 {
		t.Fatalf("files = %d, want 500", len(files))
	}
}

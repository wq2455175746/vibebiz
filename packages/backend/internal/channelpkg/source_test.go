package channelpkg

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/resource"
)

func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceSourceUsesIndexEntryAndVersion(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c').getContext('2d')`)
	src, err := NewWorkspaceSource(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "index.html" || info.Version == "" || info.FileCount != 2 {
		t.Fatalf("info = %+v", info)
	}
}

func TestWorkspaceSourceRejectsMissingProject(t *testing.T) {
	src, err := NewWorkspaceSource(t.TempDir(), "missing")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := src.Materialize(context.Background()); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("err = %v, want ErrSourceNotFound", err)
	}
}

func TestWorkspaceSourceAcceptsMiniGameWithoutIndex(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "game.js", "console.log('game')")
	write(t, dir, "game.json", `{"deviceOrientation":"portrait"}`)
	src, err := NewWorkspaceSource(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "" || info.Report.Level != core.LevelA || info.FileCount != 2 {
		t.Fatalf("info = %+v", info)
	}
}

func TestWorkspaceSourceRejectsInvalidProjectID(t *testing.T) {
	for _, projectID := range []string{"", "../p1", "p/1", "p 1", strings.Repeat("a", 65)} {
		if _, err := NewWorkspaceSource(t.TempDir(), projectID); !errors.Is(err, ErrInvalidSource) {
			t.Errorf("projectID %q: err = %v, want ErrInvalidSource", projectID, err)
		}
	}
}

func TestWorkspaceSourceVersionIsStableAndTracksContent(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas>`)
	write(t, dir, "game.js", "let score = 0")
	src, err := NewWorkspaceSource(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	first, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Version != second.Version {
		t.Fatalf("version changed between reads: %q != %q", first.Version, second.Version)
	}
	write(t, dir, "game.js", "let score = 1")
	changed, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if changed.Version == first.Version {
		t.Fatal("version did not change after content changed")
	}
}

func TestWorkspaceSourceExcludesDocsNodeModulesAndDotDirs(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas>`)
	write(t, dir, "docs/readme.md", "docs")
	write(t, dir, "node_modules/pkg/index.js", "module")
	write(t, dir, ".git/config", "git")
	write(t, dir, "nested/.cache/data", "cache")
	src, err := NewWorkspaceSource(root, "p1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.FileCount != 1 || info.TotalBytes != int64(len(`<canvas></canvas>`)) {
		t.Fatalf("info = %+v", info)
	}
}

func TestResourceSourceExtractsZipAndComputesVersion(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("index.html")
	_, _ = w.Write([]byte("<canvas></canvas>"))
	_ = zw.Close()
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
	}
	src := NewResourceSource("p1", "r1", opener, t.TempDir())
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "index.html" || info.Version == "" {
		t.Fatalf("info = %+v", info)
	}
	sum := sha256.Sum256(buf.Bytes())
	if want := hex.EncodeToString(sum[:])[:16]; info.Version != want {
		t.Fatalf("version = %q, want %q", info.Version, want)
	}
}

func TestResourceSourceAcceptsMiniGameZipWithoutIndex(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range map[string]string{"game.js": "console.log('game')", "game.json": `{}`} {
		w, _ := zw.Create(name)
		_, _ = w.Write([]byte(content))
	}
	_ = zw.Close()
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		return io.NopCloser(bytes.NewReader(buf.Bytes())), int64(buf.Len()), nil
	}
	src := NewResourceSource("p1", "r1", opener, t.TempDir())
	info, err := src.Materialize(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Entry != "" || info.Report.Level != core.LevelA || info.FileCount != 2 {
		t.Fatalf("info = %+v", info)
	}
}

func TestResourceSourceMapsResourceNotFound(t *testing.T) {
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		return nil, 0, fmt.Errorf("open: %w", resource.ErrNotFound)
	}
	src := NewResourceSource("p1", "r1", opener, t.TempDir())
	if _, err := src.Materialize(context.Background()); !errors.Is(err, ErrSourceNotFound) {
		t.Fatalf("err = %v, want ErrSourceNotFound", err)
	}
}

func TestResourceSourceRejectsInvalidIDs(t *testing.T) {
	opener := func(context.Context, string, string) (io.ReadCloser, int64, error) {
		t.Fatal("opener must not be called for invalid ids")
		return nil, 0, nil
	}
	for _, tc := range []struct{ projectID, resourceID string }{
		{"../p1", "r1"},
		{"p1", "r/1"},
	} {
		src := NewResourceSource(tc.projectID, tc.resourceID, opener, t.TempDir())
		if _, err := src.Materialize(context.Background()); !errors.Is(err, ErrInvalidSource) {
			t.Errorf("ids %q/%q: err = %v, want ErrInvalidSource", tc.projectID, tc.resourceID, err)
		}
	}
}

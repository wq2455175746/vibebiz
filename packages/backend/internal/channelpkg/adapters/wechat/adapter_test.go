package wechat

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibebiz/internal/channelpkg/core"
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

func zipNames(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		names[f.Name] = true
	}
	return names
}

func zipContent(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		content, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(content)
	}
	t.Fatalf("missing %s in zip", name)
	return ""
}

func TestWeChatBuildWritesMiniGameFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('game').getContext('2d')`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelWeChat {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{"game.js", "game.json", "project.config.json", "adapter/index.js", "package-manifest.json", "CHANNEL_GUIDE.md"} {
		if !names[want] {
			t.Errorf("missing %s in %v", want, names)
		}
	}
}

func TestWeChatBuildRejectsDomGame(t *testing.T) {
	adapter := New()
	report := adapter.Validate(core.SourceInfo{Report: core.Report{Level: core.LevelC}})
	if len(report.Blockers) == 0 {
		t.Fatal("expected blocker for level C")
	}
}

func TestWeChatBuildRejectsExternalScript(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="https://cdn.example.com/game.js"></script>`)
	var buf bytes.Buffer
	_, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf)
	if !errors.Is(err, ErrExternalScript) {
		t.Fatalf("err = %v, want ErrExternalScript", err)
	}
}

func TestWeChatBuildBundlesScriptsInDocumentOrder(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas>`+
		`<script>window.order=['inline-first']</script>`+
		`<script src="js/game.js"></script>`+
		`<script>window.order.push('inline-last')</script>`)
	write(t, dir, "js/game.js", `window.order.push('relative-file')`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{"game/index.html", "game/js/game.js", "game/game.bundle.js"} {
		if !names[want] {
			t.Errorf("missing %s in %v", want, names)
		}
	}
	bundle := zipContent(t, buf.Bytes(), "game/game.bundle.js")
	first := strings.Index(bundle, "inline-first")
	relative := strings.Index(bundle, "relative-file")
	last := strings.Index(bundle, "inline-last")
	if first < 0 || relative < 0 || last < 0 {
		t.Fatalf("bundle missing scripts: %q", bundle)
	}
	if !(first < relative && relative < last) {
		t.Fatalf("bundle order = %d,%d,%d: %q", first, relative, last, bundle)
	}
}

func TestWeChatBuildCopiesLevelASourceWithoutAdapter(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.js", `console.log('native-minigame')`)
	write(t, dir, "game.json", `{"deviceOrientation":"landscape"}`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v1", Report: core.Report{Level: core.LevelA},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, buf.Bytes())
	if names["adapter/index.js"] || names["game/game.bundle.js"] {
		t.Fatalf("level A source should not inject adapter: %v", names)
	}
	if !names["project.config.json"] || !names["package-manifest.json"] || !names["CHANNEL_GUIDE.md"] {
		t.Fatalf("missing package metadata in %v", names)
	}
	if got := zipContent(t, buf.Bytes(), "game.js"); got != `console.log('native-minigame')` {
		t.Fatalf("game.js = %q", got)
	}
	if got := zipContent(t, buf.Bytes(), "game.json"); got != `{"deviceOrientation":"landscape"}` {
		t.Fatalf("game.json = %q", got)
	}
}

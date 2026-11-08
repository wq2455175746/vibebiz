package douyin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
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

func TestDouyinBuildWritesTTProject(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('game').getContext('2d')`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelDouyin {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{"game.js", "game.json", "project.config.json", "adapter/index.js", "package-manifest.json", "CHANNEL_GUIDE.md"} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestDouyinBuildRejectsDomGame(t *testing.T) {
	adapter := New()
	report := adapter.Validate(core.SourceInfo{Report: core.Report{Level: core.LevelC}})
	found := false
	for _, blocker := range report.Blockers {
		if blocker == "dom_dependency" {
			found = true
		}
	}
	if !found {
		t.Fatalf("blockers = %v, want dom_dependency", report.Blockers)
	}
}

func TestDouyinBuildRejectsExternalScript(t *testing.T) {
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

func TestDouyinBuildRejectsUnsafeScriptURLs(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{"protocol relative", "//cdn.example.com/game.js"},
		{"absolute", "/static/game.js"},
		{"parent traversal", "../escape.js"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "index.html", `<canvas></canvas><script src="`+tc.src+`"></script>`)
			var buf bytes.Buffer
			_, err := New().Build(context.Background(), core.SourceInfo{
				Dir: dir, Entry: "index.html", Version: "v1",
				Report: core.Report{Level: core.LevelB},
			}, &buf)
			if !errors.Is(err, ErrUnsafeScript) {
				t.Fatalf("err = %v, want ErrUnsafeScript", err)
			}
		})
	}
}

func TestDouyinBuildBundlesScriptsInDocumentOrder(t *testing.T) {
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

func TestDouyinBuildCopiesLevelASourceWithoutAdapter(t *testing.T) {
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

func TestDouyinBuildSkipsWorkspaceArtifacts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	write(t, dir, ".chathistory/session.jsonl", `{"message":"secret"}`)
	write(t, dir, ".git/config", "[remote]")
	write(t, dir, "node_modules/x.js", "module.exports = 1")
	write(t, dir, "docs/a.md", "# docs")
	write(t, dir, "assets/docs/keep.js", "keep nested docs")
	write(t, dir, "package-manifest.json", `{"source":true}`)
	write(t, dir, "CHANNEL_GUIDE.md", "source guide")
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, buf.Bytes())
	for _, bad := range []string{
		"game/.chathistory/session.jsonl", "game/.git/config",
		"game/node_modules/x.js", "game/docs/a.md",
		"game/package-manifest.json", "game/CHANNEL_GUIDE.md",
	} {
		if names[bad] {
			t.Errorf("workspace artifact %s was copied", bad)
		}
	}
	if !names["game/game.js"] || !names["game/assets/docs/keep.js"] || !names["package-manifest.json"] || !names["CHANNEL_GUIDE.md"] {
		t.Fatalf("expected generated files in %v", names)
	}
}

func TestDouyinResolveScriptRejectsSymlinkOutsideSource(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "source")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "secret.js")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".leak.js")); err != nil {
		t.Skip("symlink unsupported")
	}
	if _, err := resolveScript(root, root, ".leak.js"); !errors.Is(err, ErrUnsafeScript) {
		t.Fatalf("err = %v, want ErrUnsafeScript", err)
	}
}

func TestDouyinBuildRejectsIncompleteLevelASource(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.js", `console.log('missing game.json')`)
	var buf bytes.Buffer
	_, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v1", Report: core.Report{Level: core.LevelA},
	}, &buf)
	if !errors.Is(err, ErrIncompleteSource) {
		t.Fatalf("err = %v, want ErrIncompleteSource", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("partial zip written: %d bytes", buf.Len())
	}
}

func TestDouyinBuildManifestIncludesReportAndFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	var buf bytes.Buffer
	manifest, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v9",
		ProjectID: "p1", SourceType: core.SourceHTML5,
		Report: core.Report{Level: core.LevelB, Warnings: []string{"network_fetch"}},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ProjectID != "p1" || manifest.Channel != core.ChannelDouyin ||
		manifest.SourceType != core.SourceHTML5 || manifest.SourceVersion != "v9" {
		t.Fatalf("manifest = %+v", manifest)
	}
	if manifest.Report.Level != core.LevelB || len(manifest.Report.Warnings) != 1 {
		t.Fatalf("report = %+v", manifest.Report)
	}
	if len(manifest.Files) == 0 {
		t.Fatal("manifest files are empty")
	}
	found := false
	for _, file := range manifest.Files {
		if file.Path == "game/game.js" {
			found = true
			if file.Size == 0 || len(file.SHA256) != 12 {
				t.Fatalf("file summary = %+v", file)
			}
		}
	}
	if !found {
		t.Fatalf("game/game.js missing from %+v", manifest.Files)
	}

	var decoded core.Manifest
	if err := json.Unmarshal([]byte(zipContent(t, buf.Bytes(), "package-manifest.json")), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProjectID != "p1" || decoded.Channel != core.ChannelDouyin ||
		decoded.SourceVersion != "v9" || decoded.Report.Level != core.LevelB ||
		len(decoded.Files) == 0 {
		t.Fatalf("decoded manifest = %+v", decoded)
	}
}

func TestDouyinProjectConfigUsesTTFields(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(zipContent(t, buf.Bytes(), "project.config.json")), &config); err != nil {
		t.Fatal(err)
	}
	if config["appid"] != "testappid" {
		t.Fatalf("appid = %v, want testappid", config["appid"])
	}
	if config["compileType"] != "game" {
		t.Fatalf("compileType = %v, want game", config["compileType"])
	}
	if name, ok := config["projectname"].(string); !ok || name == "" {
		t.Fatalf("projectname = %v", config["projectname"])
	}
	if _, ok := config["setting"].(map[string]any); !ok {
		t.Fatalf("setting = %v", config["setting"])
	}
}

func TestDouyinAdapterPrefersTT(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	adapter := zipContent(t, buf.Bytes(), "adapter/index.js")
	if !strings.Contains(adapter, "typeof tt !== 'undefined' ? tt : wx") {
		t.Fatalf("adapter does not prefer tt: %q", adapter)
	}
	game := zipContent(t, buf.Bytes(), "game.js")
	adapterAt := strings.Index(game, "adapter/index.js")
	bundleAt := strings.Index(game, "game/game.bundle.js")
	if adapterAt < 0 || bundleAt < 0 || adapterAt > bundleAt {
		t.Fatalf("game.js does not load adapter before bundle: %q", game)
	}
}

func TestDouyinBuildWritesChannelGuide(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	want := guides.Markdown(core.ChannelDouyin)
	if strings.TrimSpace(want) == "" {
		t.Fatal("douyin guide is empty")
	}
	if got := zipContent(t, buf.Bytes(), "CHANNEL_GUIDE.md"); got != want {
		t.Fatalf("guide = %q, want douyin guide", got)
	}
}

package steam

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

func TestSteamBuildWritesElectronShell(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas></canvas>`)
	var buf bytes.Buffer
	adapter := New()
	manifest, err := adapter.Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Channel != core.ChannelSteam {
		t.Fatalf("channel = %s", manifest.Channel)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{
		"app/index.html", "electron/main.js", "electron/preload.js", "package.json",
		"electron-builder.yml", "build-steam.sh", "build-steam.ps1",
		"steam/app_build.vdf.example", "CHANNEL_GUIDE.md", "package-manifest.json",
	} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
}

func TestSteamBuildDetectsNativeExe(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.exe", "MZ")
	adapter := New()
	report := adapter.Validate(core.SourceInfo{Dir: dir, FileCount: 1})
	if len(report.Warnings) == 0 {
		t.Fatal("expected warning for native build path")
	}
	found := false
	for _, warning := range report.Warnings {
		if warning == "native_build_detected" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want native_build_detected", report.Warnings)
	}
}

func TestSteamValidateAllowsDomHeavySource(t *testing.T) {
	report := New().Validate(core.SourceInfo{
		Dir: t.TempDir(), Entry: "index.html",
		Report: core.Report{Level: core.LevelC, Blockers: []string{"dom_dependency"}},
	})
	if report.Level == core.LevelC || len(report.Blockers) > 0 {
		t.Fatalf("report = %+v, want desktop-compatible", report)
	}
	found := false
	for _, warning := range report.Warnings {
		if warning == "dom_supported_on_desktop" {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %v, want dom_supported_on_desktop", report.Warnings)
	}
}

func TestSteamBuildDomHeavyHTML(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<div id="app"></div><script src="app.js"></script>`)
	write(t, dir, "app.js", `document.querySelector('#app').innerHTML = '<b>x</b>';`)
	var buf bytes.Buffer
	_, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelC, Blockers: []string{"dom_dependency"}},
	}, &buf)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("expected non-empty package")
	}
}

func TestSteamBuildNativeExeSkipsElectronShell(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.exe", "MZ")
	write(t, dir, "data.pak", "assets")
	var buf bytes.Buffer
	manifest, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v2", ProjectID: "p1", SourceType: core.SourceEngineArtifact,
		Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entry != "app/game.exe" {
		t.Fatalf("entry = %q, want app/game.exe", manifest.Entry)
	}
	names := zipNames(t, buf.Bytes())
	for _, want := range []string{
		"app/game.exe", "app/data.pak", "steam/app_build.vdf.example",
		"CHANNEL_GUIDE.md", "package-manifest.json",
	} {
		if !names[want] {
			t.Errorf("missing %s", want)
		}
	}
	for _, bad := range []string{
		"electron/main.js", "electron/preload.js", "package.json",
		"electron-builder.yml", "build-steam.sh", "build-steam.ps1",
	} {
		if names[bad] {
			t.Errorf("native build should not include %s", bad)
		}
	}
}

func TestSteamBuildSkipsWorkspaceArtifacts(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `window.game = true`)
	write(t, dir, ".chathistory/session.jsonl", `{"message":"secret"}`)
	write(t, dir, ".git/config", "[remote]")
	write(t, dir, ".resources/tex.png", "resource")
	write(t, dir, "node_modules/x.js", "module.exports = 1")
	write(t, dir, "docs/a.md", "# docs")
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
		"app/.chathistory/session.jsonl", "app/.git/config", "app/.resources/tex.png",
		"app/node_modules/x.js", "app/docs/a.md",
		"app/package-manifest.json", "app/CHANNEL_GUIDE.md",
	} {
		if names[bad] {
			t.Errorf("workspace artifact %s was copied", bad)
		}
	}
	for _, want := range []string{"app/index.html", "app/game.js", "package-manifest.json", "CHANNEL_GUIDE.md"} {
		if !names[want] {
			t.Errorf("expected generated file %s in %v", want, names)
		}
	}
}

func TestSteamBuildManifestIncludesReportAndFiles(t *testing.T) {
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
	if manifest.ProjectID != "p1" || manifest.Channel != core.ChannelSteam ||
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
		if file.Path == "app/game.js" {
			found = true
			if file.Size == 0 || len(file.SHA256) != 12 {
				t.Fatalf("file summary = %+v", file)
			}
		}
	}
	if !found {
		t.Fatalf("app/game.js missing from %+v", manifest.Files)
	}

	var decoded core.Manifest
	if err := json.Unmarshal([]byte(zipContent(t, buf.Bytes(), "package-manifest.json")), &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.ProjectID != "p1" || decoded.Channel != core.ChannelSteam ||
		decoded.SourceType != core.SourceHTML5 || decoded.SourceVersion != "v9" ||
		decoded.Report.Level != core.LevelB || len(decoded.Files) == 0 {
		t.Fatalf("decoded manifest = %+v", decoded)
	}
}

func TestSteamBuildWritesChannelGuide(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas></canvas>`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	want := guides.Markdown(core.ChannelSteam)
	if strings.TrimSpace(want) == "" {
		t.Fatal("steam guide is empty")
	}
	if got := zipContent(t, buf.Bytes(), "CHANNEL_GUIDE.md"); got != want {
		t.Fatalf("guide = %q, want steam guide", got)
	}
}

func TestSteamBuildRejectsUnsupportedSource(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "readme.txt", "no html or exe")
	var buf bytes.Buffer
	_, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err == nil {
		t.Fatal("expected error for unsupported source")
	}
	if buf.Len() != 0 {
		t.Fatalf("partial zip written: %d bytes", buf.Len())
	}
}

func TestSteamBuildTemplatesElectronEntry(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.html", `<canvas></canvas>`)
	var buf bytes.Buffer
	if _, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "app.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
	}, &buf); err != nil {
		t.Fatal(err)
	}
	main := zipContent(t, buf.Bytes(), "electron/main.js")
	if !strings.Contains(main, `"app.html"`) {
		t.Fatalf("main.js does not reference app.html: %q", main)
	}
	if strings.Contains(main, `"index.html"`) {
		t.Fatalf("main.js still references index.html: %q", main)
	}
}

func TestSteamBuildSelectsAppHTMLWhenEntryEmpty(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.html", `<canvas></canvas>`)
	var buf bytes.Buffer
	manifest, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entry != "app.html" {
		t.Fatalf("entry = %q, want app.html", manifest.Entry)
	}
	names := zipNames(t, buf.Bytes())
	if !names["app/app.html"] {
		t.Fatalf("missing app/app.html in %v", names)
	}
}

func TestSteamBuildRejectsUnsafeEntry(t *testing.T) {
	for _, tc := range []struct {
		name  string
		entry string
	}{
		{"parent traversal", "../escape.html"},
		{"nested traversal", "sub/../../escape.html"},
		{"absolute path", "/tmp/escape.html"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			write(t, dir, "index.html", `<canvas></canvas>`)
			var buf bytes.Buffer
			_, err := New().Build(context.Background(), core.SourceInfo{
				Dir: dir, Entry: tc.entry, Version: "v1",
				Report: core.Report{Level: core.LevelB},
			}, &buf)
			if !errors.Is(err, ErrUnsupportedSource) {
				t.Fatalf("err = %v, want ErrUnsupportedSource", err)
			}
			if buf.Len() != 0 {
				t.Fatalf("partial zip written: %d bytes", buf.Len())
			}
		})
	}
}

func TestSteamBuildNativeManifestEntryUsesPackagedPath(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "Game.EXE", "MZ")
	var buf bytes.Buffer
	manifest, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Version: "v1", Report: core.Report{Level: core.LevelB},
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Entry != "app/Game.EXE" {
		t.Fatalf("entry = %q, want app/Game.EXE", manifest.Entry)
	}
}

func TestFindExecutableReturnsWalkError(t *testing.T) {
	if _, err := findExecutable(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected walk error for missing directory")
	}
}

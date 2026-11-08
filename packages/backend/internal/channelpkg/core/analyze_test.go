package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestAnalyzeHTMLClassifiesCanvasGame(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<html><body><canvas id="game"></canvas><script src="game.js"></script></body></html>`)
	write(t, dir, "game.js", `const c = document.getElementById('game'); const x = c.getContext('2d'); x.fillRect(0,0,10,10);`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B; report=%+v", report.Level, report)
	}
}

func TestAnalyzeHTMLClassifiesInteractiveCanvasGameAsB(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `
		const canvas = document.getElementById('game');
		const ctx = canvas.getContext('2d');
		localStorage.setItem('best', '1');
		window.addEventListener('keydown', () => {});
		document.getElementById('pause').addEventListener('click', () => {});
		document.querySelectorAll('.pad-button').forEach((b) => b.addEventListener('click', () => {}));
	`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B; report=%+v", report.Level, report)
	}
	if !strings.Contains(strings.Join(report.Warnings, ","), "dom_query_selectorall_limited") {
		t.Fatalf("warnings = %v, want dom_query_selectorall_limited", report.Warnings)
	}
}

func TestAnalyzeHTMLClassifiesDomGameAsC(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<div id="app"></div><script src="app.js"></script>`)
	write(t, dir, "app.js", `document.querySelector('#app').innerHTML = '<b>x</b>'; document.querySelectorAll('div');`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelC {
		t.Fatalf("level = %s, want C; report=%+v", report.Level, report)
	}
}

func TestAnalyzeHTMLRecognizesExistingMiniGame(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "game.js", "console.log('game')")
	write(t, dir, "game.json", `{"deviceOrientation":"portrait"}`)
	report, err := AnalyzeHTML(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelA {
		t.Fatalf("level = %s, want A", report.Level)
	}
}

func TestAnalyzeHTMLRejectsMissingEntry(t *testing.T) {
	dir := t.TempDir()
	if _, err := AnalyzeHTML(dir, "missing.html"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
}

func TestAnalyzeHTMLRejectsEscapingEntry(t *testing.T) {
	dir := t.TempDir()
	if _, err := AnalyzeHTML(dir, "../index.html"); !errors.Is(err, ErrInvalidSource) {
		t.Fatalf("err = %v, want ErrInvalidSource", err)
	}
}

func TestAnalyzeHTMLFindsDefaultAppEntry(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "app.html", `<canvas id="game"></canvas>`)
	report, err := AnalyzeHTML(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B", report.Level)
	}
}

func TestAnalyzeHTMLWarnsAboutExternalAndFetch(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `fetch('https://example.com/api'); fetch('http://example.com/x');`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B; report=%+v", report.Level, report)
	}
	joined := strings.Join(report.Warnings, ",")
	for _, want := range []string{"external_url", "network_fetch"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings = %v, want %s", report.Warnings, want)
		}
	}
}

func TestAnalyzeHTMLDoesNotDoubleCountQuerySelectorAll(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="app.js"></script>`)
	write(t, dir, "app.js", `document.querySelectorAll('div'); document.querySelectorAll('span');`)
	report, err := AnalyzeHTML(dir, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if report.Level != LevelB {
		t.Fatalf("level = %s, want B; report=%+v", report.Level, report)
	}
}

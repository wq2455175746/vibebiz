package core

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidSource = errors.New("invalid channel package source")

const maxAnalyzeFileSize = 2 << 20

var domMarkers = []string{
	"document.querySelector",
	"document.querySelectorAll",
	"innerHTML",
	"document.createElement",
	"addEventListener",
	"localStorage",
}

// AnalyzeHTML classifies an HTML5 source by how portable it is to mini-game channels.
func AnalyzeHTML(dir, entry string) (Report, error) {
	if entry != "" {
		if _, err := resolveEntry(dir, entry); err != nil {
			return Report{}, err
		}
	}
	if isMiniGameSource(dir) {
		return Report{Level: LevelA}, nil
	}
	if entry == "" {
		if _, err := resolveEntry(dir, ""); err != nil {
			return Report{}, err
		}
	}

	var canvas bool
	var domCount int
	var externalURL bool
	var networkFetch bool

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".html", ".js", ".css":
		default:
			return nil
		}
		data, err := readFileLimited(path, maxAnalyzeFileSize)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.Contains(strings.ToLower(text), "<canvas") {
			canvas = true
		}
		for _, marker := range domMarkers {
			domCount += strings.Count(text, marker)
		}
		if strings.Count(text, "http://") > 0 || strings.Count(text, "https://") > 0 {
			externalURL = true
		}
		if strings.Count(text, "fetch(") > 0 {
			networkFetch = true
		}
		return nil
	})
	if err != nil {
		return Report{}, err
	}

	report := Report{Warnings: []string{}}
	if externalURL {
		report.Warnings = append(report.Warnings, "external_url")
	}
	if networkFetch {
		report.Warnings = append(report.Warnings, "network_fetch")
	}
	if canvas && domCount <= 2 {
		report.Level = LevelB
	} else {
		report.Level = LevelC
		if domCount > 2 {
			report.Blockers = append(report.Blockers, "dom_dependency")
		}
	}
	return report, nil
}

func isMiniGameSource(dir string) bool {
	gameJS := filepath.Join(dir, "game.js")
	gameJSON := filepath.Join(dir, "game.json")
	jsInfo, err := os.Stat(gameJS)
	if err != nil || jsInfo.IsDir() {
		return false
	}
	jsonInfo, err := os.Stat(gameJSON)
	if err != nil || jsonInfo.IsDir() {
		return false
	}
	return true
}

func resolveEntry(dir, entry string) (string, error) {
	if entry != "" {
		return secureJoin(dir, entry)
	}
	for _, name := range []string{"index.html", "app.html"} {
		if target, err := secureJoin(dir, name); err == nil {
			return target, nil
		}
	}
	return "", ErrInvalidSource
}

func secureJoin(dir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrInvalidSource
	}
	target := filepath.Join(dir, clean)
	rel, err := filepath.Rel(dir, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidSource
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		return "", ErrInvalidSource
	}
	return target, nil
}

func readFileLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, limit))
}

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

// countDOMDependencies counts bytes that strongly couple the source to the DOM.
func countDOMDependencies(text string) int {
	queryAll := strings.Count(text, "document.querySelectorAll")
	withoutQueryAll := strings.ReplaceAll(text, "document.querySelectorAll", "")
	return queryAll +
		strings.Count(withoutQueryAll, "document.querySelector") +
		strings.Count(text, "innerHTML") +
		strings.Count(text, "document.createElement") +
		strings.Count(text, "addEventListener") +
		strings.Count(text, "localStorage")
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

	err := filepath.WalkDir(dir, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := LstatNoSymlink(current)
		if err != nil {
			return err
		}
		if SkipPath(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		if _, err := ResolveWithin(dir, current); err != nil {
			return err
		}
		switch strings.ToLower(filepath.Ext(current)) {
		case ".html", ".js", ".css":
		default:
			return nil
		}
		data, err := readFileLimited(current, maxAnalyzeFileSize)
		if err != nil {
			return err
		}
		text := string(data)
		if strings.Contains(strings.ToLower(text), "<canvas") {
			canvas = true
		}
		domCount += countDOMDependencies(text)
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
	for _, name := range []string{"game.js", "game.json"} {
		target := filepath.Join(dir, name)
		info, err := LstatNoSymlink(target)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		if _, err := ResolveWithin(dir, target); err != nil {
			return false
		}
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
	info, err := LstatNoSymlink(target)
	if err != nil || info.IsDir() {
		return "", ErrInvalidSource
	}
	if _, err := ResolveWithin(dir, target); err != nil {
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

package steam

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
)

const (
	adapterVersion   = "1.0.0"
	maxManifestFiles = 500
)

var ErrUnsupportedSource = errors.New("steam: source is not supported")

type buildMode int

const (
	modeHTML buildMode = iota
	modeNative
)

type Adapter struct{}

func New() *Adapter { return &Adapter{} }

func (a *Adapter) Channel() string { return core.ChannelSteam }

func (a *Adapter) Validate(source core.SourceInfo) core.Report {
	report := core.Report{
		Level:    source.Report.Level,
		Warnings: append([]string{}, source.Report.Warnings...),
		Blockers: append([]string{}, source.Report.Blockers...),
	}
	if source.Entry == "" {
		exe, err := findExecutable(source.Dir)
		if err != nil {
			report.Warnings = appendUnique(report.Warnings, "source_scan_failed")
		} else if exe != "" {
			report.Warnings = appendUnique(report.Warnings, "native_build_detected")
		}
	}
	return report
}

func (a *Adapter) Build(ctx context.Context, source core.SourceInfo, out io.Writer) (core.Manifest, error) {
	if source.Dir == "" {
		return core.Manifest{}, ErrUnsupportedSource
	}
	report := a.Validate(source)
	mode, entry, err := resolveMode(source)
	if err != nil {
		return core.Manifest{}, err
	}

	summaries, err := core.SummarizeFiles(ctx, source.Dir, skipWorkspacePath)
	if err != nil {
		return core.Manifest{}, err
	}

	manifest := core.Manifest{
		ProjectID:      source.ProjectID,
		Channel:        core.ChannelSteam,
		SourceType:     source.SourceType,
		SourceVersion:  source.Version,
		AdapterVersion: adapterVersion,
		Entry:          entry,
		GeneratedAt:    time.Now().UTC(),
		Level:          report.Level,
		Warnings:       append([]string{}, report.Warnings...),
		Report:         report,
		Files:          make([]core.FileSummary, 0, len(summaries)),
	}
	for _, summary := range summaries {
		manifest.Files = appendFileSummary(manifest.Files, core.FileSummary{
			Path:   "app/" + summary.Path,
			Size:   summary.Size,
			SHA256: summary.SHA256,
		})
	}

	tmp, err := os.CreateTemp("", "vibebiz-steam-*.zip")
	if err != nil {
		return core.Manifest{}, err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()
	zw := zip.NewWriter(tmp)

	written := func(name string, data []byte) error {
		if writeErr := core.WriteFile(zw, name, data); writeErr != nil {
			return writeErr
		}
		manifest.FileCount++
		manifest.TotalBytes += int64(len(data))
		manifest.Files = appendFileSummary(manifest.Files, summarizeBytes(name, data))
		return nil
	}

	count, total, err := core.CopyDirToZip(ctx, zw, source.Dir, "app", skipWorkspacePath)
	if err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}
	manifest.FileCount += count
	manifest.TotalBytes += total

	if mode == modeHTML {
		if err := writeElectronShell(written, entry); err != nil {
			_ = zw.Close()
			return core.Manifest{}, err
		}
	}
	if err := written("steam/app_build.vdf.example", []byte(appBuildVDF)); err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}
	guide := guides.Markdown(core.ChannelSteam)
	if strings.TrimSpace(guide) == "" {
		_ = zw.Close()
		return core.Manifest{}, errors.New("steam: missing channel guide")
	}
	if err := written("CHANNEL_GUIDE.md", []byte(guide)); err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}

	manifest.Report.Warnings = append([]string{}, manifest.Warnings...)
	if err := core.WriteJSON(zw, "package-manifest.json", manifest); err != nil {
		_ = zw.Close()
		return core.Manifest{}, err
	}
	if err := zw.Close(); err != nil {
		return core.Manifest{}, err
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return core.Manifest{}, err
	}
	if _, err := io.Copy(out, tmp); err != nil {
		return core.Manifest{}, err
	}
	return manifest, nil
}

func writeElectronShell(written func(string, []byte) error, entry string) error {
	files := []struct {
		name string
		data string
	}{
		{"electron/main.js", mainJS(entry)},
		{"electron/preload.js", preloadJS},
		{"package.json", packageJSON},
		{"electron-builder.yml", electronBuilderYML},
		{"build-steam.sh", buildSteamSH},
		{"build-steam.ps1", buildSteamPS1},
	}
	for _, file := range files {
		if err := written(file.name, []byte(file.data)); err != nil {
			return err
		}
	}
	return nil
}

func resolveMode(source core.SourceInfo) (buildMode, string, error) {
	if source.Entry != "" {
		entry, err := safeEntry(source.Dir, source.Entry)
		if err != nil {
			return 0, "", err
		}
		return modeHTML, entry, nil
	}
	for _, name := range []string{"index.html", "app.html"} {
		if regularFileExists(source.Dir, name) {
			return modeHTML, name, nil
		}
	}
	exe, err := findExecutable(source.Dir)
	if err != nil {
		return 0, "", err
	}
	if exe != "" {
		return modeNative, "app/" + exe, nil
	}
	return 0, "", ErrUnsupportedSource
}

func safeEntry(dir, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(name, "\\", "/")))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", ErrUnsupportedSource
	}
	target := filepath.Join(dir, clean)
	rel, err := filepath.Rel(dir, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrUnsupportedSource
	}
	info, err := os.Stat(target)
	if err != nil || !info.Mode().IsRegular() {
		return "", ErrUnsupportedSource
	}
	return filepath.ToSlash(rel), nil
}

func findExecutable(dir string) (string, error) {
	if dir == "" {
		return "", nil
	}
	var found string
	err := filepath.WalkDir(dir, func(current string, entry os.DirEntry, walkErr error) error {
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
		if skip := skipWorkspacePath(rel); skip {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(current), ".exe") {
			found = rel
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return found, nil
}

func skipWorkspacePath(rel string) bool {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	if len(parts) > 0 && (parts[0] == "docs" || parts[0] == "node_modules") {
		return true
	}
	switch parts[len(parts)-1] {
	case "package-manifest.json", "CHANNEL_GUIDE.md":
		return true
	}
	return false
}

func regularFileExists(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(name)))
	return err == nil && info.Mode().IsRegular()
}

func appendUnique(values []string, want string) []string {
	for _, value := range values {
		if value == want {
			return values
		}
	}
	return append(values, want)
}

func appendFileSummary(files []core.FileSummary, summary core.FileSummary) []core.FileSummary {
	if len(files) >= maxManifestFiles {
		return files
	}
	return append(files, summary)
}

func summarizeBytes(name string, data []byte) core.FileSummary {
	hasher := sha256.Sum256(data)
	return core.FileSummary{
		Path:   filepath.ToSlash(name),
		Size:   int64(len(data)),
		SHA256: hex.EncodeToString(hasher[:])[:12],
	}
}

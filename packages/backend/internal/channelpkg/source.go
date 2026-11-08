package channelpkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/resource"
)

var (
	ErrSourceNotFound = errors.New("channel package source not found")
	ErrInvalidSource  = errors.New("invalid channel package source")
)

var projectIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

const (
	maxSourceBytes = 1 << 30
	maxSourceFiles = 20000
)

type ResourceOpener func(ctx context.Context, projectID, resourceID string) (io.ReadCloser, int64, error)

type WorkspaceSource struct {
	Root      string
	ProjectID string
}

func NewWorkspaceSource(root, projectID string) (*WorkspaceSource, error) {
	if !projectIDPattern.MatchString(projectID) {
		return nil, ErrInvalidSource
	}
	return &WorkspaceSource{Root: root, ProjectID: projectID}, nil
}

func (s *WorkspaceSource) Materialize(ctx context.Context) (core.SourceInfo, error) {
	dir := filepath.Join(s.Root, s.ProjectID)
	stat, err := core.LstatNoSymlink(dir)
	if err != nil || !stat.IsDir() {
		return core.SourceInfo{}, ErrSourceNotFound
	}
	if _, err := core.ResolveWithin(s.Root, dir); err != nil {
		return core.SourceInfo{}, ErrInvalidSource
	}
	info, err := materializeDir(ctx, dir)
	if err != nil {
		return core.SourceInfo{}, err
	}
	info.ProjectID = s.ProjectID
	info.SourceType = core.SourceHTML5
	return info, nil
}

type ResourceSource struct {
	ProjectID  string
	ResourceID string
	Opener     ResourceOpener
	TempRoot   string
}

func NewResourceSource(projectID, resourceID string, opener ResourceOpener, tempRoot string) *ResourceSource {
	return &ResourceSource{
		ProjectID:  projectID,
		ResourceID: resourceID,
		Opener:     opener,
		TempRoot:   tempRoot,
	}
}

func (s *ResourceSource) Materialize(ctx context.Context) (core.SourceInfo, error) {
	if !projectIDPattern.MatchString(s.ProjectID) || !projectIDPattern.MatchString(s.ResourceID) {
		return core.SourceInfo{}, ErrInvalidSource
	}
	if s.Opener == nil {
		return core.SourceInfo{}, ErrInvalidSource
	}
	body, _, err := s.Opener(ctx, s.ProjectID, s.ResourceID)
	if err != nil {
		if errors.Is(err, resource.ErrNotFound) {
			return core.SourceInfo{}, ErrSourceNotFound
		}
		return core.SourceInfo{}, err
	}
	if body == nil {
		return core.SourceInfo{}, ErrInvalidSource
	}
	defer body.Close()

	dir, err := os.MkdirTemp(s.TempRoot, ".channel-src-"+s.ResourceID+"-")
	if err != nil {
		return core.SourceInfo{}, err
	}
	ok := false
	defer func() {
		if !ok {
			_ = os.RemoveAll(dir)
		}
	}()

	hasher := sha256.New()
	if err := core.ExtractZip(ctx, io.TeeReader(body, hasher), dir, core.ExtractLimits{MaxBytes: maxSourceBytes, MaxFiles: maxSourceFiles}); err != nil {
		return core.SourceInfo{}, err
	}
	info, err := materializeDir(ctx, dir)
	if err != nil {
		return core.SourceInfo{}, err
	}
	info.ProjectID = s.ProjectID
	info.SourceType = core.SourceEngineArtifact
	info.Version = hex.EncodeToString(hasher.Sum(nil))[:16]
	ok = true
	return info, nil
}

func (s *ResourceSource) Cleanup() error { return nil }

func materializeDir(ctx context.Context, dir string) (core.SourceInfo, error) {
	entry, err := findEntry(dir)
	native := false
	if err != nil {
		if hasMiniGame(dir) {
			entry = ""
		} else {
			exe, exeErr := findExecutable(dir)
			if exeErr != nil {
				return core.SourceInfo{}, exeErr
			}
			if exe == "" {
				return core.SourceInfo{}, err
			}
			native = true
		}
	}

	var report core.Report
	if native {
		report = core.Report{Level: core.LevelB, Warnings: []string{"native_build_detected"}}
	} else {
		report, err = core.AnalyzeHTML(dir, entry)
		if err != nil {
			return core.SourceInfo{}, normalizeSourceErr(err)
		}
	}

	count, total, version, err := dirStats(ctx, dir)
	if err != nil {
		return core.SourceInfo{}, err
	}
	return core.SourceInfo{
		Dir:        dir,
		Entry:      entry,
		Version:    version,
		FileCount:  count,
		TotalBytes: total,
		Report:     report,
	}, nil
}

func normalizeSourceErr(err error) error {
	if errors.Is(err, core.ErrInvalidSource) {
		return fmt.Errorf("%w: %v", ErrInvalidSource, err)
	}
	return err
}

func findEntry(dir string) (string, error) {
	for _, name := range []string{"index.html", "app.html"} {
		target := filepath.Join(dir, name)
		info, err := core.LstatNoSymlink(target)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		if _, err := core.ResolveWithin(dir, target); err != nil {
			continue
		}
		return name, nil
	}
	return "", ErrInvalidSource
}

func hasMiniGame(dir string) bool {
	for _, name := range []string{"game.js", "game.json"} {
		target := filepath.Join(dir, name)
		info, err := core.LstatNoSymlink(target)
		if err != nil || !info.Mode().IsRegular() {
			return false
		}
		if _, err := core.ResolveWithin(dir, target); err != nil {
			return false
		}
	}
	return true
}

func findExecutable(dir string) (string, error) {
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
		info, err := core.LstatNoSymlink(current)
		if err != nil {
			return err
		}
		if core.SkipPath(rel) {
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
		if _, err := core.ResolveWithin(dir, current); err != nil {
			return err
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

func dirStats(ctx context.Context, dir string) (int, int64, string, error) {
	hasher := sha256.New()
	var count int
	var total int64

	err := filepath.WalkDir(dir, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := core.LstatNoSymlink(current)
		if err != nil {
			return err
		}
		if core.SkipPath(rel) {
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
		if _, err := core.ResolveWithin(dir, current); err != nil {
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		fileHasher := sha256.New()
		size, copyErr := io.Copy(fileHasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		if _, err := fmt.Fprintf(hasher, "%s\x00%d\x00%s\n", filepath.ToSlash(rel), size, hex.EncodeToString(fileHasher.Sum(nil))); err != nil {
			return err
		}
		count++
		total += size
		return nil
	})
	if err != nil {
		return 0, 0, "", err
	}
	return count, total, hex.EncodeToString(hasher.Sum(nil))[:16], nil
}

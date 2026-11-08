package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
)

const maxFileSummaries = 500

func SummarizeFiles(ctx context.Context, dir string, skip func(rel string) bool) ([]FileSummary, error) {
	files := make([]FileSummary, 0, maxFileSummaries)
	err := filepath.WalkDir(dir, func(current string, entry os.DirEntry, walkErr error) error {
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
		info, err := LstatNoSymlink(current)
		if err != nil {
			return err
		}
		if skip != nil && skip(rel) {
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
		if len(files) >= maxFileSummaries {
			return filepath.SkipAll
		}
		summary, err := summarizeFile(current, rel)
		if err != nil {
			return err
		}
		files = append(files, summary)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func summarizeFile(path, rel string) (FileSummary, error) {
	file, err := os.Open(path)
	if err != nil {
		return FileSummary{}, err
	}
	defer file.Close()

	hasher := sha256.New()
	size, err := io.Copy(hasher, file)
	if err != nil {
		return FileSummary{}, err
	}
	return FileSummary{
		Path:   rel,
		Size:   size,
		SHA256: hex.EncodeToString(hasher.Sum(nil))[:12],
	}, nil
}

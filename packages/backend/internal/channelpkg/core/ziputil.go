package core

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrUnsafeArchive   = errors.New("unsafe archive")
	ErrArchiveTooLarge = errors.New("archive too large")
)

type ExtractLimits struct {
	MaxBytes int64
	MaxFiles int
}

func WriteFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(filepath.ToSlash(name))
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func WriteJSON(zw *zip.Writer, name string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return WriteFile(zw, name, data)
}

func CopyDirToZip(ctx context.Context, zw *zip.Writer, src, prefix string, skip func(rel string) bool) (int, int64, error) {
	var count int
	var total int64
	prefix = strings.Trim(filepath.ToSlash(prefix), "/")

	err := filepath.WalkDir(src, func(current string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if current == src {
			return nil
		}
		rel, err := filepath.Rel(src, current)
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
		if _, err := ResolveWithin(src, current); err != nil {
			return err
		}
		name := rel
		if prefix != "" {
			name = prefix + "/" + rel
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		written, copyErr := io.Copy(w, file)
		closeErr := file.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		count++
		total += written
		return nil
	})
	return count, total, err
}

func ExtractZip(ctx context.Context, src io.Reader, dst string, limits ExtractLimits) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	archive, archivePath, err := stageArchive(src, dst, limits.MaxBytes)
	if err != nil {
		return err
	}
	defer func() {
		_ = archive.Close()
		_ = os.Remove(archivePath)
	}()
	zr := &archive.Reader

	if limits.MaxFiles > 0 && len(zr.File) > limits.MaxFiles {
		return ErrArchiveTooLarge
	}

	var entries int
	var total int64
	for _, f := range zr.File {
		if err := ctx.Err(); err != nil {
			return err
		}
		entries++
		if limits.MaxFiles > 0 && entries > limits.MaxFiles {
			return ErrArchiveTooLarge
		}
		if f.Mode()&(os.ModeSymlink|os.ModeDevice|os.ModeCharDevice|os.ModeNamedPipe|os.ModeSocket|os.ModeIrregular) != 0 {
			return ErrUnsafeArchive
		}
		clean, err := safeArchiveName(f.Name)
		if err != nil {
			return err
		}
		target, err := archiveTarget(dst, clean)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}

		if limits.MaxBytes > 0 {
			uncompressed := int64(f.UncompressedSize64)
			if uncompressed > limits.MaxBytes || total > limits.MaxBytes-uncompressed {
				return ErrArchiveTooLarge
			}
		}
		if f.CompressedSize64 == 0 {
			if f.UncompressedSize64 > 0 {
				return ErrUnsafeArchive
			}
		} else if f.UncompressedSize64 > 100*f.CompressedSize64 {
			return ErrUnsafeArchive
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		remaining := int64(-1)
		if limits.MaxBytes > 0 {
			remaining = limits.MaxBytes - total
		}
		written, err := extractFile(target, f, remaining)
		if err != nil {
			_ = os.Remove(target)
			return err
		}
		total += written
	}
	return nil
}

func stageArchive(src io.Reader, dst string, maxBytes int64) (*zip.ReadCloser, string, error) {
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".channel-archive-*")
	if err != nil {
		return nil, "", err
	}
	name := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(name)
	}

	var written int64
	if maxBytes > 0 {
		written, err = io.Copy(tmp, io.LimitReader(src, maxBytes+1))
	} else {
		written, err = io.Copy(tmp, src)
	}
	if err != nil {
		cleanup()
		return nil, "", err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return nil, "", err
	}
	if maxBytes > 0 && written > maxBytes {
		_ = os.Remove(name)
		return nil, "", ErrArchiveTooLarge
	}
	zr, err := zip.OpenReader(name)
	if err != nil {
		_ = os.Remove(name)
		return nil, "", err
	}
	return zr, name, nil
}

func extractFile(target string, f *zip.File, remaining int64) (int64, error) {
	rc, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	written, err := writeExtracted(out, rc, remaining)
	if err != nil {
		return written, err
	}
	if remaining >= 0 && written > remaining {
		return written, ErrArchiveTooLarge
	}
	return written, nil
}

func writeExtracted(out io.Writer, in io.Reader, remaining int64) (int64, error) {
	if remaining < 0 {
		return io.Copy(out, in)
	}
	return io.Copy(out, io.LimitReader(in, remaining+1))
}

func safeArchiveName(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	if name == "" || strings.HasPrefix(name, "/") || filepath.IsAbs(name) {
		return "", ErrUnsafeArchive
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", ErrUnsafeArchive
	}
	return clean, nil
}

func archiveTarget(dst, clean string) (string, error) {
	target := filepath.Join(dst, filepath.FromSlash(clean))
	rel, err := filepath.Rel(dst, target)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrUnsafeArchive
	}
	return target, nil
}

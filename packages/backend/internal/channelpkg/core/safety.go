package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var ErrUnsafePath = errors.New("unsafe source path")

// SkipPath reports whether rel is excluded from channel packaging.
func SkipPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	if rel == "" || rel == "." {
		return false
	}
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return parts[0] == "docs" || parts[0] == "node_modules"
}

// LstatNoSymlink returns file info but rejects symbolic links.
func LstatNoSymlink(name string) (os.FileInfo, error) {
	info, err := os.Lstat(name)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%w: refusing symlink %q", ErrUnsafePath, name)
	}
	return info, nil
}

// ResolveWithin resolves target and verifies it stays under root.
func ResolveWithin(root, target string) (string, error) {
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%w: %q escapes source root", ErrUnsafePath, target)
	}
	return resolved, nil
}

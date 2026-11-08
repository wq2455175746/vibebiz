package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func skipSummaryTestPath(rel string) bool {
	rel = filepath.ToSlash(rel)
	parts := strings.Split(rel, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return len(parts) > 0 && (parts[0] == "docs" || parts[0] == "node_modules")
}

func TestSummarizeFilesSkipsPathsAndReturnsHashes(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "keep.js", "hello")
	write(t, dir, ".git/config", "ignored")
	write(t, dir, ".chathistory/session.jsonl", "ignored")
	write(t, dir, "docs/a.md", "ignored")
	write(t, dir, "node_modules/x.js", "ignored")

	files, err := SummarizeFiles(context.Background(), dir, skipSummaryTestPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Fatalf("files = %+v, want only keep.js", files)
	}
	if files[0].Path != "keep.js" || files[0].Size != 5 || len(files[0].SHA256) != 12 {
		t.Fatalf("summary = %+v", files[0])
	}
}

func TestSummarizeFilesCapsAt500(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 505; i++ {
		write(t, dir, filepath.ToSlash(filepath.Join("bulk", fmt.Sprintf("%03d.js", i))), "x")
	}
	files, err := SummarizeFiles(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 500 {
		t.Fatalf("files = %d, want 500", len(files))
	}
}

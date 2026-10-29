package observability

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDailyZipWriterRotatesAndCompresses(t *testing.T) {
	dir := t.TempDir()
	current := time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)
	writer, err := newDailyZipWriterWithClock(dir, "cp", 180, func() time.Time {
		return current
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := writer.Write([]byte("day-one\n")); err != nil {
		t.Fatal(err)
	}
	current = current.Add(24 * time.Hour)
	if _, err := writer.Write([]byte("day-two\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	oldPath := filepath.Join(dir, "cp_2026-10-29.log")
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old log should be removed, err = %v", err)
	}
	zipPath := oldPath + ".zip"
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if len(reader.File) != 1 {
		t.Fatalf("zip entries = %d", len(reader.File))
	}
	entry, err := reader.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer entry.Close()
	data, err := io.ReadAll(entry)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "day-one\n" {
		t.Fatalf("zip content = %q", data)
	}

	currentPath := filepath.Join(dir, "cp_2026-10-30.log")
	currentData, err := os.ReadFile(currentPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(currentData) != "day-two\n" {
		t.Fatalf("current log = %q", currentData)
	}
}

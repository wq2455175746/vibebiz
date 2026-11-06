package usertrace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureWritableCreatesNestedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "user-trace")
	if err := EnsureWritable(dir); err != nil {
		t.Fatalf("EnsureWritable() error = %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat trace dir: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("trace path is not a directory: %s", dir)
	}
}

func TestEnsureWritableRejectsReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to read-only directories")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod trace dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chmod(dir, 0o755)
	})
	if err := EnsureWritable(dir); err == nil {
		t.Fatal("EnsureWritable() error = nil, want error")
	}
}

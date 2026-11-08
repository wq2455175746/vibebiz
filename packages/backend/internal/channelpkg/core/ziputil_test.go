package core

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDirToZipRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	if err := os.Symlink("/etc/passwd", filepath.Join(dir, "link")); err != nil {
		t.Skip("symlink unsupported")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, _, err := CopyDirToZip(context.Background(), zw, dir, "", nil); err == nil {
		t.Fatal("expected symlink error")
	}
}

func TestCopyDirToZipRejectsSkippedSymlink(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "secret.js")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "source")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, ".leak.js")); err != nil {
		t.Skip("symlink unsupported")
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	_, _, err := CopyDirToZip(context.Background(), zw, dir, "", SkipPath)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("err = %v, want ErrUnsafePath", err)
	}
}

func TestExtractZipRejectsZipSlip(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, _ := zw.Create("../escape.txt")
	_, _ = w.Write([]byte("x"))
	_ = zw.Close()
	err := ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("err = %v, want ErrUnsafeArchive", err)
	}
}

func TestCopyDirToZipWritesPrefixedEntriesAndSkips(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "a.txt", "alpha")
	write(t, dir, "sub/b.txt", "beta")
	write(t, dir, "skip.txt", "gamma")

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	count, total, err := CopyDirToZip(context.Background(), zw, dir, "game", func(rel string) bool {
		return rel == "skip.txt"
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if count != 2 || total != int64(len("alpha")+len("beta")) {
		t.Fatalf("count=%d total=%d", count, total)
	}
	names := zipNameSet(t, buf.Bytes())
	if !names["game/a.txt"] || !names["game/sub/b.txt"] {
		t.Fatalf("names = %v", names)
	}
	if names["game/skip.txt"] {
		t.Fatalf("skip.txt should be skipped: %v", names)
	}
}

func TestExtractZipRoundTrip(t *testing.T) {
	src := t.TempDir()
	write(t, src, "index.html", `<canvas></canvas>`)
	write(t, src, "js/game.js", `console.log("hello channel")`)

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	if _, _, err := CopyDirToZip(context.Background(), zw, src, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ExtractZip(context.Background(), &buf, dst, ExtractLimits{MaxBytes: 1 << 20, MaxFiles: 100}); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"index.html", "js/game.js"} {
		if _, err := os.Stat(filepath.Join(dst, filepath.FromSlash(name))); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
}

func TestExtractZipRejectsArchiveTooLargeBytes(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "big.txt", Method: zip.Store}
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("x"), 2048)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	err = ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("err = %v, want ErrArchiveTooLarge", err)
	}
}

func TestExtractZipRejectsTooManyFiles(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{"a.txt", "b.txt"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	err := ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 1})
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("err = %v, want ErrArchiveTooLarge", err)
	}
}

func TestExtractZipRejectsTooManyDirectoryEntries(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for i := 0; i < 5; i++ {
		if _, err := zw.Create(fmt.Sprintf("dir%d/", i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	err := ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1 << 20, MaxFiles: 3})
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("err = %v, want ErrArchiveTooLarge", err)
	}
}

func TestExtractZipStreamsStoredEntry(t *testing.T) {
	payload := bytes.Repeat([]byte("0123456789abcdef"), 256<<10)
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "big.bin", Method: zip.Store}
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	dst := t.TempDir()
	if err := ExtractZip(context.Background(), bytes.NewReader(buf.Bytes()), dst, ExtractLimits{MaxBytes: 16 << 20, MaxFiles: 10}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("extracted size = %d, want %d", len(got), len(payload))
	}
}

func TestExtractZipRejectsExcessiveCompressionRatio(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("bomb.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(bytes.Repeat([]byte("a"), 1<<20)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	err = ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 10 << 20, MaxFiles: 10})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("err = %v, want ErrUnsafeArchive", err)
	}
}

func TestExtractZipRejectsSymlinkEntry(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	h := &zip.FileHeader{Name: "link", Method: zip.Store}
	h.SetMode(os.ModeSymlink | 0o777)
	w, err := zw.CreateHeader(h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("/etc/passwd")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	err = ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
	if !errors.Is(err, ErrUnsafeArchive) {
		t.Fatalf("err = %v, want ErrUnsafeArchive", err)
	}
}

type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

func TestExtractZipBoundsArchiveRead(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	src := &countingReader{r: io.MultiReader(
		bytes.NewReader(buf.Bytes()),
		bytes.NewReader(bytes.Repeat([]byte("x"), 64<<10)),
	)}
	err = ExtractZip(context.Background(), src, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
	if !errors.Is(err, ErrArchiveTooLarge) {
		t.Fatalf("err = %v, want ErrArchiveTooLarge", err)
	}
	if src.n > 1025 {
		t.Fatalf("read %d bytes, want at most 1025", src.n)
	}
}

func TestExtractZipRejectsSpecialFileModes(t *testing.T) {
	for _, mode := range []os.FileMode{os.ModeNamedPipe, os.ModeSocket, os.ModeDevice} {
		t.Run(mode.String(), func(t *testing.T) {
			var buf bytes.Buffer
			zw := zip.NewWriter(&buf)
			h := &zip.FileHeader{Name: "special", Method: zip.Store}
			h.SetMode(mode | 0o644)
			w, err := zw.CreateHeader(h)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			if err := zw.Close(); err != nil {
				t.Fatal(err)
			}
			err = ExtractZip(context.Background(), &buf, t.TempDir(), ExtractLimits{MaxBytes: 1024, MaxFiles: 10})
			if !errors.Is(err, ErrUnsafeArchive) {
				t.Fatalf("err = %v, want ErrUnsafeArchive", err)
			}
		})
	}
}

func zipNameSet(t *testing.T, data []byte) map[string]bool {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool, len(zr.File))
	for _, f := range zr.File {
		names[f.Name] = true
	}
	return names
}

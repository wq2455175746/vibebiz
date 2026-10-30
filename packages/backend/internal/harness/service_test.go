package harness

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"testing"
)

type fakeSource struct {
	data []byte
	err  error
	key  string
}

func (f *fakeSource) GetRaw(_ context.Context, key string) (io.ReadCloser, int64, error) {
	f.key = key
	if f.err != nil {
		return nil, 0, f.err
	}
	return io.NopCloser(bytes.NewReader(f.data)), int64(len(f.data)), nil
}

type fakeImporter struct {
	data []byte
	id   string
	err  error
}

func (f *fakeImporter) ImportArchive(_ context.Context, id string, data []byte) error {
	f.id = id
	f.data = append([]byte(nil), data...)
	return f.err
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func tarFiles(t *testing.T, data []byte) map[string]string {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	archive := tar.NewReader(reader)
	files := make(map[string]string)
	for {
		header, err := archive.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		files[header.Name] = string(content)
	}
	return files
}

func TestImportStripsSingleRootDirectory(t *testing.T) {
	archive := zipBytes(t, map[string]string{
		"vibebiz-harness/README.md":     "# harness",
		"vibebiz-harness/app/main.py":   "print('ok')",
		"vibebiz-harness/app/routes.py": "routes = []",
	})
	source := &fakeSource{data: archive}
	importer := &fakeImporter{}
	service := NewService(source, importer, "tmp/vibebiz_prod/vibebiz-harness.zip")

	if err := service.Import(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	if source.key != "tmp/vibebiz_prod/vibebiz-harness.zip" {
		t.Fatalf("s3 key = %q", source.key)
	}
	if importer.id != "p1" {
		t.Fatalf("project id = %q", importer.id)
	}
	files := tarFiles(t, importer.data)
	if files["README.md"] != "# harness" || files["app/main.py"] != "print('ok')" {
		t.Fatalf("files = %#v", files)
	}
	if _, ok := files["vibebiz-harness/README.md"]; ok {
		t.Fatalf("single root was not stripped: %#v", files)
	}
}

func TestImportSkipsMacOSMetadataAndStripsRoot(t *testing.T) {
	archive := zipBytes(t, map[string]string{
		"vibebiz-harness/README.md":              "# harness",
		"vibebiz-harness/app/main.py":            "print('ok')",
		"__MACOSX/vibebiz-harness/._README.md":   "meta",
		"__MACOSX/vibebiz-harness/app/._main.py": "meta",
		"vibebiz-harness/.DS_Store":              "meta",
	})
	importer := &fakeImporter{}
	service := NewService(&fakeSource{data: archive}, importer, "harness.zip")

	if err := service.Import(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	files := tarFiles(t, importer.data)
	if len(files) != 2 || files["README.md"] != "# harness" || files["app/main.py"] != "print('ok')" {
		t.Fatalf("files = %#v", files)
	}
}

func TestImportKeepsMultipleRootDirectories(t *testing.T) {
	archive := zipBytes(t, map[string]string{
		"README.md":       "root",
		"backend/main.go": "package main",
	})
	importer := &fakeImporter{}
	service := NewService(&fakeSource{data: archive}, importer, "harness.zip")

	if err := service.Import(context.Background(), "p1"); err != nil {
		t.Fatal(err)
	}
	files := tarFiles(t, importer.data)
	if files["README.md"] != "root" || files["backend/main.go"] != "package main" {
		t.Fatalf("files = %#v", files)
	}
}

func TestImportRejectsInvalidZip(t *testing.T) {
	importer := &fakeImporter{}
	service := NewService(&fakeSource{data: []byte("not a zip")}, importer, "harness.zip")

	if err := service.Import(context.Background(), "p1"); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("err = %v, want ErrInvalidArchive", err)
	}
	if len(importer.data) != 0 {
		t.Fatal("sandbox import should not be called")
	}
}

func TestImportRejectsPathTraversal(t *testing.T) {
	archive := zipBytes(t, map[string]string{
		"harness/../../evil.txt": "bad",
	})
	importer := &fakeImporter{}
	service := NewService(&fakeSource{data: archive}, importer, "harness.zip")

	if err := service.Import(context.Background(), "p1"); !errors.Is(err, ErrInvalidArchive) {
		t.Fatalf("err = %v, want ErrInvalidArchive", err)
	}
	if len(importer.data) != 0 {
		t.Fatal("sandbox import should not be called")
	}
}

func TestImportPropagatesSourceError(t *testing.T) {
	sourceErr := errors.New("s3 down")
	service := NewService(&fakeSource{err: sourceErr}, &fakeImporter{}, "harness.zip")

	if err := service.Import(context.Background(), "p1"); !errors.Is(err, sourceErr) {
		t.Fatalf("err = %v, want source error", err)
	}
}

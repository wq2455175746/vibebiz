package harness

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
)

const (
	DefaultMaxArchiveBytes = 100 << 20
	DefaultMaxFiles        = 10000
)

var ErrInvalidArchive = errors.New("invalid harness archive")

type Source interface {
	GetRaw(ctx context.Context, key string) (io.ReadCloser, int64, error)
}

type Importer interface {
	ImportArchive(ctx context.Context, id string, data []byte) error
}

type Service struct {
	Source          Source
	Importer        Importer
	Key             string
	MaxArchiveBytes int64
	MaxFiles        int
}

func NewService(source Source, importer Importer, key string) *Service {
	return &Service{
		Source:          source,
		Importer:        importer,
		Key:             strings.TrimPrefix(strings.TrimSpace(key), "/"),
		MaxArchiveBytes: DefaultMaxArchiveBytes,
		MaxFiles:        DefaultMaxFiles,
	}
}

type archiveFile struct {
	name    string
	content []byte
	modTime time.Time
}

func (s *Service) Import(ctx context.Context, projectID string) error {
	body, _, err := s.Source.GetRaw(ctx, s.Key)
	if err != nil {
		return err
	}
	defer body.Close()

	data, err := io.ReadAll(io.LimitReader(body, s.maxBytes()+1))
	if err != nil {
		return fmt.Errorf("read harness archive: %w", err)
	}
	if int64(len(data)) > s.maxBytes() {
		return fmt.Errorf("%w: archive too large", ErrInvalidArchive)
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidArchive, err)
	}
	files, err := s.files(archive)
	if err != nil {
		return err
	}
	converted, err := tarGz(files)
	if err != nil {
		return err
	}
	if err := s.Importer.ImportArchive(ctx, projectID, converted); err != nil {
		return err
	}
	return nil
}

func (s *Service) maxBytes() int64 {
	if s.MaxArchiveBytes <= 0 {
		return DefaultMaxArchiveBytes
	}
	return s.MaxArchiveBytes
}

func (s *Service) maxFiles() int {
	if s.MaxFiles <= 0 {
		return DefaultMaxFiles
	}
	return s.MaxFiles
}

func (s *Service) files(archive *zip.Reader) ([]archiveFile, error) {
	if len(archive.File) > s.maxFiles() {
		return nil, fmt.Errorf("%w: too many files", ErrInvalidArchive)
	}
	files := make([]archiveFile, 0, len(archive.File))
	var total int64
	for _, entry := range archive.File {
		if entry.FileInfo().IsDir() {
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: symlink is not allowed", ErrInvalidArchive)
		}
		if skipMacOSMetadata(entry.Name) {
			continue
		}
		name, err := cleanName(entry.Name)
		if err != nil {
			return nil, err
		}
		if entry.UncompressedSize64 > uint64(s.maxBytes()) || total+int64(entry.UncompressedSize64) > s.maxBytes() {
			return nil, fmt.Errorf("%w: uncompressed data too large", ErrInvalidArchive)
		}
		reader, err := entry.Open()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
		}
		content, err := io.ReadAll(io.LimitReader(reader, int64(entry.UncompressedSize64)+1))
		closeErr := reader.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, err)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidArchive, closeErr)
		}
		if int64(len(content)) != int64(entry.UncompressedSize64) {
			return nil, fmt.Errorf("%w: file size mismatch", ErrInvalidArchive)
		}
		total += int64(len(content))
		files = append(files, archiveFile{name: name, content: content, modTime: entry.Modified})
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("%w: archive is empty", ErrInvalidArchive)
	}
	root, strip := commonRoot(files)
	if strip {
		for index := range files {
			files[index].name = strings.TrimPrefix(files[index].name, root+"/")
		}
	}
	return files, nil
}

func skipMacOSMetadata(name string) bool {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	for _, part := range strings.Split(name, "/") {
		if part == "__MACOSX" {
			return true
		}
	}
	base := path.Base(name)
	return base == ".DS_Store" || strings.HasPrefix(base, "._")
}

func cleanName(name string) (string, error) {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	if name == "" || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: invalid file name", ErrInvalidArchive)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == ".." || path.IsAbs(cleaned) || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("%w: invalid archive path", ErrInvalidArchive)
	}
	return cleaned, nil
}

func commonRoot(files []archiveFile) (string, bool) {
	root := ""
	for _, file := range files {
		parts := strings.Split(file.name, "/")
		if len(parts) < 2 {
			return "", false
		}
		if root == "" {
			root = parts[0]
			continue
		}
		if parts[0] != root {
			return "", false
		}
	}
	return root, root != ""
}

func tarGz(files []archiveFile) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	tarWriter := tar.NewWriter(gzipWriter)
	for _, file := range files {
		header := &tar.Header{
			Name:     file.name,
			Mode:     0o644,
			Size:     int64(len(file.content)),
			Typeflag: tar.TypeReg,
			ModTime:  file.modTime,
		}
		if err := tarWriter.WriteHeader(header); err != nil {
			return nil, fmt.Errorf("write harness header: %w", err)
		}
		if _, err := tarWriter.Write(file.content); err != nil {
			return nil, fmt.Errorf("write harness file: %w", err)
		}
	}
	if err := tarWriter.Close(); err != nil {
		return nil, fmt.Errorf("close harness tar: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("close harness gzip: %w", err)
	}
	return buffer.Bytes(), nil
}

package observability

import (
	"archive/zip"
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func newLogger(serviceName, plane string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	options := &slog.HandlerOptions{Level: level}
	handlers := []slog.Handler{slog.NewJSONHandler(os.Stdout, options)}
	if dir := strings.TrimSpace(os.Getenv("LOG_DIR")); dir != "" {
		writer, err := newDailyZipWriter(dir, plane, 180)
		if err != nil {
			slog.New(slog.NewJSONHandler(os.Stderr, nil)).Error("open log file failed", "error", err)
		} else {
			handlers = append(handlers, slog.NewTextHandler(writer, options))
		}
	}

	var handler slog.Handler = handlers[0]
	if len(handlers) > 1 {
		handler = newMultiHandler(handlers...)
	}
	return slog.New(handler).With("service", serviceName, "plane", plane)
}

type multiHandler struct {
	handlers []slog.Handler
}

func newMultiHandler(handlers ...slog.Handler) *multiHandler {
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, handler := range m.handlers {
		if handler.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	var firstErr error
	for _, handler := range m.handlers {
		if !handler.Enabled(ctx, record.Level) {
			continue
		}
		if err := handler.Handle(ctx, record.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for index, handler := range m.handlers {
		handlers[index] = handler.WithAttrs(attrs)
	}
	return &multiHandler{handlers: handlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	handlers := make([]slog.Handler, len(m.handlers))
	for index, handler := range m.handlers {
		handlers[index] = handler.WithGroup(name)
	}
	return &multiHandler{handlers: handlers}
}

type dailyZipWriter struct {
	mu            sync.Mutex
	dir           string
	prefix        string
	retentionDays int
	now           func() time.Time
	day           string
	file          *os.File
}

func newDailyZipWriter(dir, prefix string, retentionDays int) (*dailyZipWriter, error) {
	return newDailyZipWriterWithClock(dir, prefix, retentionDays, time.Now)
}

func newDailyZipWriterWithClock(dir, prefix string, retentionDays int, now func() time.Time) (*dailyZipWriter, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = "backend"
	}
	if retentionDays <= 0 {
		retentionDays = 180
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	writer := &dailyZipWriter{
		dir: dir, prefix: prefix, retentionDays: retentionDays, now: now,
	}
	if err := writer.open(now().Format("2006-01-02")); err != nil {
		return nil, err
	}
	return writer, nil
}

func (w *dailyZipWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	now := w.now()
	day := now.Format("2006-01-02")
	if w.file == nil || day != w.day {
		if err := w.rotate(day); err != nil {
			return 0, err
		}
	}
	return w.file.Write(data)
}

func (w *dailyZipWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func (w *dailyZipWriter) open(day string) error {
	path := filepath.Join(w.dir, w.prefix+"_"+day+".log")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = file
	w.day = day
	return nil
}

func (w *dailyZipWriter) rotate(day string) error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
		w.file = nil
	}
	if w.day != "" {
		oldPath := filepath.Join(w.dir, w.prefix+"_"+w.day+".log")
		if err := compressLogFile(oldPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	w.cleanup()
	return w.open(day)
}

func (w *dailyZipWriter) cleanup() {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	cutoff := w.now().AddDate(0, 0, -w.retentionDays)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(w.dir, entry.Name()))
	}
}

func compressLogFile(path string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()

	target, err := os.Create(path + ".zip")
	if err != nil {
		return err
	}
	archive := zip.NewWriter(target)
	entry, err := archive.Create(filepath.Base(path))
	if err != nil {
		_ = archive.Close()
		_ = target.Close()
		_ = os.Remove(path + ".zip")
		return err
	}
	if _, err := io.Copy(entry, source); err != nil {
		_ = archive.Close()
		_ = target.Close()
		_ = os.Remove(path + ".zip")
		return err
	}
	if err := archive.Close(); err != nil {
		_ = target.Close()
		_ = os.Remove(path + ".zip")
		return err
	}
	if err := target.Close(); err != nil {
		_ = os.Remove(path + ".zip")
		return err
	}
	return os.Remove(path)
}

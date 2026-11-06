package usertrace

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const batchSize = 500

type Syncer struct {
	dir      string
	plane    string
	sink     Sink
	interval time.Duration
	logger   *slog.Logger
	now      func() time.Time
	mu       sync.Mutex
}

func NewSyncer(dir, plane string, sink Sink, interval time.Duration, logger *slog.Logger) *Syncer {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Syncer{dir: dir, plane: plane, sink: sink, interval: interval, logger: logger, now: time.Now}
}

func (s *Syncer) Run(ctx context.Context) {
	if s == nil || s.sink == nil {
		return
	}
	s.run(ctx)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.run(ctx)
		}
	}
}

func (s *Syncer) run(ctx context.Context) {
	if err := s.SyncOnce(ctx); err != nil && s.logger != nil {
		s.logger.Warn("user trace sync failed", "error", err)
	}
}

func (s *Syncer) SyncOnce(ctx context.Context) error {
	if s == nil || s.sink == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncOnceLocked(ctx)
}

func (s *Syncer) syncOnceLocked(ctx context.Context) error {
	root := filepath.Join(s.dir, s.plane)
	live, err := filepath.Glob(filepath.Join(root, "*", "*.log"))
	if err != nil {
		return err
	}
	if err := s.rotateLiveFiles(live); err != nil {
		return err
	}

	pending, err := filepath.Glob(filepath.Join(root, "*", "*.syncing"))
	if err != nil {
		return err
	}
	var firstErr error
	for _, path := range pending {
		if err := s.flushFile(ctx, path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Syncer) rotateLiveFiles(live []string) error {
	traceFileMu.Lock()
	defer traceFileMu.Unlock()
	for _, path := range live {
		target := strings.TrimSuffix(path, ".log") + fmt.Sprintf(".%d.syncing", s.now().UnixNano())
		if err := os.Rename(path, target); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("rotate user trace file: %w", err)
		}
	}
	return nil
}

func (s *Syncer) flushFile(ctx context.Context, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open user trace file: %w", err)
	}
	defer file.Close()

	var batch []Event
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := s.sink.Write(ctx, batch); err != nil {
			return err
		}
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var event Event
		if err := json.Unmarshal(line, &event); err != nil {
			continue
		}
		if event.EventID == "" {
			continue
		}
		batch = append(batch, event)
		if len(batch) >= batchSize {
			if err := flush(); err != nil {
				return fmt.Errorf("write user trace batch: %w", err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan user trace file: %w", err)
	}
	if err := flush(); err != nil {
		return fmt.Errorf("write user trace batch: %w", err)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove synced user trace file: %w", err)
	}
	return nil
}

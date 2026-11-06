package usertrace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeSink struct {
	batches [][]Event
	err     error
}

func (f *fakeSink) Write(_ context.Context, events []Event) error {
	if f.err != nil {
		return f.err
	}
	copied := append([]Event(nil), events...)
	f.batches = append(f.batches, copied)
	return nil
}

func writeTraceFile(t *testing.T, root, day, name string, events []Event) string {
	t.Helper()
	dir := filepath.Join(root, "cp", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		line, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSyncerSyncOnce(t *testing.T) {
	root := t.TempDir()
	path := writeTraceFile(t, root, "2026-11-06", "7.log", []Event{
		{EventID: "e1", EventType: EventRequest, UserID: 7},
		{EventID: "e2", EventType: EventLogin, UserID: 7},
	})
	sink := &fakeSink{}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || len(sink.batches[0]) != 2 {
		t.Fatalf("batches = %+v", sink.batches)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("live file still exists: %v", err)
	}
	if pending, _ := filepath.Glob(filepath.Join(root, "cp", "2026-11-06", "*.syncing")); len(pending) != 0 {
		t.Fatalf("pending files = %v", pending)
	}
}

func TestSyncerKeepsFileOnSinkFailure(t *testing.T) {
	root := t.TempDir()
	writeTraceFile(t, root, "2026-11-06", "7.log", []Event{{EventID: "e1", EventType: EventRequest, UserID: 7}})
	sink := &fakeSink{err: errors.New("db down")}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err == nil {
		t.Fatal("SyncOnce() error = nil, want failure")
	}
	if pending, _ := filepath.Glob(filepath.Join(root, "cp", "2026-11-06", "*.syncing")); len(pending) != 1 {
		t.Fatalf("pending files = %v", pending)
	}
}

func TestSyncerSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "cp", "2026-11-06")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "not-json\n" + `{"eventId":"e1","eventType":"request","userId":7}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "7.log"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	sink := &fakeSink{}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	if err := syncer.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.batches) != 1 || len(sink.batches[0]) != 1 || sink.batches[0][0].EventID != "e1" {
		t.Fatalf("batches = %+v", sink.batches)
	}
}

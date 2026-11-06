package usertrace

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

type concurrencyTrackingSink struct {
	mu        sync.Mutex
	active    int
	maxActive int
	batches   int
	events    int
}

func (s *concurrencyTrackingSink) Write(_ context.Context, events []Event) error {
	s.mu.Lock()
	s.active++
	if s.active > s.maxActive {
		s.maxActive = s.active
	}
	s.mu.Unlock()

	time.Sleep(5 * time.Millisecond)

	s.mu.Lock()
	s.active--
	s.batches++
	s.events += len(events)
	s.mu.Unlock()
	return nil
}

func TestSyncerSyncOnceSingleFlight(t *testing.T) {
	root := t.TempDir()
	events := make([]Event, 1200)
	for i := range events {
		events[i] = Event{EventID: fmt.Sprintf("e%d", i), EventType: EventRequest, UserID: 7}
	}
	writeTraceFile(t, root, "2026-11-06", "7.log", events)
	sink := &concurrencyTrackingSink{}
	syncer := NewSyncer(root, "cp", sink, time.Minute, nil)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- syncer.SyncOnce(context.Background())
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("SyncOnce() error = %v", err)
		}
	}

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.maxActive != 1 {
		t.Fatalf("max concurrent writes = %d, want 1", sink.maxActive)
	}
	if sink.batches != 3 || sink.events != 1200 {
		t.Fatalf("batches = %d, events = %d, want 3 and 1200", sink.batches, sink.events)
	}
}

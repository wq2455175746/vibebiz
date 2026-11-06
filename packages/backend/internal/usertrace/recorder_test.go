package usertrace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRecorderAppendsPerUserPerDay(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder(dir, "cp", nil)
	r.now = func() time.Time { return time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local) }
	r.newID = func() string { return "event-1" }

	r.Append(Event{
		EventType: EventRequest, UserID: 7, Username: "wang",
		IP: "1.2.3.4", RequestMethod: "GET", RequestPath: "/cp/api/v1/projects",
		StatusCode: 200, UserAgent: "Chrome/120.0",
	})

	path := filepath.Join(dir, "cp", "2026-11-06", "7.log")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log: %v", err)
	}
	var event Event
	if err := json.Unmarshal(body, &event); err != nil {
		t.Fatalf("decode event: %v (%s)", err, body)
	}
	if event.EventID != "event-1" || event.UserID != 7 || event.DeviceType != "desktop" || event.Browser != "Chrome" {
		t.Fatalf("event = %+v", event)
	}
	if event.EventAt.IsZero() {
		t.Fatal("event time not filled")
	}
}

func TestRecorderUsesAnonymousForUnknownUser(t *testing.T) {
	dir := t.TempDir()
	r := NewRecorder(dir, "cp", nil)
	r.now = func() time.Time { return time.Date(2026, 11, 6, 10, 0, 0, 0, time.Local) }
	r.newID = func() string { return "event-2" }

	r.Append(Event{EventType: EventLogin, LoginResult: "failed"})

	if _, err := os.Stat(filepath.Join(dir, "cp", "2026-11-06", "anonymous.log")); err != nil {
		t.Fatalf("anonymous log: %v", err)
	}
}

func TestNilRecorderDoesNotPanic(t *testing.T) {
	var r *Recorder
	r.Append(Event{})
}

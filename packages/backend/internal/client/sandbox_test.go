package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSandboxInitProject(t *testing.T) {
	var called bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost || r.URL.Path != "/internal/projects/p1/init" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":0,"message":"ok","result":{}}`))
	}))
	defer srv.Close()

	if err := NewSandbox(srv.URL, nil).InitProject(context.Background(), "p1"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if !called {
		t.Fatal("sandbox not called")
	}
}

func TestSandboxReadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/projects/p1/file" || r.URL.Query().Get("path") != "index.html" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.Write([]byte(`{"code":0,"message":"ok","result":{"path":"index.html","content":"<h1>hi</h1>","version":"abc","size":11,"mtime":1,"isBinary":false}}`))
	}))
	defer srv.Close()

	file, err := NewSandbox(srv.URL, nil).ReadFile(context.Background(), "p1", "index.html")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if file.Content != "<h1>hi</h1>" || file.Version != "abc" {
		t.Fatalf("unexpected file: %+v", file)
	}
}

func TestSandboxProjectStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/projects/p1/stats" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		w.Write([]byte(`{"code":0,"message":"ok","result":{"fileSizeBytes":123,"fileCount":4,"dirCount":1,"lastModifiedAt":1793191707598}}`))
	}))
	defer srv.Close()

	stats, err := NewSandbox(srv.URL, nil).ProjectStats(context.Background(), "p1")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.FileSizeBytes != 123 || stats.FileCount != 4 || stats.DirCount != 1 || stats.LastModifiedAt != 1793191707598 {
		t.Fatalf("stats = %+v", stats)
	}
}

func TestSandboxWriteConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"code":409,"message":"version conflict"}`))
	}))
	defer srv.Close()

	_, err := NewSandbox(srv.URL, nil).WriteFile(context.Background(), "p1", "index.html", "x", "stale")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want APIError", err)
	}
	if apiErr.Status != http.StatusConflict {
		t.Fatalf("status = %d, want 409", apiErr.Status)
	}
}

func TestSandboxChatLogs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/internal/projects/p1/logs" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("limit") != "100" {
			t.Fatalf("limit = %q", r.URL.Query().Get("limit"))
		}
		w.Write([]byte(`{"code":0,"message":"ok","result":{"logs":[{"ts":"2026-10-30T01:00:00.000Z","level":"info","source":"agent","message":"agent.run.start"}]}}`))
	}))
	defer srv.Close()

	logs, err := NewSandbox(srv.URL, nil).ChatLogs(context.Background(), "p1", 100)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if len(logs) != 1 || logs[0].Message != "agent.run.start" {
		t.Fatalf("logs = %+v", logs)
	}
}

func TestSandboxParseTask(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/internal/tasks/parse" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "message": "ok",
			"result": map[string]any{
				"name": "检查", "description": "检查依赖", "scheduleType": "daily",
				"time": "09:00", "weekdays": []int{}, "cronExpression": "0 9 * * *",
				"timezone": "Asia/Shanghai",
			},
		})
	}))
	defer server.Close()
	sandbox := NewSandbox(server.URL, nil)
	draft, err := sandbox.ParseTask(context.Background(), "每天 9 点检查依赖")
	if err != nil {
		t.Fatal(err)
	}
	if draft.ScheduleType != "daily" || draft.CronExpression != "0 9 * * *" {
		t.Fatalf("draft = %+v", draft)
	}
}

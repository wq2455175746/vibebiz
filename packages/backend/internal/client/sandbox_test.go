package client

import (
	"context"
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

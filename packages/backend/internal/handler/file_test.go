package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/client"
)

type fakeFiles struct {
	writeErr error
	readErr  error
}

func (f *fakeFiles) ListFiles(_ context.Context, _ string) ([]client.FileEntry, error) {
	return []client.FileEntry{{Path: "index.html", Type: "file", Size: 10}}, nil
}

func (f *fakeFiles) ReadFile(_ context.Context, _, path string) (*client.FileContent, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return &client.FileContent{Path: path, Content: "<h1>hi</h1>", Version: "v1"}, nil
}

func (f *fakeFiles) WriteFile(_ context.Context, _, path, _, _ string) (*client.WriteResult, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	return &client.WriteResult{Path: path, Version: "v2"}, nil
}

func TestWriteFileConflict(t *testing.T) {
	h := NewFileHandler(&fakeFiles{writeErr: &client.APIError{Status: http.StatusConflict, Code: 409, Message: "version conflict"}}, nil)
	e := echo.New()
	e.PUT("/dp/api/v1/projects/:id/file", h.Write)

	req := httptest.NewRequest(http.MethodPut, "/dp/api/v1/projects/p1/file", strings.NewReader(`{"path":"index.html","content":"x","baseVersion":"stale"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Message != "version conflict" {
		t.Fatalf("message = %q, want sandbox 409 message", env.Message)
	}
}

func TestWriteFileSanitizesInternalError(t *testing.T) {
	h := NewFileHandler(&fakeFiles{writeErr: errors.New("dial tcp 10.0.0.1: secret detail")}, nil)
	e := echo.New()
	e.PUT("/dp/api/v1/projects/:id/file", h.Write)

	req := httptest.NewRequest(http.MethodPut, "/dp/api/v1/projects/p1/file", strings.NewReader(`{"path":"index.html","content":"x"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	var env struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if strings.Contains(env.Message, "secret") || strings.Contains(env.Message, "dial tcp") {
		t.Fatalf("message leaked internal error: %q", env.Message)
	}
	if env.Message != "write file failed" {
		t.Fatalf("message = %q, want generic fallback", env.Message)
	}
}

func TestReadFile(t *testing.T) {
	h := NewFileHandler(&fakeFiles{}, nil)
	e := echo.New()
	e.GET("/dp/api/v1/projects/:id/file", h.Read)

	req := httptest.NewRequest(http.MethodGet, "/dp/api/v1/projects/p1/file?path=index.html", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var env struct {
		Result client.FileContent `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Result.Content != "<h1>hi</h1>" {
		t.Fatalf("content = %q", env.Result.Content)
	}
}

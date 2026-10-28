package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type fakeInitializer struct {
	err      error
	calledID string
}

func (f *fakeInitializer) InitProject(_ context.Context, id string) error {
	f.calledID = id
	return f.err
}

func TestCreateProject(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	init := &fakeInitializer{}
	h := NewProjectHandler(projectStore, init, nil)
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	e := echo.New()
	e.POST("/cp/api/v1/projects", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"make a snake game"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var env struct {
		Code   int           `json:"code"`
		Result model.Project `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != 0 || env.Result.Prompt != "make a snake game" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if init.calledID != env.Result.ID {
		t.Fatalf("sandbox called with %q, project id %q", init.calledID, env.Result.ID)
	}
	if _, err := projectStore.Get(env.Result.ID); err != nil {
		t.Fatalf("project not stored: %v", err)
	}
}

func TestCreateProjectRequiresPrompt(t *testing.T) {
	h := NewProjectHandler(store.NewMemoryProjectStore(), &fakeInitializer{}, nil)
	e := echo.New()
	e.POST("/cp/api/v1/projects", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"  "}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestCreateProjectSandboxFailure(t *testing.T) {
	projectStore := store.NewMemoryProjectStore()
	h := NewProjectHandler(projectStore, &fakeInitializer{err: errors.New("boom")}, nil)
	e := echo.New()
	e.POST("/cp/api/v1/projects", h.Create)

	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/projects", strings.NewReader(`{"prompt":"hello"}`))
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
	if strings.Contains(env.Message, "boom") {
		t.Fatalf("message leaked internal error: %q", env.Message)
	}
	if env.Message != "workspace init failed" {
		t.Fatalf("message = %q, want generic message", env.Message)
	}
	if len(projectStore.List()) != 0 {
		t.Fatal("project should not be stored when init fails")
	}
}

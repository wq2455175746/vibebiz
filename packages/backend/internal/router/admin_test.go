package router

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"vibebiz/internal/admin"
	"vibebiz/internal/model"
	"vibebiz/internal/store"
)

type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Result  json.RawMessage `json:"result"`
}

func decodeEnvelope(t *testing.T, body []byte) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Code != 0 {
		t.Fatalf("code = %d, message = %s", env.Code, env.Message)
	}
	return env
}

func TestAdminOverview(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(&model.Project{ID: "p1", Name: "demo", State: model.ProjectStateReady, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	e, _ := newCP(t, projects)

	rec := serve(e, http.MethodGet, "/cp/api/v1/admin/overview")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeEnvelope(t, rec.Body.Bytes())
	var overview admin.Overview
	if err := json.Unmarshal(env.Result, &overview); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	if overview.Projects != 1 || overview.Users != 2 || overview.RunningSandboxes != 1 || overview.TodayRequests != 128 {
		t.Fatalf("unexpected overview: %+v", overview)
	}
}

func TestAdminUsers(t *testing.T) {
	e, _ := newCP(t, store.NewMemoryProjectStore())
	rec := serve(e, http.MethodGet, "/cp/api/v1/admin/users")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var users []admin.User
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &users); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(users) != 2 || users[0].ID == "" || users[0].CreatedAt.IsZero() {
		t.Fatalf("unexpected users: %+v", users)
	}
}

func TestAdminProjects(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(&model.Project{ID: "p1", Name: "demo", State: model.ProjectStateReady, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	e, _ := newCP(t, projects)

	rec := serve(e, http.MethodGet, "/cp/api/v1/admin/projects")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload struct {
		Projects []struct {
			ID    string `json:"id"`
			Name  string `json:"name"`
			State string `json:"state"`
			Owner string `json:"owner"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].Owner != "wangh11" {
		t.Fatalf("unexpected projects: %+v", payload.Projects)
	}
}

func TestAdminSandboxesQuotasAudit(t *testing.T) {
	e, _ := newCP(t, store.NewMemoryProjectStore())

	for _, tc := range []struct {
		path   string
		count  int
		decode func([]byte) (int, error)
	}{
		{"/cp/api/v1/admin/sandboxes", 2, func(raw []byte) (int, error) {
			var v []admin.Sandbox
			err := json.Unmarshal(raw, &v)
			return len(v), err
		}},
		{"/cp/api/v1/admin/quotas", 2, func(raw []byte) (int, error) {
			var v []admin.Quota
			err := json.Unmarshal(raw, &v)
			return len(v), err
		}},
		{"/cp/api/v1/admin/audit", 3, func(raw []byte) (int, error) {
			var v []admin.AuditEvent
			err := json.Unmarshal(raw, &v)
			return len(v), err
		}},
	} {
		rec := serve(e, http.MethodGet, tc.path)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d", tc.path, rec.Code)
		}
		env := decodeEnvelope(t, rec.Body.Bytes())
		got, err := tc.decode(env.Result)
		if err != nil {
			t.Fatalf("%s decode: %v", tc.path, err)
		}
		if got != tc.count {
			t.Fatalf("%s count = %d, want %d", tc.path, got, tc.count)
		}
	}
}

package router

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
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
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", Name: "demo", State: model.ProjectStateReady, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	e, _, authState := newCP(t, projects)

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/overview", authState.opsCookie)
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
	e, _, authState := newCP(t, store.NewMemoryProjectStore())
	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/users", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload struct {
		Users []admin.User `json:"users"`
		Total int64        `json:"total"`
		Page  int          `json:"page"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(payload.Users) != 2 || payload.Total != 2 || payload.Page != 1 || payload.Users[0].ID == "" || payload.Users[0].CreatedAt.IsZero() {
		t.Fatalf("unexpected users: %+v", payload)
	}
}

func TestAdminUsersSearchAndPagination(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())
	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/users?keyword=vistor&page=1&pageSize=10", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload struct {
		Users []admin.User `json:"users"`
		Total int64        `json:"total"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatalf("decode users: %v", err)
	}
	if len(payload.Users) != 1 || payload.Total != 1 || payload.Users[0].Name != "vistor" {
		t.Fatalf("unexpected users: %+v", payload)
	}
}

func TestAdminUpdatesUserStatus(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())
	authState.sessions.items[routerSessionKey(auth.RealmUser, "user1-sid")] = &auth.Session{
		ID: "user1-sid", Realm: auth.RealmUser, UserID: 1, Username: "wanghg11", WorkspaceIDs: []int64{1},
	}
	user1Cookie := cookieForSession(t, authState.authn.UserCookie, "user1-sid")

	rec := serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/1/status", authState.opsCookie, `{"status":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing reason status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/1/status", authState.opsCookie, `{"status":0,"reason":"违规内容"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/1/status", authState.userCookie, `{"status":1}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/99/status", authState.opsCookie, `{"status":1}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", rec.Code)
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/users?keyword=wanghg11", authState.opsCookie)
	var payload struct {
		Users []admin.User `json:"users"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Users) != 1 || payload.Users[0].Status != "disabled" {
		t.Fatalf("users = %+v", payload.Users)
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/users/1", authState.opsCookie)
	var detail admin.UserDetail
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.DisabledReason != "违规内容" || detail.DisabledBy != "admin" || detail.Status != "disabled" {
		t.Fatalf("detail = %+v", detail)
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects", user1Cookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("disabled session status = %d, want 401", rec.Code)
	}
}

func TestAdminUserDetailAndPassword(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/users/1", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var detail admin.UserDetail
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &detail); err != nil {
		t.Fatal(err)
	}
	if detail.ID != "1" || detail.Username != "wanghg11" || detail.Status != "active" {
		t.Fatalf("detail = %+v", detail)
	}

	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/1/password", authState.opsCookie, `{"password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/users/1/password", authState.opsCookie, `{"password":"new-password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("password status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestAdminChangesOwnPassword(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())
	authState.sessions.items[routerSessionKey(auth.RealmOps, "ops-other")] = &auth.Session{
		ID: "ops-other", Realm: auth.RealmOps, UserID: 9, Username: "admin",
	}

	rec := serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/auth/password", authState.userCookie, `{"oldPassword":"admin-pass","newPassword":"new-password"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user cookie status = %d, want 401", rec.Code)
	}

	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/auth/password", authState.opsCookie, `{"oldPassword":"admin-pass","newPassword":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status = %d, body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/auth/password", authState.opsCookie, `{"oldPassword":"wrong","newPassword":"new-password"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("wrong old password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var failure envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &failure); err != nil {
		t.Fatalf("decode failure: %v", err)
	}
	if failure.Code != 40002 {
		t.Fatalf("failure code = %d, want 40002", failure.Code)
	}

	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/auth/password", authState.opsCookie, `{"oldPassword":"admin-pass","newPassword":"new-password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("change password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !auth.VerifyPassword(authState.ops.updatedHash, "new-password") {
		t.Fatal("password hash not updated")
	}
	if _, ok := authState.sessions.items[routerSessionKey(auth.RealmOps, "ops-sid")]; !ok {
		t.Fatal("current session should remain")
	}
	if _, ok := authState.sessions.items[routerSessionKey(auth.RealmOps, "ops-other")]; ok {
		t.Fatal("other session should be revoked")
	}
}

func TestAdminUpdatesQuota(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	body := `{"llmTokensQuota":800000,"sandboxMinutesQuota":900,"storageQuotaMb":2048}`
	rec := serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/quotas/1", authState.opsCookie, body)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/quotas/1", authState.userCookie, body)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user status = %d, want 401", rec.Code)
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/quotas/99", authState.opsCookie, body)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d, want 404", rec.Code)
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/quotas", authState.opsCookie)
	var quotas []admin.Quota
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &quotas); err != nil {
		t.Fatal(err)
	}
	if len(quotas) != 2 || quotas[0].LLMTokensQuota != 800000 || quotas[0].SandboxMinutesQuota != 900 || quotas[0].StorageQuotaMB != 2048 {
		t.Fatalf("quotas = %+v", quotas)
	}
}

func TestAdminProjects(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	now := time.Now()
	if err := projects.Create(context.Background(), &model.Project{ID: "p1", Name: "demo", State: model.ProjectStateReady, WorkspaceID: 2, OwnerUserID: 7, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := projects.Create(context.Background(), &model.Project{ID: "p2", Name: "other", State: model.ProjectStateReady, WorkspaceID: 5, OwnerUserID: 8, CreatedAt: now.Add(-time.Minute)}); err != nil {
		t.Fatal(err)
	}
	e, _, authState := newCP(t, projects)

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/projects", authState.opsCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var payload struct {
		Projects []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			State       string `json:"state"`
			Owner       string `json:"owner"`
			OwnerUserID int64  `json:"ownerUserId"`
			WorkspaceID int64  `json:"workspaceId"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatalf("decode projects: %v", err)
	}
	if len(payload.Projects) != 2 {
		t.Fatalf("unexpected projects: %+v", payload.Projects)
	}
	found := false
	for _, project := range payload.Projects {
		if project.ID == "p1" {
			found = true
			if project.Owner != "7" || project.OwnerUserID != 7 || project.WorkspaceID != 2 {
				t.Fatalf("project ownership = %+v", project)
			}
		}
	}
	if !found {
		t.Fatalf("admin did not see all projects: %+v", payload.Projects)
	}
}

func TestAdminTogglesProjectHidden(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{
		ID: "p1", Name: "demo", State: model.ProjectStateReady, WorkspaceID: 2, OwnerUserID: 7, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	e, _, authState := newCP(t, projects)
	authState.metas.items["p1"] = auth.ProjectMeta{WorkspaceID: 2, OwnerUserID: 7}

	rec := serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/projects/p1/hidden", authState.opsCookie, `{"hidden":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("hide status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if meta := authState.metas.items["p1"]; meta.Status != "hidden" {
		t.Fatalf("meta = %+v", meta)
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/projects", authState.opsCookie)
	var payload struct {
		Projects []struct {
			ID     string `json:"id"`
			Hidden bool   `json:"hidden"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Projects) != 1 || !payload.Projects[0].Hidden {
		t.Fatalf("projects = %+v", payload.Projects)
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects/p1", authState.userCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("hidden project status = %d, want 404", rec.Code)
	}
	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/projects/p1/review", authState.opsCookie, `{"status":"approved","note":""}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if meta := authState.metas.items["p1"]; meta.Status != "hidden" {
		t.Fatalf("meta after approve = %+v", meta)
	}

	rec = serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/projects/p1/hidden", authState.opsCookie, `{"hidden":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("unhide status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if meta := authState.metas.items["p1"]; meta.Status != "" {
		t.Fatalf("meta after unhide = %+v", meta)
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects/p1", authState.userCookie)
	if rec.Code != http.StatusOK {
		t.Fatalf("visible project status = %d, want 200", rec.Code)
	}
}

func TestAdminRejectsProjectAndDisablesIt(t *testing.T) {
	projects := store.NewMemoryProjectStore()
	if err := projects.Create(context.Background(), &model.Project{
		ID: "p1", Name: "bad", State: model.ProjectStateReady, WorkspaceID: 2, OwnerUserID: 7, CreatedAt: time.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	e, _, authState := newCP(t, projects)

	rec := serveWithAuthBody(e, http.MethodPut, "/cp/api/v1/admin/projects/p1/review", authState.opsCookie,
		`{"status":"rejected","note":"违规内容"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/projects", authState.opsCookie)
	var payload struct {
		Projects []struct {
			ID           string `json:"id"`
			State        string `json:"state"`
			ReviewStatus string `json:"reviewStatus"`
			ReviewNote   string `json:"reviewNote"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(decodeEnvelope(t, rec.Body.Bytes()).Result, &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Projects) != 1 || payload.Projects[0].State != model.ProjectStateDisabled ||
		payload.Projects[0].ReviewStatus != model.ReviewStatusRejected || payload.Projects[0].ReviewNote != "违规内容" {
		t.Fatalf("projects = %+v", payload.Projects)
	}
}

func TestAdminRequiresOpsSession(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	if rec := serve(e, http.MethodGet, "/cp/api/v1/admin/overview"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", rec.Code)
	}
	if rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/overview", authState.userCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("user cookie status = %d, want 401", rec.Code)
	}
}

func TestProjectsRequireUserSession(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	if rec := serve(e, http.MethodGet, "/cp/api/v1/projects"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", rec.Code)
	}
	if rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects", authState.opsCookie); rec.Code != http.StatusUnauthorized {
		t.Fatalf("ops cookie status = %d, want 401", rec.Code)
	}
	if rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/projects", authState.userCookie); rec.Code != http.StatusOK {
		t.Fatalf("user status = %d, want 200", rec.Code)
	}
}

func TestAdminSandboxesQuotasAudit(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	for _, tc := range []struct {
		path   string
		count  int
		decode func([]byte) (int, error)
	}{
		{"/cp/api/v1/admin/sandboxes", 2, func(raw []byte) (int, error) {
			var v struct {
				Items []admin.Sandbox `json:"sandboxes"`
			}
			err := json.Unmarshal(raw, &v)
			return len(v.Items), err
		}},
		{"/cp/api/v1/admin/quotas", 2, func(raw []byte) (int, error) {
			var v []admin.Quota
			err := json.Unmarshal(raw, &v)
			return len(v), err
		}},
		{"/cp/api/v1/admin/audit", 3, func(raw []byte) (int, error) {
			var v struct {
				Items []admin.AuditEvent `json:"audit"`
			}
			err := json.Unmarshal(raw, &v)
			return len(v.Items), err
		}},
	} {
		rec := serveWithAuth(e, http.MethodGet, tc.path, authState.opsCookie)
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

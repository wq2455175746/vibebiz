package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/admin"
	"vibebiz/internal/auth"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
	"vibebiz/internal/store"
)

type fakeAdminRepository struct {
	detail       admin.UserDetail
	statusInput  admin.UserStatusUpdate
	statusID     int64
	passwordID   int64
	passwordHash string
}

func (f *fakeAdminRepository) Overview(context.Context, int) (admin.Overview, error) {
	return admin.Overview{}, nil
}

func (f *fakeAdminRepository) Users(context.Context, admin.ListQuery) (admin.Page[admin.User], error) {
	return admin.Page[admin.User]{}, nil
}

func (f *fakeAdminRepository) User(_ context.Context, id int64) (admin.UserDetail, error) {
	if id != 1 {
		return admin.UserDetail{}, admin.ErrNotFound
	}
	return f.detail, nil
}

func (f *fakeAdminRepository) UpdateUserStatus(_ context.Context, id int64, input admin.UserStatusUpdate) error {
	if id != 1 {
		return admin.ErrNotFound
	}
	f.statusID = id
	f.statusInput = input
	if input.Status == 1 {
		f.detail.Status = "active"
	} else {
		f.detail.Status = "disabled"
		f.detail.DisabledReason = input.Reason
		f.detail.DisabledAt = &input.At
		f.detail.DisabledBy = input.Operator
	}
	return nil
}

func (f *fakeAdminRepository) UpdateUserPassword(_ context.Context, id int64, hash string) error {
	if id != 1 {
		return admin.ErrNotFound
	}
	f.passwordID = id
	f.passwordHash = hash
	return nil
}

func (f *fakeAdminRepository) Sandboxes(context.Context, admin.ListQuery) (admin.Page[admin.Sandbox], error) {
	return admin.Page[admin.Sandbox]{}, nil
}

func (f *fakeAdminRepository) Quotas(context.Context) ([]admin.Quota, error) {
	return nil, nil
}

func (f *fakeAdminRepository) UpdateQuota(context.Context, int64, admin.QuotaUpdateInput) error {
	return nil
}

func (f *fakeAdminRepository) Audit(context.Context, admin.ListQuery) (admin.Page[admin.AuditEvent], error) {
	return admin.Page[admin.AuditEvent]{}, nil
}

func (f *fakeAdminRepository) RecordAudit(context.Context, admin.AuditRecord) error {
	return nil
}

type fakeAdminSessionRevoker struct {
	realm  auth.Realm
	userID int64
	calls  int
}

func (f *fakeAdminSessionRevoker) DeleteByUser(_ context.Context, realm auth.Realm, userID int64) error {
	f.realm = realm
	f.userID = userID
	f.calls++
	return nil
}

type fakeAdminNotifications struct {
	created   notification.CreateInput
	publishes int
}

func (f *fakeAdminNotifications) Create(_ context.Context, input notification.CreateInput, _ notification.Creator) (notification.View, error) {
	f.created = input
	return notification.View{ID: 9}, nil
}

func (f *fakeAdminNotifications) Publish(_ context.Context, id int64) (notification.View, error) {
	f.publishes++
	return notification.View{ID: id}, nil
}

func adminUserRequest(e *echo.Echo, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func newAdminUserHandler(repo admin.Repository) *AdminHandler {
	h := NewAdminHandler(repo, store.NewMemoryProjectStore())
	h.Now = func() time.Time { return time.Unix(1700000000, 0).UTC() }
	return h
}

func TestAdminDisableRequiresReason(t *testing.T) {
	repo := &fakeAdminRepository{detail: admin.UserDetail{ID: "1", Status: "active"}}
	h := newAdminUserHandler(repo)
	e := echo.New()
	e.PUT("/users/:id/status", h.UpdateUserStatus)

	rec := adminUserRequest(e, http.MethodPut, "/users/1/status", `{"status":0}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.statusID != 0 {
		t.Fatal("status update should not run without a reason")
	}
}

func TestAdminDisablePersistsReasonAndNotifiesUser(t *testing.T) {
	repo := &fakeAdminRepository{detail: admin.UserDetail{ID: "1", Status: "active"}}
	notifications := &fakeAdminNotifications{}
	revoker := &fakeAdminSessionRevoker{}
	h := newAdminUserHandler(repo)
	h.Notifications = notifications
	h.Sessions = revoker
	e := echo.New()
	routeWithSession(e, http.MethodPut, "/users/:id/status", h.UpdateUserStatus, &auth.Session{
		ID: "sid", Realm: auth.RealmOps, UserID: 3, Username: "admin",
	})

	rec := adminUserRequest(e, http.MethodPut, "/users/1/status", `{"status":0,"reason":"  违规内容  "}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.statusID != 1 || repo.statusInput.Status != 0 || repo.statusInput.Reason != "违规内容" || repo.statusInput.Operator != "admin" {
		t.Fatalf("status input = %+v", repo.statusInput)
	}
	if notifications.created.RecipientUserID != 1 || notifications.created.Type != model.NotificationInbox {
		t.Fatalf("notification input = %+v", notifications.created)
	}
	if !strings.Contains(notifications.created.Content, "违规内容") || notifications.publishes != 1 {
		t.Fatalf("notification = %+v publishes = %d", notifications.created, notifications.publishes)
	}
	if revoker.calls != 1 || revoker.realm != auth.RealmUser || revoker.userID != 1 {
		t.Fatalf("revoker = %+v", revoker)
	}
}

func TestAdminEnableDoesNotNotify(t *testing.T) {
	repo := &fakeAdminRepository{detail: admin.UserDetail{ID: "1", Status: "disabled"}}
	notifications := &fakeAdminNotifications{}
	revoker := &fakeAdminSessionRevoker{}
	h := newAdminUserHandler(repo)
	h.Notifications = notifications
	h.Sessions = revoker
	e := echo.New()
	routeWithSession(e, http.MethodPut, "/users/:id/status", h.UpdateUserStatus, &auth.Session{
		ID: "sid", Realm: auth.RealmOps, UserID: 3, Username: "admin",
	})

	rec := adminUserRequest(e, http.MethodPut, "/users/1/status", `{"status":1,"reason":"ignored"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.statusInput.Reason != "" || notifications.publishes != 0 || revoker.calls != 0 {
		t.Fatalf("enable input = %+v publishes = %d revokes = %d", repo.statusInput, notifications.publishes, revoker.calls)
	}
}

func TestAdminUserDetailSignsAvatar(t *testing.T) {
	repo := &fakeAdminRepository{detail: admin.UserDetail{
		ID: "1", Username: "wanghg11", AvatarS3Path: "avatar/1/a.png", AvatarURL: "https://old",
	}}
	h := newAdminUserHandler(repo)
	h.Covers = fakeCoverURLSigner{}
	e := echo.New()
	e.GET("/users/:id", h.UserDetail)

	rec := adminUserRequest(e, http.MethodGet, "/users/1", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var envelope struct {
		Result admin.UserDetail `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Result.AvatarURL != "https://s3.example/avatar/1/a.png" {
		t.Fatalf("avatar url = %q", envelope.Result.AvatarURL)
	}
}

func TestAdminUpdatesUserPassword(t *testing.T) {
	repo := &fakeAdminRepository{detail: admin.UserDetail{ID: "1", Status: "active"}}
	h := newAdminUserHandler(repo)
	e := echo.New()
	e.PUT("/users/:id/password", h.UpdateUserPassword)

	rec := adminUserRequest(e, http.MethodPut, "/users/1/password", `{"password":"short"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("short password status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.passwordID != 0 {
		t.Fatal("password update should not run for a short password")
	}

	rec = adminUserRequest(e, http.MethodPut, "/users/1/password", `{"password":"new-password"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if repo.passwordID != 1 || !auth.VerifyPassword(repo.passwordHash, "new-password") {
		t.Fatalf("password hash = %q", repo.passwordHash)
	}
}

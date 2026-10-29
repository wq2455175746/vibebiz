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

	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
)

var notificationTestNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type notificationRepo struct {
	items  map[int64]model.OpsNotification
	nextID int64
}

func newNotificationRepo() *notificationRepo {
	return &notificationRepo{items: make(map[int64]model.OpsNotification)}
}

func (r *notificationRepo) add(item model.OpsNotification) model.OpsNotification {
	r.nextID++
	item.ID = r.nextID
	r.items[item.ID] = item
	return item
}

func (r *notificationRepo) List(_ context.Context, filter dao.NotificationFilter, offset, limit int) ([]model.OpsNotification, int64, error) {
	out := make([]model.OpsNotification, 0, len(r.items))
	for _, item := range r.items {
		if filter.Type != "" && string(item.Type) != filter.Type {
			continue
		}
		if filter.Status != "" && string(item.Status) != filter.Status {
			continue
		}
		out = append(out, item)
	}
	total := int64(len(out))
	if offset >= len(out) {
		return []model.OpsNotification{}, total, nil
	}
	end := offset + limit
	if end > len(out) {
		end = len(out)
	}
	return out[offset:end], total, nil
}

func (r *notificationRepo) Get(_ context.Context, id int64) (*model.OpsNotification, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return &item, nil
}

func (r *notificationRepo) Create(_ context.Context, item *model.OpsNotification) error {
	r.nextID++
	item.ID = r.nextID
	r.items[item.ID] = *item
	return nil
}

func (r *notificationRepo) Update(_ context.Context, item *model.OpsNotification) error {
	if _, ok := r.items[item.ID]; !ok {
		return dao.ErrNotFound
	}
	r.items[item.ID] = *item
	return nil
}

func (r *notificationRepo) UpdateStatus(_ context.Context, id int64, status string, publishedAt *time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return dao.ErrNotFound
	}
	item.Status = model.NotificationStatus(status)
	item.PublishedAt = publishedAt
	r.items[id] = item
	return nil
}

func (r *notificationRepo) Delete(_ context.Context, id int64) error {
	if _, ok := r.items[id]; !ok {
		return dao.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *notificationRepo) Published(_ context.Context, now time.Time) ([]model.OpsNotification, error) {
	out := make([]model.OpsNotification, 0)
	for _, item := range r.items {
		if item.Status == model.NotificationPublished && item.ExpireAt.After(now) {
			out = append(out, item)
		}
	}
	return out, nil
}

type notificationCache struct {
	items    []notification.Published
	lastRead time.Time
}

func (c *notificationCache) Put(_ context.Context, _ model.OpsNotification) error {
	return nil
}

func (c *notificationCache) Remove(_ context.Context, _ int64) error {
	return nil
}

func (c *notificationCache) List(_ context.Context) ([]notification.Published, error) {
	return c.items, nil
}

func (c *notificationCache) LastRead(_ context.Context, _ int64) (time.Time, error) {
	return c.lastRead, nil
}

func (c *notificationCache) MarkRead(_ context.Context, _ int64, readAt time.Time) error {
	if readAt.After(c.lastRead) {
		c.lastRead = readAt
	}
	return nil
}

func (c *notificationCache) Rebuild(_ context.Context, items []model.OpsNotification) error {
	c.items = make([]notification.Published, 0, len(items))
	for _, item := range items {
		publishedAt := item.CreatedAt
		if item.PublishedAt != nil {
			publishedAt = *item.PublishedAt
		}
		c.items = append(c.items, notification.Published{
			ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
			Version: item.Version, Link: item.Link, PublishedAt: publishedAt, ExpireAt: item.ExpireAt,
		})
	}
	return nil
}

func notificationFixture(t *testing.T) (*notificationRepo, *notificationCache, *NotificationHandler, *notification.Hub) {
	t.Helper()
	repo := newNotificationRepo()
	cache := &notificationCache{}
	hub := notification.NewHub()
	t.Cleanup(hub.Close)
	service := notification.NewService(repo, cache, hub)
	service.Now = func() time.Time { return notificationTestNow }
	return repo, cache, NewNotificationHandler(service, hub, nil), hub
}

func notificationRouter(h *NotificationHandler, session *auth.Session) *echo.Echo {
	e := echo.New()
	ops := e.Group("/admin", sessionMiddleware(session))
	ops.GET("/notifications", h.AdminList)
	ops.POST("/notifications", h.AdminCreate)
	ops.PUT("/notifications/:id", h.AdminUpdate)
	ops.POST("/notifications/:id/publish", h.AdminPublish)
	ops.POST("/notifications/:id/unpublish", h.AdminUnpublish)
	ops.DELETE("/notifications/:id", h.AdminDelete)
	e.GET("/notifications", h.WebList, sessionMiddleware(session), requireSession)
	e.GET("/notifications/ws", h.Socket, sessionMiddleware(session), requireSession)
	e.POST("/notifications/read", h.MarkRead, sessionMiddleware(session), requireSession)
	return e
}

func requireSession(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if _, ok := auth.SessionFrom(c); !ok {
			return responseUnauthorized(c)
		}
		return next(c)
	}
}

func responseUnauthorized(c echo.Context) error {
	return c.JSON(http.StatusUnauthorized, map[string]any{"code": 40101, "message": "unauthorized"})
}

func opsSession() *auth.Session {
	return &auth.Session{ID: "ops-sid", Realm: auth.RealmOps, UserID: 9, Username: "admin", Role: "super_admin"}
}

func notificationRequestRecorder(e *echo.Echo, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func createNotificationBody(t *testing.T) string {
	t.Helper()
	payload := map[string]any{
		"title":    "Release 1.2.0",
		"content":  "New features",
		"type":     model.NotificationInbox,
		"expireAt": notificationTestNow.Add(time.Hour).Format(time.RFC3339),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestNotificationAdminCreateRequiresSession(t *testing.T) {
	_, _, h, _ := notificationFixture(t)
	e := notificationRouter(h, nil)

	rec := notificationRequestRecorder(e, http.MethodPost, "/admin/notifications", createNotificationBody(t))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40101 {
		t.Fatalf("code = %d, want 40101", env.Code)
	}
}

func TestNotificationAdminCreate(t *testing.T) {
	_, _, h, _ := notificationFixture(t)
	e := notificationRouter(h, opsSession())

	rec := notificationRequestRecorder(e, http.MethodPost, "/admin/notifications", createNotificationBody(t))
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeAuthEnvelope(t, rec)
	if env.Code != 0 {
		t.Fatalf("code = %d, message = %s", env.Code, env.Message)
	}
	var view notification.View
	if err := json.Unmarshal(env.Result, &view); err != nil {
		t.Fatalf("decode view: %v", err)
	}
	if view.Title != "Release 1.2.0" || view.Status != model.NotificationDraft || view.CreatedBy != 9 {
		t.Fatalf("unexpected view: %+v", view)
	}
}

func TestNotificationAdminPublishConflict(t *testing.T) {
	repo, _, h, _ := notificationFixture(t)
	published := repo.add(model.OpsNotification{
		Title: "Release", Content: "body", Type: model.NotificationInbox,
		Status: model.NotificationPublished, ExpireAt: notificationTestNow.Add(time.Hour),
	})
	e := notificationRouter(h, opsSession())
	if published.ID != 1 {
		t.Fatalf("fixture id = %d", published.ID)
	}

	rec := notificationRequestRecorder(e, http.MethodPost, "/admin/notifications/1/publish", "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40901 {
		t.Fatalf("code = %d, want 40901", env.Code)
	}
}

func TestNotificationAdminListPaginates(t *testing.T) {
	repo, _, h, _ := notificationFixture(t)
	for i := 0; i < 5; i++ {
		repo.add(model.OpsNotification{Title: "item", Content: "body", Type: model.NotificationInbox, Status: model.NotificationDraft, ExpireAt: notificationTestNow.Add(time.Hour)})
	}
	e := notificationRouter(h, opsSession())

	rec := notificationRequestRecorder(e, http.MethodGet, "/admin/notifications?page=2&pageSize=2", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Notifications []notification.View `json:"notifications"`
		Total         int64               `json:"total"`
		Page          int                 `json:"page"`
		PageSize      int                 `json:"pageSize"`
	}
	if err := json.Unmarshal(decodeAuthEnvelope(t, rec).Result, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Total != 5 || len(result.Notifications) != 2 || result.Page != 2 || result.PageSize != 2 {
		t.Fatalf("result = %+v", result)
	}
}

func TestNotificationWebListUnauthorizedEnvelope(t *testing.T) {
	_, _, h, _ := notificationFixture(t)
	e := notificationRouter(h, nil)

	rec := notificationRequestRecorder(e, http.MethodGet, "/notifications", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if env := decodeAuthEnvelope(t, rec); env.Code != 40101 {
		t.Fatalf("code = %d, want 40101", env.Code)
	}
}

func TestNotificationHandlerCheckOrigin(t *testing.T) {
	h := NewNotificationHandler(nil, nil, []string{"https://ops.example.com/"})
	cases := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{name: "configured origin", host: "api.example.com", origin: "https://ops.example.com", want: true},
		{name: "same http host", host: "api.example.com", origin: "http://api.example.com", want: true},
		{name: "same https host", host: "api.example.com", origin: "https://api.example.com", want: true},
		{name: "empty origin", host: "api.example.com", want: true},
		{name: "disallowed origin", host: "api.example.com", origin: "https://evil.example.com", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/notifications/ws", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if got := h.Upgrader.CheckOrigin(req); got != tc.want {
				t.Fatalf("CheckOrigin() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNotificationSocketUnauthorized(t *testing.T) {
	_, _, h, _ := notificationFixture(t)
	e := notificationRouter(h, nil)

	rec := notificationRequestRecorder(e, http.MethodGet, "/notifications/ws", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestNotificationSocketUpgradeErrorDoesNotWriteEnvelope(t *testing.T) {
	_, _, h, _ := notificationFixture(t)
	e := notificationRouter(h, opsSession())

	rec := notificationRequestRecorder(e, http.MethodGet, "/notifications/ws", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"code"`) {
		t.Fatalf("unexpected second response body = %s", rec.Body.String())
	}
}

func TestNotificationWebListReturnsPublished(t *testing.T) {
	repo, cache, h, _ := notificationFixture(t)
	item := repo.add(model.OpsNotification{
		Title: "Release", Content: "body", Type: model.NotificationInbox,
		Status: model.NotificationPublished, PublishedAt: &notificationTestNow,
		ExpireAt: notificationTestNow.Add(time.Hour),
	})
	cache.items = []notification.Published{{
		ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
		PublishedAt: notificationTestNow, ExpireAt: item.ExpireAt,
	}}
	e := notificationRouter(h, opsSession())

	rec := notificationRequestRecorder(e, http.MethodGet, "/notifications", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	env := decodeAuthEnvelope(t, rec)
	var result struct {
		Notifications []notification.Published `json:"notifications"`
		Unread        bool                     `json:"unread"`
	}
	if err := json.Unmarshal(env.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(result.Notifications) != 1 || result.Notifications[0].ID != item.ID {
		t.Fatalf("unexpected notifications: %+v", result.Notifications)
	}
	if !result.Unread {
		t.Fatal("unread = false, want true")
	}
}

func TestNotificationMarkRead(t *testing.T) {
	repo, cache, h, _ := notificationFixture(t)
	item := repo.add(model.OpsNotification{
		Title: "Release", Content: "body", Type: model.NotificationInbox,
		Status: model.NotificationPublished, PublishedAt: &notificationTestNow,
		ExpireAt: notificationTestNow.Add(time.Hour),
	})
	cache.items = []notification.Published{{
		ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
		PublishedAt: notificationTestNow, ExpireAt: item.ExpireAt,
	}}
	e := notificationRouter(h, opsSession())

	readAt := notificationTestNow.Format(time.RFC3339)
	rec := notificationRequestRecorder(e, http.MethodPost, "/notifications/read", `{"readAt":"`+readAt+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !cache.lastRead.Equal(notificationTestNow) {
		t.Fatalf("lastRead = %v", cache.lastRead)
	}

	rec = notificationRequestRecorder(e, http.MethodGet, "/notifications", "")
	var result struct {
		Unread bool `json:"unread"`
	}
	if err := json.Unmarshal(decodeAuthEnvelope(t, rec).Result, &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Unread {
		t.Fatal("unread = true, want false after mark read")
	}
}

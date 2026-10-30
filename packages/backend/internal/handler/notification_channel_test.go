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

	"vibebiz/internal/dao/model"
	"vibebiz/internal/notificationchannel"
)

type fakeChannelCipher struct{}

func (fakeChannelCipher) Encrypt(value string) (string, error) { return "enc:" + value, nil }
func (fakeChannelCipher) Decrypt(value string) (string, error) { return value[4:], nil }

type fakeChannelRepo struct {
	items   map[int64]model.NotificationChannel
	nextID  int64
	status  string
	message string
}

func newFakeChannelRepo() *fakeChannelRepo {
	return &fakeChannelRepo{items: make(map[int64]model.NotificationChannel), nextID: 1}
}

func (f *fakeChannelRepo) List(_ context.Context, userID int64) ([]model.NotificationChannel, error) {
	out := make([]model.NotificationChannel, 0)
	for _, item := range f.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (f *fakeChannelRepo) Get(_ context.Context, userID, id int64) (*model.NotificationChannel, error) {
	item, ok := f.items[id]
	if !ok || item.UserID != userID {
		return nil, notificationchannel.ErrNotFound
	}
	return &item, nil
}

func (f *fakeChannelRepo) Create(_ context.Context, item *model.NotificationChannel) error {
	item.ID = f.nextID
	f.nextID++
	f.items[item.ID] = *item
	return nil
}

func (f *fakeChannelRepo) Update(_ context.Context, item *model.NotificationChannel) error {
	if _, ok := f.items[item.ID]; !ok {
		return notificationchannel.ErrNotFound
	}
	f.items[item.ID] = *item
	return nil
}

func (f *fakeChannelRepo) Delete(_ context.Context, userID, id int64) error {
	item, ok := f.items[id]
	if !ok || item.UserID != userID {
		return notificationchannel.ErrNotFound
	}
	delete(f.items, id)
	return nil
}

func (f *fakeChannelRepo) NameExists(_ context.Context, userID int64, name string, excludeID int64) (bool, error) {
	for _, item := range f.items {
		if item.UserID == userID && item.Name == name && item.ID != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (f *fakeChannelRepo) UpdateTest(_ context.Context, _ int64, id int64, at time.Time, status, message string) error {
	item := f.items[id]
	item.LastTestAt = &at
	item.LastTestStatus = status
	item.LastTestError = message
	f.items[id] = item
	f.status, f.message = status, message
	return nil
}

type fakeChannelSender struct{ err error }

func (f fakeChannelSender) Send(context.Context, notificationchannel.Channel) error { return f.err }

func newChannelHandlerForTest(repo *fakeChannelRepo, sender *fakeChannelSender) (*echo.Echo, *NotificationChannelHandler) {
	service := &notificationchannel.Service{
		Repo: repo, Cipher: fakeChannelCipher{}, Sender: sender,
		Now: func() time.Time { return time.Unix(1700000000, 0).UTC() },
	}
	h := NewNotificationChannelHandler(service)
	e := echo.New()
	session := sessionForTest(3)
	routeWithSession(e, http.MethodGet, "/cp/api/v1/me/notification-channels", h.List, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/me/notification-channels", h.Create, session)
	routeWithSession(e, http.MethodPut, "/cp/api/v1/me/notification-channels/:id", h.Update, session)
	routeWithSession(e, http.MethodDelete, "/cp/api/v1/me/notification-channels/:id", h.Delete, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/me/notification-channels/:id/test", h.Test, session)
	return e, h
}

func channelRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func TestNotificationChannelCRUDAndTest(t *testing.T) {
	repo := newFakeChannelRepo()
	e, _ := newChannelHandlerForTest(repo, &fakeChannelSender{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPost, "/cp/api/v1/me/notification-channels", `{"name":"研发群","type":"webhook","enabled":true,"config":{"url":"https://example.com/hook"}}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Result notificationchannel.Channel `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Result.ID == 0 || created.Result.Config.URL != "https://example.com/hook" {
		t.Fatalf("created = %+v", created.Result)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodGet, "/cp/api/v1/me/notification-channels", ""))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "研发群") {
		t.Fatalf("list status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPost, "/cp/api/v1/me/notification-channels/1/test", ""))
	if rec.Code != http.StatusOK || repo.status != notificationchannel.TestStatusSuccess {
		t.Fatalf("test status = %d repo=%s body=%s", rec.Code, repo.status, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPut, "/cp/api/v1/me/notification-channels/1", `{"name":"研发群2","type":"email","enabled":false,"config":{"to":"a@example.com"}}`))
	if rec.Code != http.StatusOK || repo.items[1].Name != "研发群2" || repo.items[1].Enabled {
		t.Fatalf("update status = %d item=%+v body=%s", rec.Code, repo.items[1], rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodDelete, "/cp/api/v1/me/notification-channels/1", ""))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotificationChannelTestFailure(t *testing.T) {
	repo := newFakeChannelRepo()
	e, _ := newChannelHandlerForTest(repo, &fakeChannelSender{err: errors.New("boom")})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPost, "/cp/api/v1/me/notification-channels", `{"name":"a","type":"webhook","enabled":true,"config":{"url":"https://example.com/hook"}}`))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPost, "/cp/api/v1/me/notification-channels/1/test", ""))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if repo.status != notificationchannel.TestStatusFailed || repo.message == "" {
		t.Fatalf("repo status=%s message=%s", repo.status, repo.message)
	}
}

func TestNotificationChannelRejectsInvalidAndForeign(t *testing.T) {
	repo := newFakeChannelRepo()
	e, _ := newChannelHandlerForTest(repo, &fakeChannelSender{})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodPost, "/cp/api/v1/me/notification-channels", `{"name":"a","type":"webhook","enabled":true,"config":{"url":"http://example.com/hook"}}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d body=%s", rec.Code, rec.Body.String())
	}

	repo.items[9] = model.NotificationChannel{ID: 9, UserID: 99, Name: "other"}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, channelRequest(http.MethodDelete, "/cp/api/v1/me/notification-channels/9", ""))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("foreign status = %d body=%s", rec.Code, rec.Body.String())
	}
}

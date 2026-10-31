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

	"vibebiz/internal/dao/model"
	"vibebiz/internal/fderesource"
)

type fakeFdeStore struct {
	items  map[int64]model.FdeResource
	nextID int64
}

func newFakeFdeStore() *fakeFdeStore {
	return &fakeFdeStore{items: make(map[int64]model.FdeResource), nextID: 1}
}

func (f *fakeFdeStore) List(_ context.Context, enabledOnly bool) ([]model.FdeResource, error) {
	items := make([]model.FdeResource, 0)
	for _, item := range f.items {
		if !enabledOnly || item.Enabled {
			items = append(items, item)
		}
	}
	return items, nil
}

func (f *fakeFdeStore) Get(_ context.Context, id int64) (*model.FdeResource, error) {
	item, ok := f.items[id]
	if !ok {
		return nil, fderesource.ErrNotFound
	}
	return &item, nil
}

func (f *fakeFdeStore) Create(_ context.Context, item *model.FdeResource) error {
	item.ID = f.nextID
	item.CreatedAt = time.Unix(1700000000, 0).UTC()
	item.UpdatedAt = item.CreatedAt
	f.nextID++
	f.items[item.ID] = *item
	return nil
}

func (f *fakeFdeStore) Update(_ context.Context, item *model.FdeResource) error {
	if _, ok := f.items[item.ID]; !ok {
		return fderesource.ErrNotFound
	}
	item.UpdatedAt = time.Unix(1700000001, 0).UTC()
	f.items[item.ID] = *item
	return nil
}

func newFdeHandlerForTest(repo *fakeFdeStore) (*echo.Echo, *FdeResourceHandler) {
	h := NewFdeResourceHandler(fderesource.NewService(repo))
	e := echo.New()
	session := sessionForTest(3)
	routeWithSession(e, http.MethodGet, "/cp/api/v1/admin/fde-resources", h.AdminList, session)
	routeWithSession(e, http.MethodPost, "/cp/api/v1/admin/fde-resources", h.Create, session)
	routeWithSession(e, http.MethodPut, "/cp/api/v1/admin/fde-resources/:id", h.Update, session)
	routeWithSession(e, http.MethodGet, "/cp/api/v1/fde-resources", h.UserList, session)
	return e, h
}

func fdeRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func TestFdeResourceCRUDAndUserVisibility(t *testing.T) {
	repo := newFakeFdeStore()
	e, _ := newFdeHandlerForTest(repo)

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodPost, "/cp/api/v1/admin/fde-resources", `{"name":"张工","contact":"zhang@example.com","skills":["K8s","交付"],"region":"上海","note":"企业交付","enabled":true}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		Result fderesource.Resource `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Result.ID == 0 || len(created.Result.Skills) != 2 {
		t.Fatalf("created = %+v", created.Result)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodGet, "/cp/api/v1/fde-resources", ""))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "张工") {
		t.Fatalf("user list status = %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodPut, "/cp/api/v1/admin/fde-resources/1", `{"name":"张工","contact":"zhang@example.com","skills":["K8s"],"region":"上海","note":"暂停","enabled":false}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", rec.Code, rec.Body.String())
	}
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodGet, "/cp/api/v1/fde-resources", ""))
	if strings.Contains(rec.Body.String(), "张工") {
		t.Fatalf("disabled resource leaked to user: %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodGet, "/cp/api/v1/admin/fde-resources", ""))
	if !strings.Contains(rec.Body.String(), "张工") {
		t.Fatalf("admin list missing disabled resource: %s", rec.Body.String())
	}
}

func TestFdeResourceRejectsInvalid(t *testing.T) {
	e, _ := newFdeHandlerForTest(newFakeFdeStore())
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, fdeRequest(http.MethodPost, "/cp/api/v1/admin/fde-resources", `{"name":" ","contact":""}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
}

package notification

import (
	"context"
	"errors"
	"testing"
	"time"

	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

var testNow = time.Date(2026, 10, 29, 12, 0, 0, 0, time.UTC)

type fakeRepo struct {
	items   map[int64]model.OpsNotification
	nextID  int64
	listErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{items: make(map[int64]model.OpsNotification)}
}

func (r *fakeRepo) add(item model.OpsNotification) model.OpsNotification {
	r.nextID++
	item.ID = r.nextID
	r.items[item.ID] = item
	return item
}

func (r *fakeRepo) List(ctx context.Context, filter dao.NotificationFilter) ([]model.OpsNotification, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
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
	return out, nil
}

func (r *fakeRepo) Get(ctx context.Context, id int64) (*model.OpsNotification, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, dao.ErrNotFound
	}
	return &item, nil
}

func (r *fakeRepo) Create(ctx context.Context, item *model.OpsNotification) error {
	r.nextID++
	item.ID = r.nextID
	r.items[item.ID] = *item
	return nil
}

func (r *fakeRepo) Update(ctx context.Context, item *model.OpsNotification) error {
	if _, ok := r.items[item.ID]; !ok {
		return dao.ErrNotFound
	}
	r.items[item.ID] = *item
	return nil
}

func (r *fakeRepo) UpdateStatus(ctx context.Context, id int64, status string, publishedAt *time.Time) error {
	item, ok := r.items[id]
	if !ok {
		return dao.ErrNotFound
	}
	item.Status = model.NotificationStatus(status)
	item.PublishedAt = publishedAt
	r.items[id] = item
	return nil
}

func (r *fakeRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := r.items[id]; !ok {
		return dao.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *fakeRepo) Published(ctx context.Context, now time.Time) ([]model.OpsNotification, error) {
	out := make([]model.OpsNotification, 0)
	for _, item := range r.items {
		if item.Status == model.NotificationPublished && item.ExpireAt.After(now) {
			out = append(out, item)
		}
	}
	return out, nil
}

type fakeCache struct {
	puts      []model.OpsNotification
	removes   []int64
	items     []Published
	listErr   error
	rebuilds  []model.OpsNotification
	putErr    error
	removeErr error
}

func (c *fakeCache) Put(ctx context.Context, item model.OpsNotification) error {
	if c.putErr != nil {
		return c.putErr
	}
	c.puts = append(c.puts, item)
	return nil
}

func (c *fakeCache) Remove(ctx context.Context, id int64) error {
	if c.removeErr != nil {
		return c.removeErr
	}
	c.removes = append(c.removes, id)
	return nil
}

func (c *fakeCache) List(ctx context.Context) ([]Published, error) {
	if c.listErr != nil {
		return nil, c.listErr
	}
	return c.items, nil
}

func (c *fakeCache) Rebuild(ctx context.Context, items []model.OpsNotification) error {
	c.rebuilds = append(c.rebuilds, items...)
	c.items = make([]Published, 0, len(items))
	for _, item := range items {
		publishedAt := item.CreatedAt
		if item.PublishedAt != nil {
			publishedAt = *item.PublishedAt
		}
		c.items = append(c.items, Published{
			ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
			Version: item.Version, Link: item.Link, PublishedAt: publishedAt, ExpireAt: item.ExpireAt,
		})
	}
	c.listErr = nil
	return nil
}

type fakeHub struct {
	events []Event
	last   Event
}

func (h *fakeHub) Broadcast(event Event) {
	h.last = event
	h.events = append(h.events, event)
}

func newTestServiceWith(repo Repository, cache CacheStore, hub Broadcaster) *Service {
	svc := NewService(repo, cache, hub)
	svc.Now = func() time.Time { return testNow }
	return svc
}

func newTestService(t *testing.T, hub Broadcaster) *Service {
	t.Helper()
	return newTestServiceWith(newFakeRepo(), &fakeCache{}, hub)
}

func draftNotification(now time.Time) model.OpsNotification {
	return model.OpsNotification{
		Title:    "发布",
		Content:  "内容",
		Type:     model.NotificationInbox,
		Status:   model.NotificationDraft,
		ExpireAt: now.Add(24 * time.Hour),
	}
}

func TestServiceCreateValidatesExpiry(t *testing.T) {
	hub := &fakeHub{}
	svc := newTestService(t, hub)
	_, err := svc.Create(context.Background(), CreateInput{
		Title: "发布", Content: "内容", Type: model.NotificationInbox,
		ExpireAt: svc.Now().Add(31 * 24 * time.Hour),
	}, Creator{ID: 1, Name: "admin"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestServiceCreateAccepts30DayBoundary(t *testing.T) {
	hub := &fakeHub{}
	svc := newTestService(t, hub)
	view, err := svc.Create(context.Background(), CreateInput{
		Title: "发布", Content: "内容", Type: model.NotificationInbox,
		ExpireAt: svc.Now().Add(30 * 24 * time.Hour),
	}, Creator{ID: 1, Name: "admin"})
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if view.Status != model.NotificationDraft {
		t.Fatalf("status = %q, want draft", view.Status)
	}
}

func TestServicePublishWritesCacheAndBroadcasts(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	item := repo.add(draftNotification(svc.Now()))
	view, err := svc.Publish(context.Background(), item.ID)
	if err != nil || view.Status != model.NotificationPublished {
		t.Fatalf("view = %+v, err = %v", view, err)
	}
	if len(cache.puts) != 1 || hub.last.Action != "published" {
		t.Fatalf("puts = %d, event = %+v", len(cache.puts), hub.last)
	}
}

func TestServicePublishRejectsRepeat(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	item := repo.add(published)
	if _, err := svc.Publish(context.Background(), item.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if len(cache.puts) != 0 {
		t.Fatalf("puts = %d, want 0", len(cache.puts))
	}
}

func TestServicePublishRejectsExpired(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	item := draftNotification(svc.Now())
	item.ExpireAt = svc.Now().Add(-time.Hour)
	item = repo.add(item)
	if _, err := svc.Publish(context.Background(), item.ID); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
	if len(cache.puts) != 0 {
		t.Fatalf("puts = %d, want 0", len(cache.puts))
	}
}

func TestServiceUpdateRejectsPublished(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	item := repo.add(published)
	_, err := svc.Update(context.Background(), item.ID, CreateInput{
		Title: "改标题", Content: "改内容", Type: model.NotificationInbox,
		ExpireAt: svc.Now().Add(time.Hour),
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestServiceUnpublishRemovesCache(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	item := repo.add(published)
	view, err := svc.Unpublish(context.Background(), item.ID)
	if err != nil || view.Status != model.NotificationUnpublished {
		t.Fatalf("view = %+v, err = %v", view, err)
	}
	if len(cache.removes) != 1 || cache.removes[0] != item.ID || hub.last.Action != "unpublished" {
		t.Fatalf("removes = %v, event = %+v", cache.removes, hub.last)
	}
}

func TestServiceDeletePublishedRemovesCache(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	item := repo.add(published)
	if err := svc.Delete(context.Background(), item.ID); err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(cache.removes) != 1 || cache.removes[0] != item.ID || hub.last.Action != "deleted" {
		t.Fatalf("removes = %v, event = %+v", cache.removes, hub.last)
	}
}

func TestServicePublishedRebuildsMissingIndex(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{listErr: ErrIndexMissing}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	published.PublishedAt = &testNow
	published = repo.add(published)

	items, err := svc.Published(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(cache.rebuilds) != 1 {
		t.Fatalf("rebuilds = %d, want 1", len(cache.rebuilds))
	}
	if len(items) != 1 || items[0].ID != published.ID {
		t.Fatalf("items = %+v", items)
	}
}

func TestServicePublishedNoRowsReturnsEmpty(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{listErr: ErrIndexMissing}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)

	items, err := svc.Published(context.Background())
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if items == nil || len(items) != 0 {
		t.Fatalf("items = %#v, want empty non-nil", items)
	}
	if len(cache.rebuilds) != 0 {
		t.Fatalf("rebuilds = %d, want 0", len(cache.rebuilds))
	}
}

func TestServicePublishRestoresCacheWhenDBFails(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	item := repo.add(draftNotification(svc.Now()))
	repo.listErr = nil
	svc.Repo = failingStatusRepo{Repository: repo}

	if _, err := svc.Publish(context.Background(), item.ID); err == nil {
		t.Fatal("err = nil, want DB error")
	}
	if len(cache.removes) != 1 || cache.removes[0] != item.ID {
		t.Fatalf("removes = %v, want id %d", cache.removes, item.ID)
	}
}

func TestServicePublishReturnsCacheUnavailable(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{putErr: errors.New("redis down")}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	item := repo.add(draftNotification(svc.Now()))

	if _, err := svc.Publish(context.Background(), item.ID); !errors.Is(err, ErrCacheUnavailable) {
		t.Fatalf("err = %v, want ErrCacheUnavailable", err)
	}
	if len(hub.events) != 0 {
		t.Fatalf("events = %v, want none", hub.events)
	}
}

func TestServiceViewDerivesExpiredStatus(t *testing.T) {
	repo := newFakeRepo()
	cache := &fakeCache{}
	hub := &fakeHub{}
	svc := newTestServiceWith(repo, cache, hub)
	published := draftNotification(svc.Now())
	published.Status = model.NotificationPublished
	published.ExpireAt = svc.Now().Add(-time.Hour)
	repo.add(published)

	items, err := svc.List(context.Background(), dao.NotificationFilter{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(items) != 1 || items[0].Status != model.NotificationStatus("expired") {
		t.Fatalf("items = %+v, want status expired", items)
	}
}

type failingStatusRepo struct {
	Repository
}

func (failingStatusRepo) UpdateStatus(ctx context.Context, id int64, status string, publishedAt *time.Time) error {
	return errors.New("db down")
}

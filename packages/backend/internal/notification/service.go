package notification

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
)

var (
	ErrInvalid          = errors.New("invalid notification")
	ErrConflict         = errors.New("notification conflict")
	ErrNotFound         = dao.ErrNotFound
	ErrCacheUnavailable = errors.New("notification cache unavailable")
)

const maxRetention = 30 * 24 * time.Hour

type Creator struct {
	ID   int64
	Name string
}

type Event struct {
	Type   string `json:"type"`
	Action string `json:"action"`
	ID     int64  `json:"id"`
}

type Broadcaster interface {
	Publish(ctx context.Context, event Event) error
}

type Repository interface {
	List(ctx context.Context, filter dao.NotificationFilter, offset, limit int) ([]model.OpsNotification, int64, error)
	Get(ctx context.Context, id int64) (*model.OpsNotification, error)
	Create(ctx context.Context, item *model.OpsNotification) error
	Update(ctx context.Context, item *model.OpsNotification) error
	UpdateStatus(ctx context.Context, id int64, status string, publishedAt *time.Time) error
	Delete(ctx context.Context, id int64) error
	Published(ctx context.Context, now time.Time) ([]model.OpsNotification, error)
}

type CacheStore interface {
	Put(ctx context.Context, item model.OpsNotification) error
	Remove(ctx context.Context, id int64) error
	List(ctx context.Context) ([]Published, error)
	Rebuild(ctx context.Context, items []model.OpsNotification) error
	LastRead(ctx context.Context, userID int64) (time.Time, error)
	MarkRead(ctx context.Context, userID int64, readAt time.Time) error
}

type Service struct {
	Repo  Repository
	Cache CacheStore
	Hub   Broadcaster
	Now   func() time.Time
}

func NewService(repo Repository, cache CacheStore, hub Broadcaster) *Service {
	return &Service{Repo: repo, Cache: cache, Hub: hub, Now: time.Now}
}

type CreateInput struct {
	Title           string
	Content         string
	Type            model.NotificationType
	Version         string
	Link            string
	ExpireAt        time.Time
	RecipientUserID int64
}

type View struct {
	ID            int64                    `json:"id"`
	Title         string                   `json:"title"`
	Content       string                   `json:"content"`
	Type          model.NotificationType   `json:"type"`
	Version       string                   `json:"version"`
	Link          string                   `json:"link"`
	Status        model.NotificationStatus `json:"status"`
	ExpireAt      time.Time                `json:"expireAt"`
	PublishedAt   *time.Time               `json:"publishedAt,omitempty"`
	CreatedBy     int64                    `json:"createdBy"`
	CreatedByName string                   `json:"createdByName"`
	CreatedAt     time.Time                `json:"createdAt"`
	UpdatedAt     time.Time                `json:"updatedAt"`
}

type ViewPage struct {
	Items    []View `json:"notifications"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

func (s *Service) List(ctx context.Context, filter dao.NotificationFilter, page, pageSize int) (ViewPage, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	items, total, err := s.Repo.List(ctx, filter, (page-1)*pageSize, pageSize)
	if err != nil {
		return ViewPage{}, err
	}
	out := make([]View, 0, len(items))
	for _, item := range items {
		out = append(out, s.view(item))
	}
	return ViewPage{Items: out, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) Create(ctx context.Context, input CreateInput, creator Creator) (View, error) {
	item, err := s.build(input)
	if err != nil {
		return View{}, err
	}
	item.Status = model.NotificationDraft
	item.CreatedBy = creator.ID
	item.CreatedByName = creator.Name
	if err := s.Repo.Create(ctx, &item); err != nil {
		return View{}, err
	}
	return s.view(item), nil
}

func (s *Service) Update(ctx context.Context, id int64, input CreateInput) (View, error) {
	current, err := s.Repo.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if current.Status == model.NotificationPublished {
		return View{}, ErrConflict
	}
	item, err := s.build(input)
	if err != nil {
		return View{}, err
	}
	item.ID = current.ID
	if err := s.Repo.Update(ctx, &item); err != nil {
		return View{}, err
	}
	stored, err := s.Repo.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	return s.view(*stored), nil
}

func (s *Service) Publish(ctx context.Context, id int64) (View, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if item.Status == model.NotificationPublished {
		return View{}, ErrConflict
	}
	now := s.Now()
	if !item.ExpireAt.After(now) {
		return View{}, ErrConflict
	}
	if item.ExpireAt.After(now.Add(maxRetention)) {
		return View{}, ErrInvalid
	}
	publishedAt := now
	item.Status = model.NotificationPublished
	item.PublishedAt = &publishedAt
	if err := s.Cache.Put(ctx, *item); err != nil {
		return View{}, ErrCacheUnavailable
	}
	if err := s.Repo.UpdateStatus(ctx, id, string(model.NotificationPublished), &publishedAt); err != nil {
		_ = s.removeCache(ctx, id, "publish rollback")
		return View{}, err
	}
	s.broadcast(ctx, "published", id)
	stored, err := s.Repo.Get(ctx, id)
	if err != nil {
		return s.view(*item), nil
	}
	return s.view(*stored), nil
}

func (s *Service) Unpublish(ctx context.Context, id int64) (View, error) {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return View{}, err
	}
	if item.Status != model.NotificationPublished {
		return View{}, ErrConflict
	}
	if err := s.Repo.UpdateStatus(ctx, id, string(model.NotificationUnpublished), nil); err != nil {
		return View{}, err
	}
	if err := s.removeCache(ctx, id, "unpublish"); err != nil {
		return View{}, ErrCacheUnavailable
	}
	s.broadcast(ctx, "unpublished", id)
	stored, err := s.Repo.Get(ctx, id)
	if err != nil {
		item.Status = model.NotificationUnpublished
		return s.view(*item), nil
	}
	return s.view(*stored), nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if _, err := s.Repo.Get(ctx, id); err != nil {
		return err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	if err := s.removeCache(ctx, id, "delete"); err != nil {
		return ErrCacheUnavailable
	}
	s.broadcast(ctx, "deleted", id)
	return nil
}

type PublishedResult struct {
	Notifications []Published `json:"notifications"`
	Unread        bool        `json:"unread"`
}

func (s *Service) Published(ctx context.Context, userID int64) (PublishedResult, error) {
	stored, err := s.Repo.Published(ctx, s.Now())
	if err != nil {
		return PublishedResult{}, err
	}
	dbItems := make([]Published, 0, len(stored))
	for _, item := range stored {
		dbItems = append(dbItems, publishedFromModel(item))
	}
	cached, cacheErr := s.Cache.List(ctx)
	if cacheErr != nil {
		if !errors.Is(cacheErr, ErrIndexMissing) {
			return PublishedResult{}, ErrCacheUnavailable
		}
		if len(dbItems) > 0 {
			if err := s.Cache.Rebuild(ctx, stored); err != nil {
				return PublishedResult{}, ErrCacheUnavailable
			}
		}
	} else if !samePublished(cached, dbItems) {
		if err := s.Cache.Rebuild(ctx, stored); err != nil {
			return PublishedResult{}, ErrCacheUnavailable
		}
	}
	lastRead, err := s.Cache.LastRead(ctx, userID)
	if err != nil {
		return PublishedResult{}, ErrCacheUnavailable
	}
	visible := visibleForUser(dbItems, userID)
	return PublishedResult{Notifications: visible, Unread: hasUnread(visible, lastRead)}, nil
}

func visibleForUser(items []Published, userID int64) []Published {
	out := make([]Published, 0, len(items))
	for _, item := range items {
		if item.RecipientUserID == 0 || item.RecipientUserID == userID {
			out = append(out, item)
		}
	}
	return out
}

func (s *Service) MarkRead(ctx context.Context, userID int64, readAt time.Time) error {
	now := s.Now()
	if readAt.IsZero() || readAt.After(now) {
		readAt = now
	}
	if err := s.Cache.MarkRead(ctx, userID, readAt); err != nil {
		return ErrCacheUnavailable
	}
	return nil
}

func hasUnread(items []Published, lastRead time.Time) bool {
	if lastRead.IsZero() {
		return len(items) > 0
	}
	for _, item := range items {
		if item.PublishedAt.After(lastRead) {
			return true
		}
	}
	return false
}

func samePublished(cached, db []Published) bool {
	if len(cached) != len(db) {
		return false
	}
	byID := make(map[int64]Published, len(db))
	for _, item := range db {
		byID[item.ID] = item
	}
	for _, item := range cached {
		expected, ok := byID[item.ID]
		if !ok {
			return false
		}
		if expected.PublishedAt.UnixMilli() != item.PublishedAt.UnixMilli() {
			return false
		}
		if expected.ExpireAt.UnixMilli() != item.ExpireAt.UnixMilli() {
			return false
		}
	}
	return true
}

func (s *Service) removeCache(ctx context.Context, id int64, action string) error {
	if err := s.Cache.Remove(ctx, id); err != nil {
		slog.Default().Error("notification cache remove failed",
			"notification_id", id,
			"action", action,
			"error", err,
		)
		return err
	}
	return nil
}

func (s *Service) build(input CreateInput) (model.OpsNotification, error) {
	title := strings.TrimSpace(input.Title)
	content := strings.TrimSpace(input.Content)
	version := strings.TrimSpace(input.Version)
	link := strings.TrimSpace(input.Link)
	if title == "" || len([]rune(title)) > 200 {
		return model.OpsNotification{}, ErrInvalid
	}
	if content == "" || len(content) > 512*1024 {
		return model.OpsNotification{}, ErrInvalid
	}
	if input.Type != model.NotificationVersionRelease && input.Type != model.NotificationInbox {
		return model.OpsNotification{}, ErrInvalid
	}
	if input.Type == model.NotificationInbox {
		version = ""
	}
	if len([]rune(version)) > 64 {
		return model.OpsNotification{}, ErrInvalid
	}
	if link != "" {
		if len(link) > 1024 || (!strings.HasPrefix(link, "http://") && !strings.HasPrefix(link, "https://")) {
			return model.OpsNotification{}, ErrInvalid
		}
		parsed, err := url.Parse(link)
		if err != nil || parsed.Host == "" || parsed.Opaque != "" {
			return model.OpsNotification{}, ErrInvalid
		}
	}
	now := s.Now()
	if !input.ExpireAt.After(now) || input.ExpireAt.After(now.Add(maxRetention)) {
		return model.OpsNotification{}, ErrInvalid
	}
	if input.RecipientUserID < 0 {
		return model.OpsNotification{}, ErrInvalid
	}
	return model.OpsNotification{
		Title: title, Content: content, Type: input.Type, Version: version,
		Link: link, ExpireAt: input.ExpireAt, RecipientUserID: input.RecipientUserID,
	}, nil
}

func (s *Service) view(item model.OpsNotification) View {
	status := item.Status
	if item.Status == model.NotificationPublished && !item.ExpireAt.After(s.Now()) {
		status = model.NotificationStatus("expired")
	}
	return View{
		ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
		Version: item.Version, Link: item.Link, Status: status,
		ExpireAt: item.ExpireAt, PublishedAt: item.PublishedAt,
		CreatedBy: item.CreatedBy, CreatedByName: item.CreatedByName,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (s *Service) broadcast(ctx context.Context, action string, id int64) {
	if s.Hub == nil {
		return
	}
	if err := s.Hub.Publish(ctx, Event{Type: "notifications.changed", Action: action, ID: id}); err != nil {
		slog.Default().Error("notification event publish failed",
			"notification_id", id,
			"action", action,
			"error", err,
		)
	}
}

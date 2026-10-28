package notification

import (
	"context"
	"errors"
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
	Broadcast(event Event)
}

type Repository interface {
	List(ctx context.Context, filter dao.NotificationFilter) ([]model.OpsNotification, error)
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
	Title    string
	Content  string
	Type     model.NotificationType
	Version  string
	Link     string
	ExpireAt time.Time
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

func (s *Service) List(ctx context.Context, filter dao.NotificationFilter) ([]View, error) {
	items, err := s.Repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]View, 0, len(items))
	for _, item := range items {
		out = append(out, s.view(item))
	}
	return out, nil
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
	item.Status = current.Status
	item.CreatedBy = current.CreatedBy
	item.CreatedByName = current.CreatedByName
	item.CreatedAt = current.CreatedAt
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
		_ = s.Cache.Remove(ctx, id)
		return View{}, err
	}
	s.broadcast("published", id)
	return s.view(*item), nil
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
	if err := s.Cache.Remove(ctx, id); err != nil {
		return View{}, ErrCacheUnavailable
	}
	s.broadcast("unpublished", id)
	item.Status = model.NotificationUnpublished
	return s.view(*item), nil
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	item, err := s.Repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.Repo.Delete(ctx, id); err != nil {
		return err
	}
	if item.Status == model.NotificationPublished {
		if err := s.Cache.Remove(ctx, id); err != nil {
			return ErrCacheUnavailable
		}
	}
	s.broadcast("deleted", id)
	return nil
}

func (s *Service) Published(ctx context.Context) ([]Published, error) {
	items, err := s.Cache.List(ctx)
	if err == nil {
		return items, nil
	}
	if !errors.Is(err, ErrIndexMissing) {
		return nil, ErrCacheUnavailable
	}
	now := s.Now()
	stored, err := s.Repo.Published(ctx, now)
	if err != nil {
		return nil, err
	}
	if len(stored) == 0 {
		return []Published{}, nil
	}
	if err := s.Cache.Rebuild(ctx, stored); err != nil {
		return nil, ErrCacheUnavailable
	}
	return s.Cache.List(ctx)
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
		parsed, err := url.Parse(link)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || len(link) > 1024 {
			return model.OpsNotification{}, ErrInvalid
		}
	}
	now := s.Now()
	if !input.ExpireAt.After(now) || input.ExpireAt.After(now.Add(maxRetention)) {
		return model.OpsNotification{}, ErrInvalid
	}
	return model.OpsNotification{
		Title: title, Content: content, Type: input.Type, Version: version,
		Link: link, ExpireAt: input.ExpireAt,
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

func (s *Service) broadcast(action string, id int64) {
	if s.Hub != nil {
		s.Hub.Broadcast(Event{Type: "notifications.changed", Action: action, ID: id})
	}
}

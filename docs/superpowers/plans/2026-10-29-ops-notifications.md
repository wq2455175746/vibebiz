# Ops 消息通知与 Web 实时提醒 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 ops-web 可管理运营消息，web 铃铛按类型展示已发布消息，并在发布/取消发布/删除时通过 WebSocket 静默刷新。

**Architecture:** CP 新增 `internal/notification` 模块：MySQL 为事实源，Redis 缓存已发布消息，进程内 Hub 广播变更事件；ops-web 走 `RequireOps` 管理接口，web 走 `RequireUser` 读取接口与 WS。前端分别新增消息管理页和铃铛弹窗。

**Tech Stack:** Go + Echo + GORM/MySQL + go-redis + gorilla/websocket；Vue 3 + TypeScript + PrimeVue + Pinia + marked + DOMPurify；goose 迁移；sqlmock / miniredis / vitest。

**Spec:** `docs/superpowers/specs/2026-10-29-ops-notifications-design.md`

## Global Constraints

- 工作树：`/Users/haoguangwang/DOC/workspace/ai/vibebiz-ops-notifications`，分支 `feat/ops-notifications`；不要修改 `dev`。
- 统一响应 `{ code, message, result }`，成功 `code=0`；错误使用 `response.Fail`。
- 业务前缀：CP `/cp/api/v1`；admin `/cp/api/v1/admin/*`；DP 不注册通知路由。
- 消息停留时间上限 30 天：`now < expireAt <= now+30d`。
- 已发布消息不可编辑；发布、取消发布、删除必须同步 Redis 与广播。
- 所有 Markdown 渲染必须经 `DOMPurify.sanitize`。
- 颜色仅黑 / 白 / 灰 / 蓝；不写无意义注释。
- 每个任务结束运行对应测试并提交。

---

### Task 1: 数据库迁移、模型与 Repository

**Files:**
- Create: `packages/backend/migrations/00008_ops_notification.sql`
- Create: `packages/backend/internal/dao/model/notification.go`
- Create: `packages/backend/internal/dao/notification_repository.go`
- Test: `packages/backend/internal/dao/notification_repository_test.go`

**Interfaces:**
- Consumes: `dao.newMockDB(t)`、`dao.ErrNotFound`、`model.Base`
- Produces:
  - `model.NotificationType`、`model.NotificationStatus` 常量
  - `model.OpsNotification`，字段 `Base`、`Title`、`Content`、`Type`、`Version`、`Link`、`Status`、`ExpireAt`、`PublishedAt`、`CreatedBy`、`CreatedByName`
  - `dao.NotificationFilter{Type, Status string}`
  - `dao.NotificationRepository` 及方法 `List(ctx, filter)`、`Get(ctx, id)`、`Create(ctx, *model.OpsNotification)`、`Update(ctx, *model.OpsNotification)`、`UpdateStatus(ctx, id int64, status string, publishedAt *time.Time)`、`Delete(ctx, id int64)`、`Published(ctx, now time.Time)`

- [ ] **Step 1: 写迁移**

```sql
-- ops notifications: version releases and inbox announcements

-- +goose Up

CREATE TABLE `ops_notification` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `title` varchar(200) NOT NULL COMMENT '消息标题',
  `content` mediumtext NOT NULL COMMENT 'Markdown 正文',
  `type` varchar(32) NOT NULL COMMENT '消息类型：version_release/inbox',
  `version` varchar(64) NOT NULL DEFAULT '' COMMENT '版本号',
  `link` varchar(1024) NOT NULL DEFAULT '' COMMENT '跳转链接',
  `status` varchar(24) NOT NULL DEFAULT 'draft' COMMENT '状态：draft/published/unpublished',
  `expire_at` datetime NOT NULL COMMENT '消息过期时间',
  `published_at` datetime DEFAULT NULL COMMENT '最近发布时间',
  `created_by` bigint UNSIGNED NOT NULL COMMENT '创建人 ops_user.id',
  `created_by_name` varchar(64) NOT NULL COMMENT '创建人用户名快照',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  KEY `idx_ops_notification_status` (`status`, `deleted_at`),
  KEY `idx_ops_notification_type` (`type`, `status`, `deleted_at`),
  KEY `idx_ops_notification_expire` (`expire_at`),
  KEY `idx_ops_notification_published` (`published_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='运营消息通知表';

-- +goose Down

DROP TABLE IF EXISTS `ops_notification`;
```

- [ ] **Step 2: 写模型**

```go
package model

import "time"

type NotificationType string

const (
	NotificationVersionRelease NotificationType = "version_release"
	NotificationInbox          NotificationType = "inbox"
)

type NotificationStatus string

const (
	NotificationDraft       NotificationStatus = "draft"
	NotificationPublished   NotificationStatus = "published"
	NotificationUnpublished NotificationStatus = "unpublished"
)

type OpsNotification struct {
	Base
	Title         string             `gorm:"column:title" json:"title"`
	Content       string             `gorm:"column:content" json:"content"`
	Type          NotificationType   `gorm:"column:type" json:"type"`
	Version       string             `gorm:"column:version" json:"version"`
	Link          string             `gorm:"column:link" json:"link"`
	Status        NotificationStatus `gorm:"column:status" json:"status"`
	ExpireAt      time.Time          `gorm:"column:expire_at" json:"expireAt"`
	PublishedAt   *time.Time         `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CreatedBy     int64              `gorm:"column:created_by" json:"createdBy"`
	CreatedByName string             `gorm:"column:created_by_name" json:"createdByName"`
}

func (OpsNotification) TableName() string { return "ops_notification" }
```

- [ ] **Step 3: 写 Repository**

```go
package dao

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

type NotificationFilter struct {
	Type   string
	Status string
}

type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) List(ctx context.Context, filter NotificationFilter) ([]model.OpsNotification, error) {
	query := r.db.WithContext(ctx).Model(&model.OpsNotification{})
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
	}
	items := make([]model.OpsNotification, 0)
	if err := query.Order("created_at DESC, id DESC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	return items, nil
}

func (r *NotificationRepository) Get(ctx context.Context, id int64) (*model.OpsNotification, error) {
	var item model.OpsNotification
	if err := r.db.WithContext(ctx).First(&item, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get notification: %w", err)
	}
	return &item, nil
}

func (r *NotificationRepository) Create(ctx context.Context, item *model.OpsNotification) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (r *NotificationRepository) Update(ctx context.Context, item *model.OpsNotification) error {
	result := r.db.WithContext(ctx).Model(&model.OpsNotification{}).
		Where("id = ?", item.ID).
		Updates(map[string]any{
			"title": item.Title, "content": item.Content, "type": item.Type,
			"version": item.Version, "link": item.Link, "expire_at": item.ExpireAt,
		})
	if result.Error != nil {
		return fmt.Errorf("update notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) UpdateStatus(ctx context.Context, id int64, status string, publishedAt *time.Time) error {
	updates := map[string]any{"status": status}
	if publishedAt != nil {
		updates["published_at"] = *publishedAt
	}
	result := r.db.WithContext(ctx).Model(&model.OpsNotification{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update notification status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) Delete(ctx context.Context, id int64) error {
	result := r.db.WithContext(ctx).Delete(&model.OpsNotification{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete notification: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *NotificationRepository) Published(ctx context.Context, now time.Time) ([]model.OpsNotification, error) {
	items := make([]model.OpsNotification, 0)
	err := r.db.WithContext(ctx).
		Where("status = ? AND expire_at > ?", model.NotificationPublished, now).
		Order("published_at DESC, id DESC").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list published notifications: %w", err)
	}
	return items, nil
}
```

- [ ] **Step 4: 写 DAO 测试**

至少覆盖：

```go
func TestNotificationRepositoryListFilters(t *testing.T)
func TestNotificationRepositoryGetNotFound(t *testing.T)
func TestNotificationRepositoryUpdateStatus(t *testing.T)
func TestNotificationRepositoryPublished(t *testing.T)
```

使用 `newMockDB(t)`。断言 SQL 包含 `type = ?` / `status = ?` / `expire_at > ?`，并验证 `ErrNotFound`。

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && go test ./internal/dao/...`
Expected: PASS

- [ ] **Step 6: 提交**

```bash
git add packages/backend/migrations/00008_ops_notification.sql packages/backend/internal/dao/model/notification.go packages/backend/internal/dao/notification_repository.go packages/backend/internal/dao/notification_repository_test.go
git commit -m "feat: add ops notification storage"
```

---

### Task 2: Redis 发布缓存

**Files:**
- Create: `packages/backend/internal/notification/cache.go`
- Test: `packages/backend/internal/notification/cache_test.go`

**Interfaces:**
- Consumes: `*redis.Client`、`model.OpsNotification`
- Produces:
  - `notification.Published{ID,Title,Content,Type,Version,Link,PublishedAt,ExpireAt}`
  - `notification.ErrIndexMissing`
  - `notification.Cache` 及 `NewCache(client *redis.Client, prefix string) *Cache`
  - `Put(ctx, item model.OpsNotification) error`
  - `Remove(ctx, id int64) error`
  - `List(ctx) ([]Published, error)`
  - `Rebuild(ctx, items []model.OpsNotification) error`

- [ ] **Step 1: 写失败测试**

使用 miniredis：

```go
func TestCachePutListAndTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cache := NewCache(client, "vibebiz")
	now := time.Now().UTC().Truncate(time.Second)

	item := model.OpsNotification{
		Base: model.Base{ID: 12, CreatedAt: now},
		Title: "v1.2.0 发布", Content: "## 更新", Type: model.NotificationVersionRelease,
		Version: "v1.2.0", Link: "https://help.atoms.dev/zh",
		Status: model.NotificationPublished, ExpireAt: now.Add(7 * 24 * time.Hour),
		PublishedAt: &now,
	}
	if err := cache.Put(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	list, err := cache.List(context.Background())
	if err != nil || len(list) != 1 || list[0].ID != 12 {
		t.Fatalf("list = %+v, err = %v", list, err)
	}
	ttl := mr.TTL("vibebiz:notification:item:12")
	if ttl <= 0 || ttl > 7*24*time.Hour {
		t.Fatalf("ttl = %v", ttl)
	}
}
```

另写：

```go
func TestCacheExpiredItemIsFiltered(t *testing.T) // mr.FastForward + List 返回 0
func TestCacheRemove(t *testing.T)                // Remove 后 List 为空
func TestCacheListReportsMissingIndex(t *testing.T) // 空库返回 ErrIndexMissing
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/backend && go test ./internal/notification/...`
Expected: FAIL（`NewCache` 未定义）

- [ ] **Step 3: 实现缓存**

```go
package notification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"vibebiz/internal/dao/model"
)

var ErrIndexMissing = errors.New("notification index missing")

type Published struct {
	ID          int64                 `json:"id"`
	Title       string                `json:"title"`
	Content     string                `json:"content"`
	Type        model.NotificationType `json:"type"`
	Version     string                `json:"version"`
	Link        string                `json:"link"`
	PublishedAt time.Time             `json:"publishedAt"`
	ExpireAt    time.Time             `json:"expireAt"`
}

type Cache struct {
	client *redis.Client
	index  string
	prefix string
}

func NewCache(client *redis.Client, prefix string) *Cache {
	return &Cache{
		client: client,
		index:  prefix + ":notification:index",
		prefix: prefix + ":notification:item:",
	}
}

func (c *Cache) itemKey(id int64) string {
	return fmt.Sprintf("%s%d", c.prefix, id)
}

func (c *Cache) Put(ctx context.Context, item model.OpsNotification) error {
	publishedAt := item.CreatedAt
	if item.PublishedAt != nil {
		publishedAt = *item.PublishedAt
	}
	payload := Published{
		ID: item.ID, Title: item.Title, Content: item.Content, Type: item.Type,
		Version: item.Version, Link: item.Link, PublishedAt: publishedAt, ExpireAt: item.ExpireAt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	pipe := c.client.TxPipeline()
	pipe.Set(ctx, c.itemKey(item.ID), data, 0)
	pipe.ExpireAt(ctx, c.itemKey(item.ID), item.ExpireAt)
	pipe.ZAdd(ctx, c.index, redis.Z{Score: float64(publishedAt.UnixMilli()), Member: item.ID})
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("cache notification: %w", err)
	}
	return nil
}

func (c *Cache) Remove(ctx context.Context, id int64) error {
	pipe := c.client.TxPipeline()
	pipe.Del(ctx, c.itemKey(id))
	pipe.ZRem(ctx, c.index, id)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("remove notification cache: %w", err)
	}
	return nil
}

func (c *Cache) List(ctx context.Context) ([]Published, error) {
	exists, err := c.client.Exists(ctx, c.index).Result()
	if err != nil {
		return nil, fmt.Errorf("check notification index: %w", err)
	}
	if exists == 0 {
		return nil, ErrIndexMissing
	}
	ids, err := c.client.ZRevRange(ctx, c.index, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("read notification index: %w", err)
	}
	if len(ids) == 0 {
		return []Published{}, nil
	}
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		var numeric int64
		if _, err := fmt.Sscan(id, &numeric); err != nil {
			continue
		}
		keys = append(keys, c.itemKey(numeric))
	}
	values, err := c.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, fmt.Errorf("read notification items: %w", err)
	}
	out := make([]Published, 0, len(values))
	for _, value := range values {
		raw, ok := value.(string)
		if !ok {
			continue
		}
		var item Published
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (c *Cache) Rebuild(ctx context.Context, items []model.OpsNotification) error {
	if err := c.client.Del(ctx, c.index).Err(); err != nil {
		return fmt.Errorf("reset notification index: %w", err)
	}
	for _, item := range items {
		if err := c.Put(ctx, item); err != nil {
			return err
		}
	}
	return nil
}
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/notification/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/notification/cache.go packages/backend/internal/notification/cache_test.go
git commit -m "feat: add notification redis cache"
```

---

### Task 3: 通知 Service（校验、状态机、发布编排）

**Files:**
- Create: `packages/backend/internal/notification/service.go`
- Test: `packages/backend/internal/notification/service_test.go`

**Interfaces:**
- Consumes: Task 1 `NotificationRepository` 方法、Task 2 `Cache`
- Produces:
  - `notification.Repository` 接口
  - `notification.CacheStore` 接口
  - `notification.Broadcaster` 接口
  - `notification.Event{Type, Action string; ID int64}`
  - `notification.Service`、`NewService(repo Repository, cache CacheStore, hub Broadcaster) *Service`
  - `notification.CreateInput{Title,Content string; Type model.NotificationType; Version,Link string; ExpireAt time.Time}`
  - `notification.View`（API 返回对象，含派生 `Status`）
  - `Service.List(ctx, filter)`, `Service.Create(ctx, input, creator)`, `Service.Update(ctx, id, input)`, `Service.Publish(ctx, id)`, `Service.Unpublish(ctx, id)`, `Service.Delete(ctx, id)`, `Service.Published(ctx)`
  - 错误：`ErrInvalid`、`ErrConflict`、`ErrNotFound`、`ErrCacheUnavailable`

- [ ] **Step 1: 写失败测试**

```go
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

func TestServicePublishRejectsExpired(t *testing.T)
func TestServiceUpdateRejectsPublished(t *testing.T)
func TestServiceUnpublishRemovesCache(t *testing.T)
func TestServiceDeletePublishedRemovesCache(t *testing.T)
func TestServicePublishedRebuildsMissingIndex(t *testing.T)
```

- [ ] **Step 2: 运行确认失败**

Run: `cd packages/backend && go test ./internal/notification/ -run TestService`
Expected: FAIL（`Service` 未定义）

- [ ] **Step 3: 实现 Service**

关键代码：

```go
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
	ID            int64                  `json:"id"`
	Title         string                 `json:"title"`
	Content       string                 `json:"content"`
	Type          model.NotificationType `json:"type"`
	Version       string                 `json:"version"`
	Link          string                 `json:"link"`
	Status        string                 `json:"status"`
	ExpireAt      time.Time              `json:"expireAt"`
	PublishedAt   *time.Time             `json:"publishedAt,omitempty"`
	CreatedBy     int64                  `json:"createdBy"`
	CreatedByName string                 `json:"createdByName"`
	CreatedAt     time.Time              `json:"createdAt"`
	UpdatedAt     time.Time              `json:"updatedAt"`
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
	item, err := s.build(ctx, input)
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
	item, err := s.build(ctx, input)
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

func (s *Service) build(ctx context.Context, input CreateInput) (model.OpsNotification, error) {
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
	status := string(item.Status)
	if item.Status == model.NotificationPublished && !item.ExpireAt.After(s.Now()) {
		status = "expired"
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
```

- [ ] **Step 4: 运行测试**

Run: `cd packages/backend && go test ./internal/notification/...`
Expected: PASS

- [ ] **Step 5: 提交**

```bash
git add packages/backend/internal/notification/service.go packages/backend/internal/notification/service_test.go
git commit -m "feat: add notification service"
```

---

### Task 4: WebSocket Hub、HTTP Handler、路由与 CP 装配

**Files:**
- Create: `packages/backend/internal/notification/hub.go`
- Test: `packages/backend/internal/notification/hub_test.go`
- Create: `packages/backend/internal/handler/notification.go`
- Test: `packages/backend/internal/handler/notification_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/go.mod`

**Interfaces:**
- Consumes: Task 3 `Service`、`auth.SessionFrom`、`response`
- Produces:
  - `notification.NewHub() *Hub`、`Hub.Handle(conn *websocket.Conn)`、`Hub.Broadcast(Event)`、`Hub.Close()`
  - `handler.NewNotificationHandler(service *notification.Service, hub *notification.Hub, origins []string) *NotificationHandler`
  - `NotificationHandler.AdminList/AdminCreate/AdminUpdate/AdminPublish/AdminUnpublish/AdminDelete/WebList/Socket`
  - 路由：`/cp/api/v1/admin/notifications`、`/cp/api/v1/notifications`、`/cp/api/v1/notifications/ws`

- [ ] **Step 1: 添加依赖**

Run: `cd packages/backend && go get github.com/gorilla/websocket@v1.5.3`
Expected: `go.mod`、`go.sum` 更新。

- [ ] **Step 2: 写 Hub 失败测试**

```go
func TestHubBroadcastToClient(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		hub.Handle(conn)
	}))
	defer server.Close()

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	want := Event{Type: "notifications.changed", Action: "published", ID: 12}
	deadline := time.Now().Add(3 * time.Second)
	for {
		hub.Broadcast(want)
		_ = conn.SetReadDeadline(deadline)
		var got Event
		if err := conn.ReadJSON(&got); err == nil {
			if got != want {
				t.Fatalf("event = %+v, want %+v", got, want)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for broadcast")
		}
	}
}
```

- [ ] **Step 3: 实现 Hub**

```go
package notification

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxSendBacklog = 16
)

type Hub struct {
	mu      sync.Mutex
	clients map[*hubClient]struct{}
	closed  bool
}

type hubClient struct {
	conn *websocket.Conn
	send chan []byte
}

func NewHub() *Hub {
	return &Hub{clients: make(map[*hubClient]struct{})}
}

func (h *Hub) Handle(conn *websocket.Conn) {
	client := &hubClient{conn: conn, send: make(chan []byte, maxSendBacklog)}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	go client.writeLoop()
	client.readLoop(h)
}

func (h *Hub) Broadcast(event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			delete(h.clients, client)
			close(client.send)
			_ = client.conn.Close()
		}
	}
}

func (h *Hub) remove(client *hubClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send)
	}
}

func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for client := range h.clients {
		delete(h.clients, client)
		close(client.send)
		_ = client.conn.Close()
	}
}

func (c *hubClient) readLoop(h *Hub) {
	defer func() {
		h.remove(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *hubClient) writeLoop() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case data, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
```

- [ ] **Step 4: 写 Handler**

```go
package handler

import (
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/dao"
	"vibebiz/internal/dao/model"
	"vibebiz/internal/notification"
	"vibebiz/internal/response"
)

type NotificationHandler struct {
	Service  *notification.Service
	Hub      *notification.Hub
	Upgrader websocket.Upgrader
}

func NewNotificationHandler(service *notification.Service, hub *notification.Hub, origins []string) *NotificationHandler {
	allowed := make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		allowed[strings.TrimRight(origin, "/")] = struct{}{}
	}
	return &NotificationHandler{
		Service: service,
		Hub:     hub,
		Upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := strings.TrimRight(r.Header.Get("Origin"), "/")
				if origin == "" {
					return true
				}
				if _, ok := allowed[origin]; ok {
					return true
				}
				return strings.EqualFold(origin, "http://"+r.Host) || strings.EqualFold(origin, "https://"+r.Host)
			},
		},
	}
}

type notificationRequest struct {
	Title    string                 `json:"title"`
	Content  string                 `json:"content"`
	Type     model.NotificationType `json:"type"`
	Version  string                 `json:"version"`
	Link     string                 `json:"link"`
	ExpireAt string                 `json:"expireAt"`
}

func (r notificationRequest) input() (notification.CreateInput, error) {
	expireAt, err := time.Parse(time.RFC3339, r.ExpireAt)
	if err != nil {
		return notification.CreateInput{}, notification.ErrInvalid
	}
	return notification.CreateInput{
		Title: r.Title, Content: r.Content, Type: r.Type,
		Version: r.Version, Link: r.Link, ExpireAt: expireAt,
	}, nil
}

func (h *NotificationHandler) AdminList(c echo.Context) error {
	items, err := h.Service.List(c.Request().Context(), dao.NotificationFilter{
		Type: c.QueryParam("type"), Status: c.QueryParam("status"),
	})
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, map[string]any{"notifications": items})
}

func (h *NotificationHandler) AdminCreate(c echo.Context) error {
	var req notificationRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	input, err := req.input()
	if err != nil {
		return notificationError(c, err)
	}
	session, ok := auth.SessionFrom(c)
	if !ok {
		return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized")
	}
	view, err := h.Service.Create(c.Request().Context(), input, notification.Creator{ID: session.UserID, Name: session.Username})
	if err != nil {
		return notificationError(c, err)
	}
	return response.Created(c, view)
}

func (h *NotificationHandler) AdminUpdate(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid id")
	}
	var req notificationRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid body")
	}
	input, err := req.input()
	if err != nil {
		return notificationError(c, err)
	}
	view, err := h.Service.Update(c.Request().Context(), id, input)
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, view)
}

func (h *NotificationHandler) AdminPublish(c echo.Context) error {
	return h.transition(c, h.Service.Publish)
}

func (h *NotificationHandler) AdminUnpublish(c echo.Context) error {
	return h.transition(c, h.Service.Unpublish)
}

func (h *NotificationHandler) AdminDelete(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid id")
	}
	if err := h.Service.Delete(c.Request().Context(), id); err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *NotificationHandler) WebList(c echo.Context) error {
	items, err := h.Service.Published(c.Request().Context())
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, map[string]any{"notifications": items})
}

func (h *NotificationHandler) Socket(c echo.Context) error {
	conn, err := h.Upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	h.Hub.Handle(conn)
	return nil
}

func (h *NotificationHandler) transition(c echo.Context, action func(context.Context, int64) (notification.View, error)) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid id")
	}
	view, err := action(c.Request().Context(), id)
	if err != nil {
		return notificationError(c, err)
	}
	return response.OK(c, view)
}

func notificationError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, notification.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid notification")
	case errors.Is(err, notification.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40401, "notification not found")
	case errors.Is(err, notification.ErrConflict):
		return response.Fail(c, http.StatusConflict, 40901, "notification state conflict")
	case errors.Is(err, notification.ErrCacheUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50301, "notification cache unavailable")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50001, "internal error")
	}
}
```

`Service.Hub` 字段类型为 `*Hub`，同时实现 `Broadcaster`。Handler 中补齐 `strconv`、`time`、`errors`、`context` import。

- [ ] **Step 5: 写 Handler 测试**

用 fake service/repo 或真实 service + fake repo/cache。至少覆盖：

```go
func TestNotificationAdminCreateRequiresSession(t *testing.T)
func TestNotificationAdminCreate(t *testing.T)
func TestNotificationAdminPublishConflict(t *testing.T)
func TestNotificationWebListUnauthorizedEnvelope(t *testing.T)
func TestNotificationWebListReturnsPublished(t *testing.T)
```

通过 `echo.New()` 注册路由并注入 `sessionMiddleware`（复用 `handler` 包已有 helper）。

- [ ] **Step 6: 路由装配**

`router.RegisterControl` 末尾新增参数 `notifications *handler.NotificationHandler`，注册：

```go
notificationsAPI := api.Group("/notifications", authn.RequireUser)
notificationsAPI.GET("", notifications.WebList)
notificationsAPI.GET("/ws", notifications.Socket)

adminProtected.GET("/notifications", notifications.AdminList)
adminProtected.POST("/notifications", notifications.AdminCreate)
adminProtected.PUT("/notifications/:id", notifications.AdminUpdate)
adminProtected.POST("/notifications/:id/publish", notifications.AdminPublish)
adminProtected.POST("/notifications/:id/unpublish", notifications.AdminUnpublish)
adminProtected.DELETE("/notifications/:id", notifications.AdminDelete)
```

更新 `helpers_test.go`、`span_test.go` 的 `RegisterControl` 调用，传入基于空仓库/空缓存的 handler 或新增的可选 nil 检查。路由测试中断言 `/cp/api/v1/notifications` 存在、`/dp/api/v1/notifications` 不存在。

- [ ] **Step 7: CP 装配**

在 `cmd/cp/main.go`：

```go
notificationRepo := dao.NewNotificationRepository(db)
notificationCache := notification.NewCache(redisClient, redisPrefix)
notificationHub := notification.NewHub()
notificationService := notification.NewService(notificationRepo, notificationCache, notificationHub)
notificationHandler := handler.NewNotificationHandler(notificationService, notificationHub, cfg.CorsOrigins)
```

传入 `router.RegisterControl`，并在 shutdown 时调用 `notificationHub.Close()`。

- [ ] **Step 8: 运行测试**

Run: `cd packages/backend && go test ./...`
Expected: PASS；`go vet ./...` 通过。

- [ ] **Step 9: 提交**

```bash
git add packages/backend
git commit -m "feat: expose notification api and websocket"
```

---

### Task 5: web 铃铛弹窗、帮助外链与 WebSocket 客户端

**Files:**
- Modify: `packages/web/package.json`
- Modify: `packages/web/vite.config.ts`
- Modify: `packages/web/nginx.conf`
- Modify: `packages/web/src/types/index.ts`
- Create: `packages/web/src/api/notifications.ts`
- Create: `packages/web/src/stores/notifications.ts`
- Create: `packages/web/src/composables/useNotificationSocket.ts`
- Create: `packages/web/src/utils/notificationSocket.ts`
- Create: `packages/web/src/utils/notificationGroups.ts`
- Create: `packages/web/src/components/home/NotificationsDialog.vue`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Test: `packages/web/src/api/notifications.test.ts`
- Test: `packages/web/src/utils/notificationSocket.test.ts`
- Test: `packages/web/src/utils/notificationGroups.test.ts`

**Interfaces:**
- Consumes: `cpHttp`、`request`、`auth` store、`HomeSidebar`
- Produces:
  - `NotificationItem` 类型
  - `listNotifications(): Promise<NotificationItem[]>`
  - `useNotificationsStore()`，含 `items`、`loading`、`error`、`load()`
  - `buildNotificationSocketUrl(base: string, location: Location): string`
  - `useNotificationSocket(onChange: () => void)`，返回 `{ connect, disconnect }`

- [ ] **Step 1: 安装依赖**

Run: `cd packages/web && npm install marked dompurify`
Expected: `package.json`、`package-lock.json` 更新。

- [ ] **Step 2: 写失败测试**

`notificationSocket.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { buildNotificationSocketUrl } from './notificationSocket'

describe('buildNotificationSocketUrl', () => {
  it('builds ws url from relative base', () => {
    const location = { protocol: 'https:', host: 'vibebiz-dev.com:5173' } as Location
    expect(buildNotificationSocketUrl('/cp/api/v1', location)).toBe(
      'wss://vibebiz-dev.com:5173/cp/api/v1/notifications/ws',
    )
  })

  it('builds ws url from absolute base', () => {
    const location = { protocol: 'http:', host: 'localhost:5173' } as Location
    expect(buildNotificationSocketUrl('http://cp:8080/cp/api/v1', location)).toBe(
      'ws://cp:8080/cp/api/v1/notifications/ws',
    )
  })
})
```

`notificationGroups.test.ts` 覆盖按 `type` 过滤与空数组，纯函数 `groupNotifications(items)` 放在 `src/utils/notificationGroups.ts`：

```ts
export function groupNotifications(items: NotificationItem[]) {
  return {
    version_release: items.filter((item) => item.type === 'version_release'),
    inbox: items.filter((item) => item.type === 'inbox'),
  }
}
```

测试断言分组顺序与空数组。

- [ ] **Step 3: 实现 API / Store / Socket**

`types/index.ts` 增加：

```ts
export type NotificationType = 'version_release' | 'inbox'

export interface NotificationItem {
  id: number
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  publishedAt: string
  expireAt: string
}
```

`api/notifications.ts`：

```ts
import { cpHttp, request } from './client'
import type { NotificationItem } from '@/types'

export async function listNotifications(): Promise<NotificationItem[]> {
  const result = await request<{ notifications: NotificationItem[] }>(cpHttp, {
    method: 'get',
    url: '/notifications',
  })
  return result.notifications ?? []
}
```

`utils/notificationSocket.ts`：

```ts
export function buildNotificationSocketUrl(base: string, location: Location): string {
  const url = new URL(base, `${location.protocol}//${location.host}`)
  url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
  url.pathname = `${url.pathname.replace(/\/$/, '')}/notifications/ws`
  return url.toString()
}
```

`stores/notifications.ts`：

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'

import { listNotifications } from '@/api/notifications'
import type { NotificationItem } from '@/types'

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<NotificationItem[]>([])
  const loading = ref(false)
  const error = ref('')

  async function load(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      items.value = await listNotifications()
    } catch (err) {
      error.value = err instanceof Error ? err.message : '加载失败'
    } finally {
      loading.value = false
    }
  }

  function clear(): void {
    items.value = []
    error.value = ''
  }

  return { items, loading, error, load, clear }
})
```

`composables/useNotificationSocket.ts`：

```ts
import { onBeforeUnmount, ref } from 'vue'

import { buildNotificationSocketUrl } from '@/utils/notificationSocket'

export function useNotificationSocket(onChange: () => void) {
  const connected = ref(false)
  let socket: WebSocket | null = null
  let retry = 1000
  let timer: ReturnType<typeof setTimeout> | null = null
  let stopped = true

  function open(): void {
    const base = import.meta.env.VITE_CP_BASE || '/cp/api/v1'
    socket = new WebSocket(buildNotificationSocketUrl(base, window.location))
    socket.onopen = () => {
      connected.value = true
      retry = 1000
    }
    socket.onmessage = (event) => {
      try {
        const payload = JSON.parse(event.data) as { type?: string }
        if (payload.type === 'notifications.changed') onChange()
      } catch {
        // ignore malformed events
      }
    }
    socket.onclose = () => {
      connected.value = false
      socket = null
      if (!stopped) schedule()
    }
    socket.onerror = () => socket?.close()
  }

  function schedule(): void {
    timer = setTimeout(open, retry)
    retry = Math.min(retry * 2, 30_000)
  }

  function connect(): void {
    stopped = false
    if (socket) return
    open()
  }

  function disconnect(): void {
    stopped = true
    if (timer) clearTimeout(timer)
    timer = null
    socket?.close()
    socket = null
    connected.value = false
  }

  onBeforeUnmount(disconnect)

  return { connected, connect, disconnect }
}
```

- [ ] **Step 4: 实现 Dialog**

`NotificationsDialog.vue`：

- props `{ visible: boolean }`，emit `update:visible`。
- `watch(visible)` 打开时 `store.load()`。
- `tabs` 用 PrimeVue `Tabs/TabList/Tab/TabPanels/TabPanel`。
- `renderMarkdown(content)`：

```ts
import DOMPurify from 'dompurify'
import { marked } from 'marked'

function renderMarkdown(content: string): string {
  return DOMPurify.sanitize(marked.parse(content, { async: false }) as string)
}
```

- 列表项：标题、`formatDateTime(publishedAt)`、版本号 tag、`v-html="renderMarkdown(item.content)"`、链接按钮 `target="_blank" rel="noopener noreferrer"`。
- 空状态“暂无消息”。
- 样式只用现有 CSS 变量与黑/白/灰/蓝。

- [ ] **Step 5: 接入 HomeSidebar**

- 帮助：

```vue
<a
  class="sidebar-icon-btn"
  href="https://help.atoms.dev/zh"
  target="_blank"
  rel="noopener noreferrer"
  aria-label="帮助"
  title="帮助"
>
  <i class="pi pi-question-circle" />
</a>
```

- 铃铛：

```vue
<button class="sidebar-icon-btn" type="button" aria-label="通知" title="通知" @click="notificationsOpen = true">
  <i class="pi pi-bell" />
</button>
```

- 引入 `NotificationsDialog`，`const notificationsOpen = ref(false)`。
- `HomeView.vue` 挂载后 `auth.ensure()` 成功时启动 WS，`onBeforeUnmount` 断开；收到事件调用 `store.load()`。退出登录调用 `disconnect()` 与 `store.clear()`。

- [ ] **Step 6: 代理与 nginx**

`vite.config.ts` 的 `/cp` 增加 `ws: true`。

`nginx.conf` 新增：

```nginx
location = /cp/api/v1/notifications/ws {
    proxy_pass http://cp:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 3600s;
}
```

- [ ] **Step 7: 运行测试与类型检查**

Run: `cd packages/web && npm test && npm run build`
Expected: PASS；`vue-tsc` 无错误。

- [ ] **Step 8: 提交**

```bash
git add packages/web
git commit -m "feat: add web notification dialog and socket"
```

---

### Task 6: ops-web 消息管理页

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Create: `packages/ops-web/src/api/notifications.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`
- Create: `packages/ops-web/src/views/NotificationsView.vue`
- Create: `packages/ops-web/src/utils/notificationStatus.ts`
- Test: `packages/ops-web/src/utils/notificationStatus.test.ts`
- Test: `packages/ops-web/src/api/notifications.test.ts`

**Interfaces:**
- Consumes: ops `request`（base `/cp/api/v1/admin`）、`DataTable`、`formatDateTime`
- Produces:
  - `OpsNotification` 类型
  - `listNotifications()`, `createNotification(input)`, `updateNotification(id,input)`, `publishNotification(id)`, `unpublishNotification(id)`, `deleteNotification(id)`
  - `notificationStatusLabel(status)`、`isExpired(item, now)`、`canEdit(item)`

- [ ] **Step 1: 写失败测试**

`notificationStatus.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { canEdit, notificationStatusLabel } from './notificationStatus'

describe('notificationStatusLabel', () => {
  it('labels draft, published, unpublished and expired', () => {
    expect(notificationStatusLabel('draft')).toBe('草稿')
    expect(notificationStatusLabel('published')).toBe('已发布')
    expect(notificationStatusLabel('unpublished')).toBe('已取消发布')
    expect(notificationStatusLabel('expired')).toBe('已过期')
  })
})

describe('canEdit', () => {
  it('allows draft and unpublished only', () => {
    expect(canEdit({ status: 'draft' } as never)).toBe(true)
    expect(canEdit({ status: 'unpublished' } as never)).toBe(true)
    expect(canEdit({ status: 'published' } as never)).toBe(false)
  })
})
```

`api/notifications.test.ts` 使用 `vi.mock('./client')` 断言 method/url/data。

- [ ] **Step 2: 实现类型与 API**

```ts
export type NotificationType = 'version_release' | 'inbox'
export type NotificationStatus = 'draft' | 'published' | 'unpublished' | 'expired'

export interface OpsNotification {
  id: number
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  status: NotificationStatus
  expireAt: string
  publishedAt: string | null
  createdBy: number
  createdByName: string
  createdAt: string
  updatedAt: string
}

export interface NotificationInput {
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
  expireAt: string
}
```

API 函数全部返回 `unwrap` 后的实体或数组。

- [ ] **Step 3: 实现状态工具**

```ts
import type { NotificationStatus, OpsNotification } from '@/types'

const labels: Record<NotificationStatus, string> = {
  draft: '草稿',
  published: '已发布',
  unpublished: '已取消发布',
  expired: '已过期',
}

export function notificationStatusLabel(status: NotificationStatus): string {
  return labels[status] ?? status
}

export function isExpired(item: Pick<OpsNotification, 'status'>, now = new Date()): boolean {
  if (item.status === 'expired') return true
  return false
}

export function canEdit(item: Pick<OpsNotification, 'status'>): boolean {
  return item.status === 'draft' || item.status === 'unpublished'
}
```

后端已派生 `expired`，前端直接使用；`isExpired` 保留以兼容后端口径。

- [ ] **Step 4: 实现页面**

`NotificationsView.vue`：

- 顶部：标题“消息通知”、描述、刷新按钮、新建按钮。
- 表格列：标题、类型、状态、版本号、过期时间、创建人、创建时间、操作。
- 操作区按钮：编辑（`canEdit`）、发布（`status !== 'published' && status !== 'expired'`）、取消发布（`published`）、删除。
- 新建/编辑 Dialog：`Dialog` + 表单；类型 `Select`；版本号仅 `version_release` 显示；`DatePicker` 选过期日期（`showTime` 可选，默认当天 23:59:59）。
- 提交时构造 `expireAt` ISO；发布/取消发布/删除后 `load()` 并显示 `Toast` 或页面错误文案。
- 不引入新 UI 库，使用 PrimeVue 已有组件。

- [ ] **Step 5: 路由与导航**

`navigation.ts` 增加：

```ts
{ name: 'notifications', label: '消息通知', path: '/notifications', icon: 'pi pi-bell' },
```

`routes.ts` 增加：

```ts
{ path: 'notifications', name: 'notifications', component: NotificationsView, meta: { title: '消息通知' } },
```

更新 `navigation.test.ts` 断言包含该路由。

- [ ] **Step 6: 运行测试与构建**

Run: `cd packages/ops-web && npm test && npm run build`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add packages/ops-web
git commit -m "feat: add ops notification management"
```

---

### Task 7: 迁移执行、全量验证与冒烟

**Files:**
- Modify: `.ai/CURRENT_FOCUS`（checkpoint 自动生成）
- No source changes expected

- [ ] **Step 1: 执行迁移**

Run: `make db-migrate-up`
Expected: `00008_ops_notification.sql` 执行成功。

- [ ] **Step 2: 全量验证**

Run: `make validate`
Expected: gofmt / go vet / ruff / web eslint / ops-web eslint / 调色板 / 构建全部通过。

- [ ] **Step 3: 手工冒烟**

启动：`make dev`

1. 登录 ops-web，创建两条消息（版本发布 + 收集箱），保存草稿。
2. 发布版本发布消息。
3. 登录 web，点击铃铛，两个 Tab 分别展示正确消息。
4. 保持 web 弹窗打开，ops 发布收集箱消息；web 列表静默新增。
5. ops 取消发布版本消息；web 列表静默移除。
6. ops 删除消息；DB 软删除、Redis key 不存在。
7. `redis-cli TTL vibebiz:notification:item:{id}` 约等于剩余停留时间。
8. 点击帮助按钮新窗口打开 `https://help.atoms.dev/zh`。

- [ ] **Step 4: 记录失败与检查点**

```bash
make checkpoint
```

如有可复用经验：

```bash
make memory-add
```

- [ ] **Step 5: 提交收尾**

```bash
git add -A
git commit -m "chore: checkpoint ops notifications"
```

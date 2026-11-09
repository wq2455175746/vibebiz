# 个人名片后端与真实数据接入 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 用 CP + MySQL + S3 替换名片的前端 mock/localStorage，完成真实的名片编辑、发现、公开分享与 ops 管理。

**Architecture:** 新增 `internal/businesscard` 聚合（Repository + Service），两张 MySQL 表；项目条目只存 project public_id，读取时实时解析；图片/PDF 主预览写 S3，MySQL 只存 key。web 与 ops-web 删除 mock，全部走 CP API。

**Tech Stack:** Go 1.24 + Echo v4 + GORM + MySQL 8 + goose + AWS S3/minio；Vue 3 + Pinia + Vitest；ops-web 复用 DataTable/useAdminPage。

**Spec:** `docs/superpowers/specs/2026-11-09-business-card-backend-design.md`

## Global Constraints

- 只改 `packages/backend`、`packages/web`、`packages/ops-web`；不改 DP / sandbox / 现有 `/p/:slug` 行为。
- 一个用户一张名片；`business_card.user_id` 唯一；`public_id` 使用 UUID。
- 名片最多 5 个项目、5 个标签；单标签 ≤12 字，去重。
- 项目条目只存 `project_public_id + sort_order`，不存名称/封面快照。
- 只能添加自己工作区内、未隐藏、已发布的项。
- 主预览文件仅 PDF / 图片，≤2MB；校验 MIME + magic bytes；S3 key 前缀 `cards/<userID>/`。
- API 统一信封 `{code,message,result}`；成功 `code=0`。
- 公开分享接口无登录；发现接口需登录；admin 接口 RequireOps。
- 颜色只允许黑/白/灰/蓝。
- `comment_count` 固定 0；不做评论/IM。
- 删除 web `src/mocks/cards.ts` 与 ops `src/mocks/cards.ts`；旧 localStorage 名片不迁移。
- 计划中的 `git commit` 步骤只在用户明确要求提交时执行；否则跳过提交。
- 最终验证：`go test ./...`、`go vet ./...`、web/ops `npm test -- --run && npm run build && npm run lint`、根目录 `make validate`。

---

## 文件结构

```
packages/backend/
├── migrations/00033_business_card.sql
├── migrations/migrations_test.go
├── internal/dao/model/business_card.go
├── internal/dao/model/table_test.go
├── internal/businesscard/
│   ├── repository.go          # Repository interface + MySQL impl + records
│   ├── repository_test.go
│   ├── service.go             # business rules + resolution + S3
│   ├── service_test.go
│   ├── handler.go             # optional HTTP DTO helpers (if needed)
│   ├── handler_test.go
├── internal/handler/card.go
├── internal/handler/card_test.go
├── internal/router/control.go
├── internal/router/helpers_test.go
├── internal/router/span_test.go
└── cmd/cp/main.go

packages/web/src/
├── api/cards.ts
├── api/cards.test.ts
├── types/index.ts
├── utils/card.ts
├── stores/card.ts
├── stores/card.test.ts
├── components/card/{MyCardPanel,CardProfilePanel,CardPreview,CardTile,DiscoverPanel}.vue
├── components/card/*.test.ts
├── views/CardShareView.vue
├── views/ProjectShareView.vue
├── views/*.test.ts
└── styles/card.css

packages/ops-web/src/
├── api/cards.ts
├── api/cards.test.ts
├── types/index.ts
├── views/CardsView.vue
└── views/CardsView.test.ts
```

---

### Task 1: 迁移、DAO 模型与迁移测试

**Files:**
- Create: `packages/backend/migrations/00033_business_card.sql`
- Create: `packages/backend/internal/dao/model/business_card.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Modify: `packages/backend/internal/dao/model/table_test.go`

**Interfaces:**
- Produces: GORM 模型 `model.BusinessCard`、`model.BusinessCardItem`；表名 `business_card`、`business_card_item`；迁移版本 33。

- [ ] **Step 1: 先更新测试，确认失败**

`packages/backend/migrations/migrations_test.go`：

- `TestInitialSchemaCoversAllBusinessTables` 的 `tables` 切片加入 `"business_card"`、`"business_card_item"`。
- `TestGooseCanCollectMigrations` 的期望数量从 `32` 改成 `33`。

`packages/backend/internal/dao/model/table_test.go` 的 `cases` 加入：

```go
{&BusinessCard{}, "business_card"},
{&BusinessCardItem{}, "business_card_item"},
```

Run:

```bash
cd packages/backend && go test ./migrations ./internal/dao/model
```

Expected: FAIL — 表不存在的迁移断言 / `BusinessCard` 未定义。

- [ ] **Step 2: 写迁移**

Create `packages/backend/migrations/00033_business_card.sql`:

```sql
-- +goose Up

CREATE TABLE `business_card` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '名片公开ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '所属用户ID',
  `headline` varchar(80) NOT NULL DEFAULT '' COMMENT '一句话简介',
  `tags` json DEFAULT NULL COMMENT '标签数组',
  `featured_type` varchar(16) NOT NULL DEFAULT '' COMMENT 'project/image/pdf',
  `featured_project_public_id` char(36) NOT NULL DEFAULT '' COMMENT '项目主预览',
  `featured_s3_key` varchar(512) NOT NULL DEFAULT '' COMMENT '文件主预览S3 key',
  `featured_file_name` varchar(255) NOT NULL DEFAULT '' COMMENT '原始文件名',
  `featured_file_size` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '文件大小',
  `published` tinyint UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否已发布',
  `published_at` datetime DEFAULT NULL COMMENT '发布时间',
  `comment_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '评论数',
  `is_hidden` tinyint UNSIGNED NOT NULL DEFAULT 0 COMMENT 'ops隐藏',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_public` (`public_id`),
  UNIQUE KEY `uk_business_card_user` (`user_id`),
  KEY `idx_business_card_discover` (`published`, `is_hidden`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='用户名片';

CREATE TABLE `business_card_item` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `sort_order` int NOT NULL DEFAULT 0 COMMENT '展示顺序',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_item` (`card_id`, `project_public_id`),
  KEY `idx_business_card_item_project` (`project_public_id`),
  KEY `idx_business_card_item_order` (`card_id`, `sort_order`),
  CONSTRAINT `fk_business_card_item_card` FOREIGN KEY (`card_id`) REFERENCES `business_card` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片项目条目';

-- +goose Down

DROP TABLE IF EXISTS `business_card_item`;
DROP TABLE IF EXISTS `business_card`;
```

- [ ] **Step 3: 写模型**

Create `packages/backend/internal/dao/model/business_card.go`:

```go
package model

import "time"

type BusinessCard struct {
	Base
	PublicID                string     `gorm:"column:public_id" json:"publicId"`
	UserID                  int64      `gorm:"column:user_id" json:"userId"`
	Headline                string     `gorm:"column:headline" json:"headline"`
	Tags                    JSON       `gorm:"column:tags;type:json" json:"tags,omitempty"`
	FeaturedType            string     `gorm:"column:featured_type" json:"featuredType"`
	FeaturedProjectPublicID string     `gorm:"column:featured_project_public_id" json:"featuredProjectPublicId"`
	FeaturedS3Key           string     `gorm:"column:featured_s3_key" json:"featuredS3Key"`
	FeaturedFileName        string     `gorm:"column:featured_file_name" json:"featuredFileName"`
	FeaturedFileSize        int64      `gorm:"column:featured_file_size" json:"featuredFileSize"`
	Published               int8       `gorm:"column:published" json:"published"`
	PublishedAt             *time.Time `gorm:"column:published_at" json:"publishedAt,omitempty"`
	CommentCount            int        `gorm:"column:comment_count" json:"commentCount"`
	IsHidden                int8       `gorm:"column:is_hidden" json:"hidden"`
}

func (BusinessCard) TableName() string { return "business_card" }

type BusinessCardItem struct {
	Base
	CardID          int64  `gorm:"column:card_id" json:"cardId"`
	ProjectPublicID string `gorm:"column:project_public_id" json:"projectPublicId"`
	SortOrder       int    `gorm:"column:sort_order" json:"sortOrder"`
}

func (BusinessCardItem) TableName() string { return "business_card_item" }
```

- [ ] **Step 4: 跑测试**

```bash
cd packages/backend && go test ./migrations ./internal/dao/model
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add packages/backend/migrations/00033_business_card.sql packages/backend/migrations/migrations_test.go packages/backend/internal/dao/model/business_card.go packages/backend/internal/dao/model/table_test.go
git commit -m "feat(backend): add business card tables and models"
```

---

### Task 2: businesscard Repository（MySQL）

**Files:**
- Create: `packages/backend/internal/businesscard/repository.go`
- Test: `packages/backend/internal/businesscard/repository_test.go`

**Interfaces:**
- Consumes: `model.BusinessCard`、`model.BusinessCardItem`。
- Produces:

```go
type CardRecord struct {
	ID                int64
	PublicID          string
	UserID            int64
	OwnerName         string
	OwnerNickname     string
	OwnerAvatarS3Path string
	Headline          string
	Tags              []string
	FeaturedType      string
	FeaturedProjectID string
	FeaturedS3Key     string
	FeaturedFileName  string
	FeaturedFileSize  int64
	Published         bool
	PublishedAt       *time.Time
	CommentCount      int
	Hidden            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Items             []ItemRecord
}

type ItemRecord struct {
	ProjectID string
	SortOrder int
}

type OwnerProfile struct {
	UserID       int64
	Username     string
	Nickname     string
	AvatarS3Path string
}

type Repository interface {
	GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error)
	GetByUser(ctx context.Context, userID int64) (*CardRecord, error)
	GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error)
	UpdateProfile(ctx context.Context, cardID int64, headline string, tags []string) error
	ListItems(ctx context.Context, cardID int64) ([]ItemRecord, error)
	AddItem(ctx context.Context, cardID int64, projectID string, sortOrder int) error
	RemoveItem(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedProject(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedFile(ctx context.Context, cardID int64, key, name, fileType string, size int64) error
	ClearFeatured(ctx context.Context, cardID int64) error
	Publish(ctx context.Context, cardID int64, at time.Time) error
	SetHidden(ctx context.Context, cardID int64, hidden bool) error
	Delete(ctx context.Context, cardID int64) (featuredKey string, err error)
	Discover(ctx context.Context, page, pageSize int) ([]*CardRecord, int64, error)
	AdminList(ctx context.Context, keyword string, page, pageSize int) ([]*CardRecord, int64, error)
	FindByProject(ctx context.Context, projectID string) (*CardRecord, error)
	OwnerProfile(ctx context.Context, userID int64) (*OwnerProfile, error)
}

var ErrNotFound = errors.New("business card not found")
```

- [ ] **Step 1: 写失败测试**

Create `packages/backend/internal/businesscard/repository_test.go`。使用 `sqlmock` + GORM 的标准写法（与 `internal/dao/repository_test.go` 相同的 `newMockDB`）：

```go
package businesscard

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock
}

func TestGetOrCreateCreatesMissingCard(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)

	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `business_card`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "public_id", "user_id", "headline", "tags", "featured_type",
			"featured_project_public_id", "featured_s3_key", "featured_file_name",
			"featured_file_size", "published", "published_at", "comment_count", "is_hidden",
			"created_at", "updated_at", "owner_username", "owner_nickname", "owner_avatar_s3_path",
		}).AddRow(1, "card-1", 7, "", "[]", "", "", "", "", 0, 0, nil, 0, 0, "2026-11-09T00:00:00Z", "2026-11-09T00:00:00Z", "alice", "alice", ""))
	mock.ExpectQuery("SELECT \\* FROM `business_card_item`").
		WillReturnRows(sqlmock.NewRows([]string{"id", "card_id", "project_public_id", "sort_order"}))

	card, err := repo.GetOrCreate(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if card.PublicID != "card-1" || card.OwnerName != "alice" {
		t.Fatalf("card = %+v", card)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveItemHardDeletes(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `business_card_item` WHERE card_id = \\? AND project_public_id = \\?").
		WithArgs(int64(1), "p1").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := repo.RemoveItem(context.Background(), 1, "p1"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteReturnsFeaturedKey(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT `id`,`featured_s3_key` FROM `business_card` WHERE `business_card`\\.`id` = \\?").
		WithArgs(int64(1), 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "featured_s3_key"}).AddRow(1, "cards/7/a.pdf"))
	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM `business_card_item` WHERE card_id = \\?").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("DELETE FROM `business_card` WHERE `business_card`\\.`id` = \\?").
		WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	key, err := repo.Delete(context.Background(), 1)
	if err != nil || key != "cards/7/a.pdf" {
		t.Fatalf("key = %q err = %v", key, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestFindByProjectNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	mock.ExpectQuery("SELECT bc\\.\\*, u\\.username AS owner_username").
		WillReturnRows(sqlmock.NewRows([]string{"id"}))
	if _, err := repo.FindByProject(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}
```

Run:

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: FAIL — 包/类型不存在。

- [ ] **Step 2: 写 repository.go**

Create `packages/backend/internal/businesscard/repository.go`。完整实现如下（可把 `OwnerProfile` 与 records 放在同文件）：

```go
package businesscard

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"vibebiz/internal/dao/model"
)

var ErrNotFound = errors.New("business card not found")

type CardRecord struct {
	ID                int64
	PublicID          string
	UserID            int64
	OwnerName         string
	OwnerNickname     string
	OwnerAvatarS3Path string
	Headline          string
	Tags              []string
	FeaturedType      string
	FeaturedProjectID string
	FeaturedS3Key     string
	FeaturedFileName  string
	FeaturedFileSize  int64
	Published         bool
	PublishedAt       *time.Time
	CommentCount      int
	Hidden            bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Items             []ItemRecord
}

type ItemRecord struct {
	ProjectID string
	SortOrder int
}

type OwnerProfile struct {
	UserID       int64
	Username     string
	Nickname     string
	AvatarS3Path string
}

type Repository interface {
	GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error)
	GetByUser(ctx context.Context, userID int64) (*CardRecord, error)
	GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error)
	UpdateProfile(ctx context.Context, cardID int64, headline string, tags []string) error
	ListItems(ctx context.Context, cardID int64) ([]ItemRecord, error)
	AddItem(ctx context.Context, cardID int64, projectID string, sortOrder int) error
	RemoveItem(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedProject(ctx context.Context, cardID int64, projectID string) error
	SetFeaturedFile(ctx context.Context, cardID int64, key, name, fileType string, size int64) error
	ClearFeatured(ctx context.Context, cardID int64) error
	Publish(ctx context.Context, cardID int64, at time.Time) error
	SetHidden(ctx context.Context, cardID int64, hidden bool) error
	Delete(ctx context.Context, cardID int64) (featuredKey string, err error)
	Discover(ctx context.Context, page, pageSize int) ([]*CardRecord, int64, error)
	AdminList(ctx context.Context, keyword string, page, pageSize int) ([]*CardRecord, int64, error)
	FindByProject(ctx context.Context, projectID string) (*CardRecord, error)
	OwnerProfile(ctx context.Context, userID int64) (*OwnerProfile, error)
}

type MySQLRepository struct {
	db *gorm.DB
}

func NewMySQLRepository(db *gorm.DB) *MySQLRepository {
	return &MySQLRepository{db: db}
}

type cardRow struct {
	ID                      int64          `gorm:"column:id"`
	PublicID                string         `gorm:"column:public_id"`
	UserID                  int64          `gorm:"column:user_id"`
	Headline                string         `gorm:"column:headline"`
	Tags                    model.JSON     `gorm:"column:tags"`
	FeaturedType            string         `gorm:"column:featured_type"`
	FeaturedProjectPublicID string         `gorm:"column:featured_project_public_id"`
	FeaturedS3Key           string         `gorm:"column:featured_s3_key"`
	FeaturedFileName        string         `gorm:"column:featured_file_name"`
	FeaturedFileSize        int64          `gorm:"column:featured_file_size"`
	Published               int8           `gorm:"column:published"`
	PublishedAt             *time.Time     `gorm:"column:published_at"`
	CommentCount            int            `gorm:"column:comment_count"`
	IsHidden                int8           `gorm:"column:is_hidden"`
	CreatedAt               time.Time      `gorm:"column:created_at"`
	UpdatedAt               time.Time      `gorm:"column:updated_at"`
	OwnerUsername           string         `gorm:"column:owner_username"`
	OwnerNickname           string         `gorm:"column:owner_nickname"`
	OwnerAvatarS3Path       string         `gorm:"column:owner_avatar_s3_path"`
}

const cardSelect = "bc.*, u.username AS owner_username, u.nickname AS owner_nickname, u.avatar_s3_path AS owner_avatar_s3_path"

func (r cardRow) record(items []ItemRecord) *CardRecord {
	tags := []string{}
	if len(r.Tags) > 0 {
		_ = json.Unmarshal(r.Tags, &tags)
	}
	if tags == nil {
		tags = []string{}
	}
	name := r.OwnerNickname
	if name == "" {
		name = r.OwnerUsername
	}
	return &CardRecord{
		ID: r.ID, PublicID: r.PublicID, UserID: r.UserID,
		OwnerName: name, OwnerNickname: r.OwnerNickname, OwnerAvatarS3Path: r.OwnerAvatarS3Path,
		Headline: r.Headline, Tags: tags,
		FeaturedType: r.FeaturedType, FeaturedProjectID: r.FeaturedProjectPublicID,
		FeaturedS3Key: r.FeaturedS3Key, FeaturedFileName: r.FeaturedFileName, FeaturedFileSize: r.FeaturedFileSize,
		Published: r.Published == 1, PublishedAt: r.PublishedAt, CommentCount: r.CommentCount,
		Hidden: r.IsHidden == 1, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Items: items,
	}
}

func (r *MySQLRepository) baseCardQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("business_card AS bc").
		Joins("JOIN user AS u ON u.id = bc.user_id AND u.deleted_at IS NULL").
		Where("bc.deleted_at IS NULL")
}

func (r *MySQLRepository) getCard(ctx context.Context, where string, args ...any) (*CardRecord, error) {
	var row cardRow
	err := r.baseCardQuery(ctx).Select(cardSelect).Where(where, args...).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get card: %w", err)
	}
	items, err := r.loadItems(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return row.record(items), nil
}

func (r *MySQLRepository) GetByUser(ctx context.Context, userID int64) (*CardRecord, error) {
	return r.getCard(ctx, "bc.user_id = ?", userID)
}

func (r *MySQLRepository) GetByPublicID(ctx context.Context, publicID string) (*CardRecord, error) {
	return r.getCard(ctx, "bc.public_id = ?", publicID)
}

func (r *MySQLRepository) GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error) {
	card, err := r.GetByUser(ctx, userID)
	if err == nil {
		return card, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	row := &model.BusinessCard{
		PublicID: uuid.NewString(),
		UserID:   userID,
		Tags:     model.JSON("[]"),
	}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		if card, retryErr := r.GetByUser(ctx, userID); retryErr == nil {
			return card, nil
		}
		return nil, fmt.Errorf("create card: %w", err)
	}
	return r.GetByUser(ctx, userID)
}

func (r *MySQLRepository) UpdateProfile(ctx context.Context, cardID int64, headline string, tags []string) error {
	payload, _ := json.Marshal(tags)
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{"headline": headline, "tags": model.JSON(payload)}).Error
}

func (r *MySQLRepository) loadItems(ctx context.Context, cardID int64) ([]ItemRecord, error) {
	var rows []model.BusinessCardItem
	if err := r.db.WithContext(ctx).Where("card_id = ?", cardID).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list card items: %w", err)
	}
	out := make([]ItemRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, ItemRecord{ProjectID: row.ProjectPublicID, SortOrder: row.SortOrder})
	}
	return out, nil
}

func (r *MySQLRepository) ListItems(ctx context.Context, cardID int64) ([]ItemRecord, error) {
	return r.loadItems(ctx, cardID)
}

func (r *MySQLRepository) loadItemsByCards(ctx context.Context, cardIDs []int64) (map[int64][]ItemRecord, error) {
	out := make(map[int64][]ItemRecord, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	var rows []model.BusinessCardItem
	if err := r.db.WithContext(ctx).Where("card_id IN ?", cardIDs).
		Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list card items: %w", err)
	}
	for _, row := range rows {
		out[row.CardID] = append(out[row.CardID], ItemRecord{ProjectID: row.ProjectPublicID, SortOrder: row.SortOrder})
	}
	return out, nil
}

func (r *MySQLRepository) AddItem(ctx context.Context, cardID int64, projectID string, sortOrder int) error {
	row := &model.BusinessCardItem{CardID: cardID, ProjectPublicID: projectID, SortOrder: sortOrder}
	if err := r.db.WithContext(ctx).Create(row).Error; err != nil {
		return fmt.Errorf("add card item: %w", err)
	}
	return nil
}

func (r *MySQLRepository) RemoveItem(ctx context.Context, cardID int64, projectID string) error {
	return r.db.WithContext(ctx).Unscoped().
		Where("card_id = ? AND project_public_id = ?", cardID, projectID).
		Delete(&model.BusinessCardItem{}).Error
}

func (r *MySQLRepository) SetFeaturedProject(ctx context.Context, cardID int64, projectID string) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": "project", "featured_project_public_id": projectID,
			"featured_s3_key": "", "featured_file_name": "", "featured_file_size": 0,
		}).Error
}

func (r *MySQLRepository) SetFeaturedFile(ctx context.Context, cardID int64, key, name, fileType string, size int64) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": fileType, "featured_project_public_id": "",
			"featured_s3_key": key, "featured_file_name": name, "featured_file_size": size,
		}).Error
}

func (r *MySQLRepository) ClearFeatured(ctx context.Context, cardID int64) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{
			"featured_type": "", "featured_project_public_id": "",
			"featured_s3_key": "", "featured_file_name": "", "featured_file_size": 0,
		}).Error
}

func (r *MySQLRepository) Publish(ctx context.Context, cardID int64, at time.Time) error {
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Updates(map[string]any{"published": 1, "published_at": at}).Error
}

func (r *MySQLRepository) SetHidden(ctx context.Context, cardID int64, hidden bool) error {
	value := int8(0)
	if hidden {
		value = 1
	}
	return r.db.WithContext(ctx).Model(&model.BusinessCard{}).Where("id = ?", cardID).
		Update("is_hidden", value).Error
}

func (r *MySQLRepository) Delete(ctx context.Context, cardID int64) (string, error) {
	var row model.BusinessCard
	if err := r.db.WithContext(ctx).Select("id", "featured_s3_key").First(&row, cardID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", fmt.Errorf("get card for delete: %w", err)
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("card_id = ?", cardID).Delete(&model.BusinessCardItem{}).Error; err != nil {
			return err
		}
		return tx.Unscoped().Delete(&model.BusinessCard{}, cardID).Error
	})
	if err != nil {
		return "", err
	}
	return row.FeaturedS3Key, nil
}

func (r *MySQLRepository) listCards(ctx context.Context, scope func(*gorm.DB) *gorm.DB, order string, page, pageSize int) ([]*CardRecord, int64, error) {
	var total int64
	if err := scope(r.baseCardQuery(ctx)).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count cards: %w", err)
	}
	var rows []cardRow
	if err := scope(r.baseCardQuery(ctx).Select(cardSelect)).
		Order(order).Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list cards: %w", err)
	}
	cardIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		cardIDs = append(cardIDs, row.ID)
	}
	itemsByCard, err := r.loadItemsByCards(ctx, cardIDs)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*CardRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record(itemsByCard[row.ID]))
	}
	return out, total, nil
}

func (r *MySQLRepository) Discover(ctx context.Context, page, pageSize int) ([]*CardRecord, int64, error) {
	scope := func(tx *gorm.DB) *gorm.DB {
		return tx.Where("bc.published = 1 AND bc.is_hidden = 0")
	}
	return r.listCards(ctx, scope, "bc.updated_at DESC, bc.id DESC", page, pageSize)
}

func (r *MySQLRepository) AdminList(ctx context.Context, keyword string, page, pageSize int) ([]*CardRecord, int64, error) {
	scope := func(tx *gorm.DB) *gorm.DB {
		if strings.TrimSpace(keyword) != "" {
			like := "%" + strings.TrimSpace(keyword) + "%"
			tx = tx.Where("(u.username LIKE ? OR u.nickname LIKE ? OR bc.headline LIKE ?)", like, like, like)
		}
		return tx
	}
	return r.listCards(ctx, scope, "bc.updated_at DESC, bc.id DESC", page, pageSize)
}

func (r *MySQLRepository) FindByProject(ctx context.Context, projectID string) (*CardRecord, error) {
	var row cardRow
	err := r.db.WithContext(ctx).Table("business_card_item AS i").
		Select(cardSelect).
		Joins("JOIN business_card AS bc ON bc.id = i.card_id AND bc.deleted_at IS NULL").
		Joins("JOIN user AS u ON u.id = bc.user_id AND u.deleted_at IS NULL").
		Where("i.deleted_at IS NULL AND i.project_public_id = ?", projectID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find card by project: %w", err)
	}
	items, err := r.loadItems(ctx, row.ID)
	if err != nil {
		return nil, err
	}
	return row.record(items), nil
}

func (r *MySQLRepository) OwnerProfile(ctx context.Context, userID int64) (*OwnerProfile, error) {
	var row struct {
		ID           int64  `gorm:"column:id"`
		Username     string `gorm:"column:username"`
		Nickname     string `gorm:"column:nickname"`
		AvatarS3Path string `gorm:"column:avatar_s3_path"`
	}
	err := r.db.WithContext(ctx).Table("user").
		Select("id, username, nickname, avatar_s3_path").
		Where("id = ? AND deleted_at IS NULL", userID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get owner profile: %w", err)
	}
	return &OwnerProfile{UserID: row.ID, Username: row.Username, Nickname: row.Nickname, AvatarS3Path: row.AvatarS3Path}, nil
}
```

注意：

- `listCards` 的 `scope(r.baseCardQuery(ctx)).Count` 与 `Select(cardSelect)` 分开构建，避免 GORM 复用同一 statement。
- `RemoveItem` / `Delete` 使用 `Unscoped()` 硬删除条目。
- `GetOrCreate` 依赖 `user_id` 唯一键处理并发；唯一冲突时重查。

- [ ] **Step 3: 跑测试**

```bash
cd packages/backend && go test ./internal/businesscard
```

如果 sqlmock 的 SQL 正则与实际 GORM 输出不一致，按实际输出调整正则，但不要放宽到匹配任意 SQL。Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add packages/backend/internal/businesscard/repository.go packages/backend/internal/businesscard/repository_test.go
git commit -m "feat(backend): add business card repository"
```

---

### Task 3: businesscard Service 核心（名片、项目、主预览项目、发布、解析）

**Files:**
- Create: `packages/backend/internal/businesscard/service.go`
- Test: `packages/backend/internal/businesscard/service_test.go`

**Interfaces:**
- Consumes: Task 2 `Repository`、`CardRecord`、`ItemRecord`；`store.ProjectStore`、`publish.Service`、`s3.Presigner`。
- Produces:
  - `businesscard.Card`、`CardItem`、`Featured`、`DiscoverResult`、`AdminRow`、`PublicProject`（本任务先实现 Card/CardItem/Featured 与核心方法；discovery/admin/public 在 Task 5 补）
  - `NewService(repo Repository, projects ProjectReader, publisher Publisher, s3 S3, publicBaseURL string, presignExpiry time.Duration) *Service`
  - `GetMine(ctx, userID int64, workspaceIDs []int64) (*Card, error)`
  - `GetPublic(ctx, publicID string) (*Card, error)`
  - `UpdateProfile(ctx, userID int64, workspaceIDs []int64, headline string, tags []string) (*Card, error)`
  - `AddProject(ctx, userID int64, workspaceIDs []int64, projectID string) (*Card, error)`
  - `RemoveProject(ctx, userID int64, workspaceIDs []int64, projectID string) (*Card, error)`
  - `SetFeaturedProject(ctx, userID int64, workspaceIDs []int64, projectID string) (*Card, error)`
  - `ClearFeatured(ctx, userID int64, workspaceIDs []int64) (*Card, error)`
  - `Publish(ctx, userID int64, workspaceIDs []int64) (*Card, error)`

- [ ] **Step 1: 写失败测试**

Create `packages/backend/internal/businesscard/service_test.go` with a function-field fake repository and project/publisher/S3 fakes. Start with these tests:

```go
package businesscard

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

type fakeRepo struct {
	card            *CardRecord
	getOrCreateErr  error
	items           []ItemRecord
	addItemErr      error
	removeItemErr   error
	setFeaturedErr  error
	clearFeaturedErr error
	publishErr      error
}

func (f *fakeRepo) GetOrCreate(context.Context, int64) (*CardRecord, error) {
	if f.getOrCreateErr != nil { return nil, f.getOrCreateErr }
	if f.card == nil { f.card = &CardRecord{ID: 1, PublicID: "card-1", UserID: 7, OwnerName: "alice", Tags: []string{}} }
	f.card.Items = append([]ItemRecord(nil), f.items...)
	return f.card, nil
}
func (f *fakeRepo) GetByUser(context.Context, int64) (*CardRecord, error) { return f.GetOrCreate(context.Background(), 0) }
func (f *fakeRepo) GetByPublicID(context.Context, string) (*CardRecord, error) {
	if f.card == nil { return nil, ErrNotFound }
	f.card.Items = append([]ItemRecord(nil), f.items...)
	return f.card, nil
}
func (f *fakeRepo) UpdateProfile(_ context.Context, _ int64, headline string, tags []string) error {
	f.card.Headline = headline
	f.card.Tags = tags
	return nil
}
func (f *fakeRepo) ListItems(context.Context, int64) ([]ItemRecord, error) { return f.items, nil }
func (f *fakeRepo) AddItem(_ context.Context, _ int64, projectID string, sortOrder int) error {
	if f.addItemErr != nil { return f.addItemErr }
	f.items = append(f.items, ItemRecord{ProjectID: projectID, SortOrder: sortOrder})
	return nil
}
func (f *fakeRepo) RemoveItem(_ context.Context, _ int64, projectID string) error {
	if f.removeItemErr != nil { return f.removeItemErr }
	out := f.items[:0]
	for _, item := range f.items { if item.ProjectID != projectID { out = append(out, item) } }
	f.items = out
	return nil
}
func (f *fakeRepo) SetFeaturedProject(_ context.Context, _ int64, projectID string) error {
	if f.setFeaturedErr != nil { return f.setFeaturedErr }
	f.card.FeaturedType = "project"
	f.card.FeaturedProjectID = projectID
	return nil
}
func (f *fakeRepo) SetFeaturedFile(_ context.Context, _ int64, key, name, fileType string, size int64) error {
	f.card.FeaturedType = fileType
	f.card.FeaturedS3Key = key
	f.card.FeaturedFileName = name
	f.card.FeaturedFileSize = size
	return nil
}
func (f *fakeRepo) ClearFeatured(context.Context, int64) error {
	if f.clearFeaturedErr != nil { return f.clearFeaturedErr }
	f.card.FeaturedType = ""
	f.card.FeaturedProjectID = ""
	f.card.FeaturedS3Key = ""
	return nil
}
func (f *fakeRepo) Publish(_ context.Context, _ int64, at time.Time) error {
	if f.publishErr != nil { return f.publishErr }
	f.card.Published = true
	f.card.PublishedAt = &at
	return nil
}
func (f *fakeRepo) SetHidden(_ context.Context, _ int64, hidden bool) error { f.card.Hidden = hidden; return nil }
func (f *fakeRepo) Delete(context.Context, int64) (string, error) { return f.card.FeaturedS3Key, nil }
func (f *fakeRepo) Discover(context.Context, int, int) ([]*CardRecord, int64, error) { return nil, 0, nil }
func (f *fakeRepo) AdminList(context.Context, string, int, int) ([]*CardRecord, int64, error) { return nil, 0, nil }
func (f *fakeRepo) FindByProject(context.Context, string) (*CardRecord, error) { return f.card, nil }
func (f *fakeRepo) OwnerProfile(context.Context, int64) (*OwnerProfile, error) {
	return &OwnerProfile{UserID: 7, Username: "alice"}, nil
}

type fakeProjects struct{ items map[string]*model.Project }

func (f fakeProjects) Get(_ context.Context, id string) (*model.Project, error) {
	project, ok := f.items[id]
	if !ok { return nil, errors.New("project not found") }
	return project, nil
}

type fakePublisher struct{ items []publish.Published }

func (f fakePublisher) List(context.Context) ([]publish.Published, error) { return f.items, nil }

type fakeS3 struct {
	putKey   string
	deleted  []string
	presign  string
}

func (f *fakeS3) Put(_ context.Context, key, _ string, _ int64, _ io.Reader) error { f.putKey = key; return nil }
func (f *fakeS3) PresignGet(context.Context, string, time.Duration) (string, error) { return f.presign, nil }
func (f *fakeS3) Delete(_ context.Context, key string) error { f.deleted = append(f.deleted, key); return nil }

func newTestService(repo Repository) (*Service, fakeProjects, fakePublisher, *fakeS3) {
	projects := fakeProjects{items: map[string]*model.Project{
		"p1": {ID: "p1", Name: "作品一", WorkspaceID: 10, OwnerUserID: 7, PreviewPath: "index.html", CoverURL: "https://cdn/p1.png"},
	}}
	published := fakePublisher{items: []publish.Published{{ProjectID: "p1", Title: "作品一", Description: "介绍"}}}
	s3 := &fakeS3{presign: "https://s3/card.pdf"}
	svc := NewService(repo, projects, published, s3, "https://play.example.com", 15*time.Minute)
	svc.Now = func() time.Time { return time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC) }
	svc.NewID = func() string { return "fixed-id" }
	return svc, projects, published, s3
}

func TestGetMineResolvesProjectItems(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p1", SortOrder: 0}}}
	svc, _, _, _ := newTestService(repo)
	card, err := svc.GetMine(context.Background(), 7, []int64{10})
	if err != nil { t.Fatal(err) }
	if card.ID != "card-1" || card.OwnerName != "alice" {
		t.Fatalf("card = %+v", card)
	}
	if len(card.Items) != 1 || card.Items[0].ShareURL != "https://play.example.com/p/p1/" || card.Items[0].Type != "html" {
		t.Fatalf("items = %+v", card.Items)
	}
}

func TestAddProjectRejectsUnpublished(t *testing.T) {
	repo := &fakeRepo{}
	svc, projects, published, _ := newTestService(repo)
	projects.items["p2"] = &model.Project{ID: "p2", WorkspaceID: 10}
	_ = published
	_, err := svc.AddProject(context.Background(), 7, []int64{10}, "p2")
	if !errors.Is(err, ErrInvalidProject) { t.Fatalf("err = %v", err) }
}

func TestAddProjectRejectsSixthItem(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p1"}}}
	repo.items = append(repo.items, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"}, ItemRecord{ProjectID: "p1"})
	svc, _, _, _ := newTestService(repo)
	_, err := svc.AddProject(context.Background(), 7, []int64{10}, "p1")
	if !errors.Is(err, ErrItemLimit) { t.Fatalf("err = %v", err) }
}

func TestPublishRequiresVisibleProject(t *testing.T) {
	repo := &fakeRepo{items: []ItemRecord{{ProjectID: "p2"}}}
	svc, _, _, _ := newTestService(repo)
	_, err := svc.Publish(context.Background(), 7, []int64{10})
	if !errors.Is(err, ErrNoItems) { t.Fatalf("err = %v", err) }
}
```

Add `io` to imports. Run:

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: FAIL — Service/类型未定义。

- [ ] **Step 2: 写 service.go（核心部分）**

Create `packages/backend/internal/businesscard/service.go`。本任务先写到 `Publish`，Task 4/5 再补方法。核心代码：

```go
package businesscard

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"vibebiz/internal/model"
	"vibebiz/internal/publish"
)

var (
	ErrInvalidInput       = errors.New("invalid business card input")
	ErrInvalidProject     = errors.New("project cannot be added to card")
	ErrItemLimit          = errors.New("business card item limit reached")
	ErrItemDuplicate      = errors.New("project already in card")
	ErrFeaturedNotFound   = errors.New("featured project is not in card")
	ErrNoItems            = errors.New("business card has no visible projects")
	ErrStorageUnavailable = errors.New("card storage is not configured")
	ErrInvalidFile        = errors.New("invalid featured file")
	ErrFileTooLarge       = errors.New("featured file is too large")
)

type ProjectReader interface {
	Get(ctx context.Context, id string) (*model.Project, error)
}

type Publisher interface {
	List(ctx context.Context) ([]publish.Published, error)
}

type S3 interface {
	Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
	Delete(ctx context.Context, key string) error
}

type Service struct {
	Repo             Repository
	Projects         ProjectReader
	Publish          Publisher
	S3               S3
	PublicBaseURL    string
	PresignExpiry    time.Duration
	MaxFeaturedBytes int64
	Now              func() time.Time
	NewID            func() string
}

func NewService(repo Repository, projects ProjectReader, publisher Publisher, s3 S3, publicBaseURL string, presignExpiry time.Duration) *Service {
	if presignExpiry <= 0 { presignExpiry = 15 * time.Minute }
	return &Service{
		Repo: repo, Projects: projects, Publish: publisher, S3: s3,
		PublicBaseURL: publicBaseURL, PresignExpiry: presignExpiry,
		MaxFeaturedBytes: 2 << 20, Now: time.Now, NewID: uuid.NewString,
	}
}

type Card struct {
	ID           string       `json:"id"`
	OwnerUserID  int64        `json:"ownerUserId"`
	OwnerName    string       `json:"ownerName"`
	AvatarURL    string       `json:"avatarUrl"`
	Headline     string       `json:"headline"`
	Tags         []string     `json:"tags"`
	Items        []CardItem   `json:"items"`
	Featured     *Featured    `json:"featured"`
	Published    bool         `json:"published"`
	CommentCount int          `json:"commentCount"`
	CreatedAt    time.Time    `json:"createdAt"`
	UpdatedAt    time.Time    `json:"updatedAt"`
}

type CardItem struct {
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CoverURL    string `json:"coverUrl"`
	Type        string `json:"type"`
	ShareURL    string `json:"shareUrl"`
}

type Featured struct {
	Kind      string `json:"kind"`
	ProjectID string `json:"projectId,omitempty"`
	FileType  string `json:"fileType,omitempty"`
	Name      string `json:"name,omitempty"`
	Size      int64  `json:"size,omitempty"`
	URL       string `json:"url,omitempty"`
}

func (s *Service) GetMine(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	return s.resolve(ctx, rec, workspaceSet(workspaceIDs), true)
}

func (s *Service) GetPublic(ctx context.Context, publicID string) (*Card, error) {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil { return nil, ErrNotFound }
	if rec.Hidden { return nil, ErrNotFound }
	return s.resolve(ctx, rec, nil, false)
}

func (s *Service) UpdateProfile(ctx context.Context, userID int64, workspaceIDs []int64, headline string, tags []string) (*Card, error) {
	headline = strings.TrimSpace(headline)
	if utf8.RuneCountInString(headline) > 80 { return nil, ErrInvalidInput }
	normalized, err := normalizeTags(tags)
	if err != nil { return nil, err }
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	if err := s.Repo.UpdateProfile(ctx, rec.ID, headline, normalized); err != nil { return nil, err }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) AddProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	items, err := s.Repo.ListItems(ctx, rec.ID)
	if err != nil { return nil, err }
	if len(items) >= 5 { return nil, ErrItemLimit }
	for _, item := range items { if item.ProjectID == projectID { return nil, ErrItemDuplicate } }
	if _, _, err := s.validateProject(ctx, projectID, workspaceIDs, true); err != nil { return nil, err }
	sortOrder := 0
	for _, item := range items { if item.SortOrder >= sortOrder { sortOrder = item.SortOrder + 1 } }
	if err := s.Repo.AddItem(ctx, rec.ID, projectID, sortOrder); err != nil { return nil, err }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) RemoveProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	if err := s.Repo.RemoveItem(ctx, rec.ID, projectID); err != nil { return nil, err }
	if rec.FeaturedType == "project" && rec.FeaturedProjectID == projectID {
		if err := s.Repo.ClearFeatured(ctx, rec.ID); err != nil { return nil, err }
	}
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) SetFeaturedProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	items, err := s.Repo.ListItems(ctx, rec.ID)
	if err != nil { return nil, err }
	found := false
	for _, item := range items { if item.ProjectID == projectID { found = true; break } }
	if !found { return nil, ErrFeaturedNotFound }
	oldKey := rec.FeaturedS3Key
	if err := s.Repo.SetFeaturedProject(ctx, rec.ID, projectID); err != nil { return nil, err }
	if oldKey != "" && s.S3 != nil { _ = s.S3.Delete(ctx, oldKey) }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) ClearFeatured(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	oldKey := rec.FeaturedS3Key
	if err := s.Repo.ClearFeatured(ctx, rec.ID); err != nil { return nil, err }
	if oldKey != "" && s.S3 != nil { _ = s.S3.Delete(ctx, oldKey) }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) Publish(ctx context.Context, userID int64, workspaceIDs []int64) (*Card, error) {
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	card, err := s.resolve(ctx, rec, workspaceSet(workspaceIDs), true)
	if err != nil { return nil, err }
	if len(card.Items) == 0 { return nil, ErrNoItems }
	if err := s.Repo.Publish(ctx, rec.ID, s.now()); err != nil { return nil, err }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func (s *Service) resolve(ctx context.Context, rec *CardRecord, allowed map[int64]bool, enforce bool) (*Card, error) {
	published, err := s.publishMap(ctx)
	if err != nil { return nil, err }
	items := make([]CardItem, 0, len(rec.Items))
	for _, item := range rec.Items {
		project, err := s.Projects.Get(ctx, item.ProjectID)
		if err != nil || project.IsHidden { continue }
		if enforce && !allowed[project.WorkspaceID] { continue }
		pub, ok := published[item.ProjectID]
		if !ok { continue }
		items = append(items, s.cardItem(project, pub))
	}
	tags := rec.Tags
	if tags == nil { tags = []string{} }
	return &Card{
		ID: rec.PublicID, OwnerUserID: rec.UserID, OwnerName: rec.OwnerName,
		AvatarURL: s.avatarURL(ctx, rec.OwnerAvatarS3Path), Headline: rec.Headline, Tags: tags,
		Items: items, Featured: s.resolveFeatured(ctx, rec, items), Published: rec.Published,
		CommentCount: rec.CommentCount, CreatedAt: rec.CreatedAt, UpdatedAt: rec.UpdatedAt,
	}, nil
}

func (s *Service) cardItem(project *model.Project, pub publish.Published) CardItem {
	name := pub.Title
	if name == "" { name = project.Name }
	cover := project.CoverURL
	if project.CoverS3Path != "" && s.S3 != nil {
		if signed, err := s.S3.PresignGet(context.Background(), project.CoverS3Path, s.resizeExpiry()); err == nil { cover = signed }
	}
	return CardItem{
		ProjectID: project.ID, Name: name, Description: pub.Description,
		CoverURL: cover, Type: contentKind(project.PreviewPath),
		ShareURL: strings.TrimRight(s.PublicBaseURL, "/") + "/p/" + project.ID + "/",
	}
}

func (s *Service) resolveFeatured(ctx context.Context, rec *CardRecord, items []CardItem) *Featured {
	switch rec.FeaturedType {
	case "project":
		for _, item := range items {
			if item.ProjectID == rec.FeaturedProjectID { return &Featured{Kind: "project", ProjectID: item.ProjectID} }
		}
	case "pdf", "image":
		if rec.FeaturedS3Key == "" || s.S3 == nil { return nil }
		url, err := s.S3.PresignGet(ctx, rec.FeaturedS3Key, s.resizeExpiry())
		if err != nil { return nil }
		return &Featured{Kind: "file", FileType: rec.FeaturedType, Name: rec.FeaturedFileName, Size: rec.FeaturedFileSize, URL: url}
	}
	return nil
}

func (s *Service) avatarURL(ctx context.Context, key string) string {
	if key == "" || s.S3 == nil { return "" }
	url, err := s.S3.PresignGet(ctx, key, s.resizeExpiry())
	if err != nil { return "" }
	return url
}

func (s *Service) publishMap(ctx context.Context) (map[string]publish.Published, error) {
	items, err := s.Publish.List(ctx)
	if err != nil { return nil, err }
	out := make(map[string]publish.Published, len(items))
	for _, item := range items { out[item.ProjectID] = item }
	return out, nil
}

func (s *Service) validateProject(ctx context.Context, projectID string, workspaceIDs []int64, enforce bool) (*model.Project, publish.Published, error) {
	project, err := s.Projects.Get(ctx, projectID)
	if err != nil || project.IsHidden { return nil, publish.Published{}, ErrInvalidProject }
	if enforce && !workspaceSet(workspaceIDs)[project.WorkspaceID] { return nil, publish.Published{}, ErrInvalidProject }
	published, err := s.publishMap(ctx)
	if err != nil { return nil, publish.Published{}, err }
	pub, ok := published[projectID]
	if !ok { return nil, publish.Published{}, ErrInvalidProject }
	return project, pub, nil
}

func (s *Service) now() time.Time { if s.Now != nil { return s.Now().UTC() }; return time.Now().UTC() }
func (s *Service) nextID() string { if s.NewID != nil { return s.NewID() }; return uuid.NewString() }
func (s *Service) resizeExpiry() time.Duration { if s.PresignExpiry > 0 { return s.PresignExpiry }; return 15 * time.Minute }

func workspaceSet(ids []int64) map[int64]bool {
	out := make(map[int64]bool, len(ids))
	for _, id := range ids { out[id] = true }
	return out
}

func normalizeTags(tags []string) ([]string, error) {
	out := make([]string, 0, 5)
	seen := map[string]bool{}
	for _, raw := range tags {
		tag := strings.TrimSpace(raw)
		if tag == "" { continue }
		if utf8.RuneCountInString(tag) > 12 { return nil, ErrInvalidInput }
		if seen[tag] { continue }
		seen[tag] = true
		out = append(out, tag)
	}
	if len(out) > 5 { return nil, ErrInvalidInput }
	return out, nil
}

func contentKind(previewPath string) string {
	name := strings.ToLower(strings.TrimSpace(previewPath))
	if i := strings.IndexAny(name, "?#"); i >= 0 { name = name[:i] }
	switch {
	case strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".htm"): return "html"
	case strings.HasSuffix(name, ".pdf"): return "pdf"
	case strings.HasSuffix(name, ".mp4") || strings.HasSuffix(name, ".webm") || strings.HasSuffix(name, ".mov"): return "video"
	case strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown") || strings.HasSuffix(name, ".txt") || strings.HasSuffix(name, ".doc") || strings.HasSuffix(name, ".docx"): return "document"
	default:
		ext := strings.TrimPrefix(filepath.Ext(name), ".")
		switch ext {
		case "png", "jpg", "jpeg", "webp", "gif", "svg": return "image"
		}
	}
	return "html"
}
```

`UploadFeatured` 与 `detectFeatured` 在 Task 4 加入；`Discover/Admin/PublicProject` 在 Task 5 加入。本任务测试只覆盖已实现方法；`fakeRepo` 的未用方法可以保留。

注意：`cardItem` 内使用 `context.Background()` 会丢失请求上下文；实现时把 `ctx` 传进 `cardItem(ctx, project, pub)`，避免超时/取消丢失。

- [ ] **Step 3: 跑测试**

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: PASS（当前测试只覆盖核心方法）。

- [ ] **Step 4: Commit**

```bash
git add packages/backend/internal/businesscard/service.go packages/backend/internal/businesscard/service_test.go
git commit -m "feat(backend): add business card core service"
```

---

### Task 4: 主预览文件上传（S3）

**Files:**
- Modify: `packages/backend/internal/businesscard/service.go`
- Modify: `packages/backend/internal/businesscard/service_test.go`

**Interfaces:**
- Produces: `UploadFeatured(ctx, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*Card, error)`
- 内部：`detectFeatured(data []byte, contentType string) (fileType, ext string, ok bool)`、`sanitizeFilename(name string) string`。

- [ ] **Step 1: 写失败测试**

Append to `service_test.go`:

```go
func TestUploadFeaturedRejectsMismatchedMagic(t *testing.T) {
	repo := &fakeRepo{}
	svc, _, _, _ := newTestService(repo)
	body := strings.NewReader("<html>not a pdf</html>")
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "x.pdf", int64(body.Len()), body)
	if !errors.Is(err, ErrInvalidFile) { t.Fatalf("err = %v", err) }
}

func TestUploadFeaturedStoresS3KeyAndDeletesOld(t *testing.T) {
	repo := &fakeRepo{card: &CardRecord{ID: 1, PublicID: "card-1", UserID: 7, FeaturedS3Key: "cards/7/old.pdf"}}
	svc, _, _, s3 := newTestService(repo)
	data := []byte("%PDF-1.4\n")
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "resume.pdf", int64(len(data)), bytes.NewReader(data))
	if err != nil { t.Fatal(err) }
	if s3.putKey != "cards/7/fixed-id.pdf" { t.Fatalf("key = %q", s3.putKey) }
	if len(s3.deleted) != 1 || s3.deleted[0] != "cards/7/old.pdf" { t.Fatalf("deleted = %+v", s3.deleted) }
}

func TestUploadFeaturedRejectsTooLarge(t *testing.T) {
	repo := &fakeRepo{}
	svc, _, _, _ := newTestService(repo)
	svc.MaxFeaturedBytes = 4
	_, err := svc.UploadFeatured(context.Background(), 7, []int64{10}, "application/pdf", "x.pdf", 5, strings.NewReader("%PDF-"))
	if !errors.Is(err, ErrFileTooLarge) { t.Fatalf("err = %v", err) }
}
```

Add imports `bytes`, `strings`.

- [ ] **Step 2: 实现**

在 `service.go` 追加（并补 `"bytes"`、`"fmt"` 两个 import）：

```go
func (s *Service) UploadFeatured(ctx context.Context, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*Card, error) {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if size <= 0 || size > s.MaxFeaturedBytes { return nil, ErrFileTooLarge }
	rec, err := s.Repo.GetOrCreate(ctx, userID)
	if err != nil { return nil, err }
	data, err := io.ReadAll(io.LimitReader(body, size+1))
	if err != nil { return nil, err }
	if int64(len(data)) != size { return nil, ErrInvalidFile }
	fileType, ext, ok := detectFeatured(data, contentType)
	if !ok { return nil, ErrInvalidFile }
	if s.S3 == nil { return nil, ErrStorageUnavailable }
	key := fmt.Sprintf("cards/%d/%s.%s", userID, s.nextID(), ext)
	if err := s.S3.Put(ctx, key, contentType, size, bytes.NewReader(data)); err != nil { return nil, err }
	if err := s.Repo.SetFeaturedFile(ctx, rec.ID, key, sanitizeFilename(filename), fileType, size); err != nil {
		_ = s.S3.Delete(ctx, key)
		return nil, err
	}
	if rec.FeaturedS3Key != "" && rec.FeaturedS3Key != key { _ = s.S3.Delete(ctx, rec.FeaturedS3Key) }
	return s.GetMine(ctx, userID, workspaceIDs)
}

func detectFeatured(data []byte, contentType string) (string, string, bool) {
	switch contentType {
	case "application/pdf":
		if bytes.HasPrefix(data, []byte("%PDF-")) { return "pdf", "pdf", true }
	case "image/png":
		if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) { return "image", "png", true }
	case "image/jpeg":
		if bytes.HasPrefix(data, []byte{0xff, 0xd8, 0xff}) { return "image", "jpg", true }
	case "image/gif":
		if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) { return "image", "gif", true }
	case "image/webp":
		if len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP" { return "image", "webp", true }
	}
	return "", "", false
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(filepath.Base(name))
	if name == "." || name == "/" { name = "" }
	runes := []rune(name)
	if len(runes) > 255 { name = string(runes[:255]) }
	return name
}
```

- [ ] **Step 3: 跑测试**

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add packages/backend/internal/businesscard/service.go packages/backend/internal/businesscard/service_test.go
git commit -m "feat(backend): add card featured upload"
```

---

### Task 5: 发现、公开项目、ops 管理

**Files:**
- Modify: `packages/backend/internal/businesscard/service.go`
- Modify: `packages/backend/internal/businesscard/service_test.go`

**Interfaces:**
- Produces:
  - `Discover(ctx, page, pageSize int) (*DiscoverResult, error)`
  - `AdminList(ctx, keyword string, page, pageSize int) (*AdminListResult, error)`
  - `AdminSetHidden(ctx, publicID string, hidden bool) (*AdminRow, error)`
  - `AdminDelete(ctx, publicID string) error`
  - `PublicProject(ctx, projectID string) (*PublicProject, error)`
  - 类型 `DiscoverResult`、`AdminListResult`、`AdminRow`、`PublicProject`、`PublicCardSummary`

- [ ] **Step 1: 写失败测试**

在 `service_test.go` 的 `fakeRepo` 增加字段并实现：

```go
discoverRecords []*CardRecord
discoverTotal   int64
adminRecords    []*CardRecord
adminTotal      int64
```

```go
func (f *fakeRepo) Discover(context.Context, int, int) ([]*CardRecord, int64, error) {
	return f.discoverRecords, f.discoverTotal, nil
}
func (f *fakeRepo) AdminList(context.Context, string, int, int) ([]*CardRecord, int64, error) {
	return f.adminRecords, f.adminTotal, nil
}
```

新增测试：

```go
func TestDiscoverFiltersCardsWithoutVisibleItems(t *testing.T) {
	repo := &fakeRepo{
		discoverRecords: []*CardRecord{
			{ID: 1, PublicID: "c1", UserID: 7, OwnerName: "alice", Items: []ItemRecord{{ProjectID: "p1"}}},
			{ID: 2, PublicID: "c2", UserID: 8, OwnerName: "bob"},
		},
		discoverTotal: 2,
	}
	svc, _, _, _ := newTestService(repo)
	result, err := svc.Discover(context.Background(), 1, 12)
	if err != nil { t.Fatal(err) }
	if len(result.Cards) != 1 || result.Cards[0].ID != "c1" {
		t.Fatalf("cards = %+v", result.Cards)
	}
}

func TestAdminListBuildsStatusAndFeaturedLabel(t *testing.T) {
	repo := &fakeRepo{adminRecords: []*CardRecord{
		{ID: 1, PublicID: "c1", OwnerName: "alice", FeaturedType: "pdf", Published: true},
		{ID: 2, PublicID: "c2", OwnerName: "bob", Hidden: true},
		{ID: 3, PublicID: "c3", OwnerName: "carol"},
	}, adminTotal: 3}
	svc, _, _, _ := newTestService(repo)
	result, err := svc.AdminList(context.Background(), "", 1, 10)
	if err != nil { t.Fatal(err) }
	if result.Rows[0].Status != "正常" || result.Rows[0].FeaturedType != "PDF" {
		t.Fatalf("row0 = %+v", result.Rows[0])
	}
	if result.Rows[1].Status != "已隐藏" { t.Fatalf("row1 = %+v", result.Rows[1]) }
	if result.Rows[2].Status != "未发布" { t.Fatalf("row2 = %+v", result.Rows[2]) }
}

func TestAdminDeleteDeletesFeaturedS3Object(t *testing.T) {
	repo := &fakeRepo{card: &CardRecord{ID: 1, PublicID: "c1", FeaturedS3Key: "cards/7/a.pdf"}}
	svc, _, _, s3 := newTestService(repo)
	if err := svc.AdminDelete(context.Background(), "c1"); err != nil { t.Fatal(err) }
	if len(s3.deleted) != 1 || s3.deleted[0] != "cards/7/a.pdf" { t.Fatalf("deleted = %+v", s3.deleted) }
}

func TestPublicProjectFallsBackToProjectOwner(t *testing.T) {
	repo := &fakeRepo{}
	repo.card = nil
	svc, _, _, _ := newTestService(repo)
	svc.Repo.(*fakeRepo).FindByProjectFn = nil
	// fakeRepo.FindByProject 默认返回 nil card；覆盖为 NotFound 以走兜底
	repo.FindByProjectFn = func(context.Context, string) (*CardRecord, error) { return nil, ErrNotFound }
	project, err := svc.PublicProject(context.Background(), "p1")
	if err != nil { t.Fatal(err) }
	if project.Item.ProjectID != "p1" || project.Card.OwnerName != "alice" {
		t.Fatalf("project = %+v", project)
	}
}
```

为此在 `fakeRepo` 增加：

```go
FindByProjectFn func(context.Context, string) (*CardRecord, error)
```

并把 `FindByProject` 改为：

```go
func (f *fakeRepo) FindByProject(ctx context.Context, projectID string) (*CardRecord, error) {
	if f.FindByProjectFn != nil { return f.FindByProjectFn(ctx, projectID) }
	return f.card, nil
}
```

Run:

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: FAIL — 方法/类型未定义。

- [ ] **Step 2: 实现**

在 `service.go` 追加：

```go
type DiscoverResult struct {
	Cards    []Card `json:"cards"`
	Total    int64  `json:"total"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

type AdminRow struct {
	ID           string    `json:"id"`
	OwnerUserID  int64     `json:"ownerUserId"`
	OwnerName    string    `json:"ownerName"`
	Headline     string    `json:"headline"`
	Tags         []string  `json:"tags"`
	ProjectCount int       `json:"projectCount"`
	FeaturedType string    `json:"featuredType"`
	Published    bool      `json:"published"`
	Hidden       bool      `json:"hidden"`
	Status       string    `json:"status"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type AdminListResult struct {
	Rows     []AdminRow `json:"items"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

type PublicCardSummary struct {
	ID        string `json:"id"`
	OwnerName string `json:"ownerName"`
	AvatarURL string `json:"avatarUrl"`
}

type PublicProject struct {
	Card PublicCardSummary `json:"card"`
	Item CardItem          `json:"item"`
}

func (s *Service) Discover(ctx context.Context, page, pageSize int) (*DiscoverResult, error) {
	page, pageSize = normalizePage(page, pageSize, 12, 50)
	records, total, err := s.Repo.Discover(ctx, page, pageSize)
	if err != nil { return nil, err }
	cards := make([]Card, 0, len(records))
	for _, rec := range records {
		card, err := s.resolve(ctx, rec, nil, false)
		if err != nil { return nil, err }
		if len(card.Items) == 0 { continue }
		cards = append(cards, *card)
	}
	return &DiscoverResult{Cards: cards, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) AdminList(ctx context.Context, keyword string, page, pageSize int) (*AdminListResult, error) {
	page, pageSize = normalizePage(page, pageSize, 10, 100)
	records, total, err := s.Repo.AdminList(ctx, keyword, page, pageSize)
	if err != nil { return nil, err }
	rows := make([]AdminRow, 0, len(records))
	for _, rec := range records {
		tags := rec.Tags
		if tags == nil { tags = []string{} }
		rows = append(rows, AdminRow{
			ID: rec.PublicID, OwnerUserID: rec.UserID, OwnerName: rec.OwnerName,
			Headline: rec.Headline, Tags: tags, ProjectCount: len(rec.Items),
			FeaturedType: featuredLabel(rec.FeaturedType),
			Published: rec.Published, Hidden: rec.Hidden, Status: cardStatus(rec),
			UpdatedAt: rec.UpdatedAt,
		})
	}
	return &AdminListResult{Rows: rows, Total: total, Page: page, PageSize: pageSize}, nil
}

func (s *Service) AdminSetHidden(ctx context.Context, publicID string, hidden bool) (*AdminRow, error) {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil { return nil, ErrNotFound }
	if err := s.Repo.SetHidden(ctx, rec.ID, hidden); err != nil { return nil, err }
	return &AdminRow{ID: rec.PublicID, Hidden: hidden, Status: cardStatus(&CardRecord{Hidden: hidden, Published: rec.Published})}, nil
}

func (s *Service) AdminDelete(ctx context.Context, publicID string) error {
	rec, err := s.Repo.GetByPublicID(ctx, publicID)
	if err != nil { return ErrNotFound }
	key, err := s.Repo.Delete(ctx, rec.ID)
	if err != nil { return err }
	if key != "" && s.S3 != nil { _ = s.S3.Delete(ctx, key) }
	return nil
}

func (s *Service) PublicProject(ctx context.Context, projectID string) (*PublicProject, error) {
	project, pub, err := s.validateProject(ctx, projectID, nil, false)
	if err != nil { return nil, ErrNotFound }
	ownerName := ""
	avatar := ""
	cardID := ""
	if rec, err := s.Repo.FindByProject(ctx, projectID); err == nil && rec != nil {
		cardID = rec.PublicID
		ownerName = rec.OwnerName
		avatar = s.avatarURL(ctx, rec.OwnerAvatarS3Path)
	} else if owner, err := s.Repo.OwnerProfile(ctx, project.OwnerUserID); err == nil {
		ownerName = owner.Nickname
		if ownerName == "" { ownerName = owner.Username }
		avatar = s.avatarURL(ctx, owner.AvatarS3Path)
	}
	return &PublicProject{
		Card: PublicCardSummary{ID: cardID, OwnerName: ownerName, AvatarURL: avatar},
		Item: s.cardItem(ctx, project, pub),
	}, nil
}

func normalizePage(page, pageSize, defaultSize, maxSize int) (int, int) {
	if page < 1 { page = 1 }
	if pageSize <= 0 { pageSize = defaultSize }
	if pageSize > maxSize { pageSize = maxSize }
	return page, pageSize
}

func featuredLabel(value string) string {
	switch value {
	case "project": return "项目"
	case "pdf": return "PDF"
	case "image": return "图片"
	default: return "—"
	}
}

func cardStatus(rec *CardRecord) string {
	if rec.Hidden { return "已隐藏" }
	if !rec.Published { return "未发布" }
	return "正常"
}
```

注意：`cardItem` 需要 `ctx`，按 Task 3 的修正把签名改成 `cardItem(ctx, project, pub)`，并在 `resolve`/`PublicProject` 传入当前 ctx。

- [ ] **Step 3: 跑测试**

```bash
cd packages/backend && go test ./internal/businesscard
```

Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add packages/backend/internal/businesscard/service.go packages/backend/internal/businesscard/service_test.go
git commit -m "feat(backend): add card discovery, public project and admin service"
```

---

### Task 6: HTTP Handler、路由与 CP 装配

**Files:**
- Create: `packages/backend/internal/handler/card.go`
- Test: `packages/backend/internal/handler/card_test.go`
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/span_test.go`
- Modify: `packages/backend/cmd/cp/main.go`

**Interfaces:**
- Consumes: `businesscard.Service` 全部方法。
- Produces: `handler.NewCardHandler(service CardService, publicBaseURL string) *CardHandler`；所有 `/me/card`、`/cards/discover`、`/public/*`、`/admin/cards*` 路由；当 `publicBaseURL` 为空时用请求 scheme/host 补全项目 `shareUrl`。

- [ ] **Step 1: 写失败 handler 测试**

Create `packages/backend/internal/handler/card_test.go`。用 `fakeCardService`（实现 `CardService`，每个方法一个函数字段）覆盖成功、校验错误与路由边界：

```go
func TestCardMineGetAndUpdate(t *testing.T) {
	e, _ := newCardEcho(t)
	// GET /cp/api/v1/me/card
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/cp/api/v1/me/card", nil)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"card-1"`) {
		t.Fatalf("get status=%d body=%s", rec.Code, rec.Body.String())
	}

	// PUT /cp/api/v1/me/card
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/cp/api/v1/me/card", strings.NewReader(`{"headline":"前端工程师","tags":["Vue","Vue"]}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"headline":"前端工程师"`) {
		t.Fatalf("put status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardAddProjectValidation(t *testing.T) {
	e, _ := newCardEcho(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/me/card/items", strings.NewReader(`{"projectId":"missing"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":40016`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardUploadTooLarge(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.uploadFeatured = func(context.Context, int64, []int64, string, string, int64, io.Reader) (*businesscard.Card, error) {
		return nil, businesscard.ErrFileTooLarge
	}
	body := strings.NewReader("%PDF-12345")
	req := httptest.NewRequest(http.MethodPost, "/cp/api/v1/me/card/featured/upload", body)
	req.Header.Set(echo.HeaderContentType, "application/pdf")
	req.Header.Set("X-File-Name", "a.pdf")
	req.ContentLength = 11
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCardPublicAndAdminRoutes(t *testing.T) {
	e, _ := newCardEcho(t)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if rec.Code != http.StatusOK { t.Fatalf("public status=%d", rec.Code) }

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/admin/cards", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total"`) {
		t.Fatalf("admin status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

`newCardEcho(t) (*echo.Echo, *fakeCardService)` 构造 echo、注册路由（`e.GET("/cp/api/v1/me/card", ...)` 等），并用 `sessionForTest(7)` 给用户路由注入会话。参考 `handler/leaderboard_test.go` 的 `routeWithSession`。`fakeCardService` 为每个 `CardService` 方法提供函数字段；默认值满足上述成功用例（`GetMine`/`UpdateProfile` 返回 card-1，`AddProject` 返回 `ErrInvalidProject`，`AdminList` 返回空页），测试按需覆盖。

Run:

```bash
cd packages/backend && go test ./internal/handler -run Card
```

Expected: FAIL — handler/路由不存在。

- [ ] **Step 2: 实现 handler**

Create `packages/backend/internal/handler/card.go`：

```go
package handler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/auth"
	"vibebiz/internal/businesscard"
	"vibebiz/internal/response"
)

type CardService interface {
	GetMine(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	UpdateProfile(ctx context.Context, userID int64, workspaceIDs []int64, headline string, tags []string) (*businesscard.Card, error)
	AddProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	RemoveProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	SetFeaturedProject(ctx context.Context, userID int64, workspaceIDs []int64, projectID string) (*businesscard.Card, error)
	UploadFeatured(ctx context.Context, userID int64, workspaceIDs []int64, contentType, filename string, size int64, body io.Reader) (*businesscard.Card, error)
	ClearFeatured(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	Publish(ctx context.Context, userID int64, workspaceIDs []int64) (*businesscard.Card, error)
	Discover(ctx context.Context, page, pageSize int) (*businesscard.DiscoverResult, error)
	GetPublic(ctx context.Context, publicID string) (*businesscard.Card, error)
	PublicProject(ctx context.Context, projectID string) (*businesscard.PublicProject, error)
	AdminList(ctx context.Context, keyword string, page, pageSize int) (*businesscard.AdminListResult, error)
	AdminSetHidden(ctx context.Context, publicID string, hidden bool) (*businesscard.AdminRow, error)
	AdminDelete(ctx context.Context, publicID string) error
}

type CardHandler struct {
	Service       CardService
	PublicBaseURL string
}

func NewCardHandler(service CardService, publicBaseURL string) *CardHandler {
	return &CardHandler{Service: service, PublicBaseURL: publicBaseURL}
}

func (h *CardHandler) GetMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	card, err := h.Service.GetMine(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) UpdateMine(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	var req struct {
		Headline string   `json:"headline"`
		Tags     []string `json:"tags"`
	}
	if err := c.Bind(&req); err != nil { return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body") }
	card, err := h.Service.UpdateProfile(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.Headline, req.Tags)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) AddItem(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	var req struct{ ProjectID string `json:"projectId"` }
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.ProjectID) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.AddProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.ProjectID)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) RemoveItem(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	card, err := h.Service.RemoveProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, c.Param("projectId"))
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) SetFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	var req struct {
		Kind      string `json:"kind"`
		ProjectID string `json:"projectId"`
	}
	if err := c.Bind(&req); err != nil || req.Kind != "project" || strings.TrimSpace(req.ProjectID) == "" {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	card, err := h.Service.SetFeaturedProject(c.Request().Context(), session.UserID, session.WorkspaceIDs, req.ProjectID)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) UploadFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	contentType := strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType))
	name := c.Request().Header.Get("X-File-Name")
	if decoded, err := url.QueryUnescape(name); err == nil { name = decoded }
	if strings.TrimSpace(name) == "" { name = "featured" }
	size := c.Request().ContentLength
	if size <= 0 { return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body") }
	card, err := h.Service.UploadFeatured(c.Request().Context(), session.UserID, session.WorkspaceIDs, contentType, name, size, c.Request().Body)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) ClearFeatured(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	card, err := h.Service.ClearFeatured(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) Publish(c echo.Context) error {
	session, ok := auth.SessionFrom(c)
	if !ok { return response.Fail(c, http.StatusUnauthorized, 40101, "unauthorized") }
	card, err := h.Service.Publish(c.Request().Context(), session.UserID, session.WorkspaceIDs)
	if err != nil { return cardError(c, err) }
	return response.OK(c, card)
}

func (h *CardHandler) Discover(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.Discover(c.Request().Context(), page, pageSize)
	if err != nil { return cardError(c, err) }
	return response.OK(c, result)
}

func (h *CardHandler) PublicCard(c echo.Context) error {
	card, err := h.Service.GetPublic(c.Request().Context(), c.Param("id"))
	if err != nil { return response.Fail(c, http.StatusNotFound, 40410, "card not found") }
	return response.OK(c, card)
}

func (h *CardHandler) PublicProject(c echo.Context) error {
	project, err := h.Service.PublicProject(c.Request().Context(), c.Param("projectId"))
	if err != nil { return response.Fail(c, http.StatusNotFound, 40411, "project not found") }
	return response.OK(c, project)
}

func (h *CardHandler) AdminList(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	pageSize, _ := strconv.Atoi(c.QueryParam("pageSize"))
	result, err := h.Service.AdminList(c.Request().Context(), c.QueryParam("keyword"), page, pageSize)
	if err != nil { return cardError(c, err) }
	return response.OK(c, result)
}

func (h *CardHandler) AdminHide(c echo.Context) error {
	var req struct{ Hidden bool `json:"hidden"` }
	if err := c.Bind(&req); err != nil { return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body") }
	row, err := h.Service.AdminSetHidden(c.Request().Context(), c.Param("id"), req.Hidden)
	if err != nil { return cardError(c, err) }
	return response.OK(c, map[string]any{"id": row.ID, "hidden": row.Hidden})
}

func (h *CardHandler) AdminDelete(c echo.Context) error {
	if err := h.Service.AdminDelete(c.Request().Context(), c.Param("id")); err != nil { return cardError(c, err) }
	return response.OK(c, map[string]string{"id": c.Param("id")})
}

func (h *CardHandler) okCard(c echo.Context, card *businesscard.Card) error {
	if card != nil {
		h.normalizeItems(c, card.Items)
	}
	return response.OK(c, card)
}

func (h *CardHandler) normalizeItems(c echo.Context, items []businesscard.CardItem) {
	base := leaderboardPublicBase(c, h.PublicBaseURL)
	for index := range items {
		if strings.HasPrefix(items[index].ShareURL, "/") {
			items[index].ShareURL = base + items[index].ShareURL
		}
	}
}

func cardError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, businesscard.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "card not found")
	case errors.Is(err, businesscard.ErrInvalidProject):
		return response.Fail(c, http.StatusBadRequest, 40016, "project cannot be added to card")
	case errors.Is(err, businesscard.ErrItemLimit), errors.Is(err, businesscard.ErrItemDuplicate),
		errors.Is(err, businesscard.ErrFeaturedNotFound), errors.Is(err, businesscard.ErrNoItems),
		errors.Is(err, businesscard.ErrInvalidInput), errors.Is(err, businesscard.ErrInvalidFile):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid card request")
	case errors.Is(err, businesscard.ErrFileTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41302, "featured file is too large")
	case errors.Is(err, businesscard.ErrStorageUnavailable):
		return response.Fail(c, http.StatusServiceUnavailable, 50304, "card storage is not configured")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "business card request failed")
	}
}
```

把返回 `Card` 的 `response.OK(c, card)` 全部改为 `h.okCard(c, card)`；`Discover` 渲染前遍历 `result.Cards` 调用 `h.normalizeItems(c, card.Items)`；`PublicProject` 调用 `h.normalizeItems(c, []businesscard.CardItem{project.Item})`，再把 `project.Item` 写回。

- [ ] **Step 3: 注册路由**

`internal/router/control.go`：在 `RegisterControl` 参数末尾加 `cardHandler *handler.CardHandler`。

在 `meAPI` 区域加入：

```go
if cardHandler != nil {
	meAPI.GET("/card", cardHandler.GetMine)
	meAPI.PUT("/card", cardHandler.UpdateMine)
	meAPI.POST("/card/items", cardHandler.AddItem)
	meAPI.DELETE("/card/items/:projectId", cardHandler.RemoveItem)
	meAPI.PUT("/card/featured", cardHandler.SetFeatured)
	meAPI.POST("/card/featured/upload", cardHandler.UploadFeatured)
	meAPI.DELETE("/card/featured", cardHandler.ClearFeatured)
	meAPI.POST("/card/publish", cardHandler.Publish)
}
```

在 `api` 区域加入：

```go
if cardHandler != nil {
	api.GET("/cards/discover", cardHandler.Discover, authn.RequireUser)
	api.GET("/public/cards/:id", cardHandler.PublicCard)
	api.GET("/public/card-projects/:projectId", cardHandler.PublicProject)
}
```

在 `adminProtected` 区域加入：

```go
if cardHandler != nil {
	adminProtected.GET("/cards", cardHandler.AdminList)
	adminProtected.PUT("/cards/:id/hidden", cardHandler.AdminHide)
	adminProtected.DELETE("/cards/:id", cardHandler.AdminDelete)
}
```

- [ ] **Step 4: 装配 CP**

`cmd/cp/main.go`：

```go
var cardS3 businesscard.S3
if s3Presigner != nil {
	cardS3 = s3Presigner
}
cardService := businesscard.NewService(
	businesscard.NewMySQLRepository(db),
	projectStore,
	publishService,
	cardS3,
	cfg.PublicBaseURL,
	cfg.S3PresignExpiry(),
)
cardHandler := handler.NewCardHandler(cardService, cfg.PublicBaseURL)
```

在 `router.RegisterControl(...)` 末尾追加 `cardHandler`。导入 `vibebiz/internal/businesscard`。

`internal/router/helpers_test.go` 与 `span_test.go` 的 `RegisterControl(...)` 调用末尾追加 `nil`。

- [ ] **Step 5: 跑测试**

```bash
cd packages/backend && go test ./internal/handler ./internal/router ./cmd/cp
cd packages/backend && go vet ./...
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/backend/internal/handler/card.go packages/backend/internal/handler/card_test.go packages/backend/internal/router/control.go packages/backend/internal/router/helpers_test.go packages/backend/internal/router/span_test.go packages/backend/cmd/cp/main.go
git commit -m "feat(backend): expose business card APIs"
```

---

### Task 7: Web 类型与卡片 API 客户端

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Create: `packages/web/src/api/cards.ts`
- Test: `packages/web/src/api/cards.test.ts`
- Modify: `packages/web/src/utils/card.ts`（`CARD_COVER_PLACEHOLDER`）

**Interfaces:**
- Produces:
  - `CardFeatured` 文件分支改为 `url`
  - `DiscoverCardsResult`、`PublicCardSummary`、`PublicProjectCard`
  - `getMyCard`、`updateMyCard`、`addCardProject`、`removeCardProject`、`setCardFeaturedProject`、`uploadCardFeatured`、`clearCardFeatured`、`publishMyCard`、`getDiscoverCards`、`getPublicCard`、`getPublicCardProject`
  - `CARD_COVER_PLACEHOLDER`

- [ ] **Step 1: 写失败测试**

Create `packages/web/src/api/cards.test.ts`：

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import {
  addCardProject,
  clearCardFeatured,
  getDiscoverCards,
  getMyCard,
  getPublicCard,
  getPublicCardProject,
  publishMyCard,
  removeCardProject,
  setCardFeaturedProject,
  updateMyCard,
  uploadCardFeatured,
} from './cards'

vi.mock('./client', () => ({ cpHttp: {}, request: vi.fn() }))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({ id: 'card-1' })
})

describe('card api', () => {
  it('loads and updates my card', async () => {
    await getMyCard()
    expect(requestMock).toHaveBeenNthCalledWith(1, cpHttp, { method: 'get', url: '/me/card' })
    await updateMyCard({ headline: 'hi', tags: ['Vue'] })
    expect(requestMock).toHaveBeenNthCalledWith(2, cpHttp, {
      method: 'put',
      url: '/me/card',
      data: { headline: 'hi', tags: ['Vue'] },
    })
  })

  it('adds and removes projects', async () => {
    await addCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/items',
      data: { projectId: 'p1' },
    })
    await removeCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/me/card/items/p1',
    })
  })

  it('sets and clears featured project', async () => {
    await setCardFeaturedProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'put',
      url: '/me/card/featured',
      data: { kind: 'project', projectId: 'p1' },
    })
    await clearCardFeatured()
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/me/card/featured',
    })
  })

  it('uploads featured file with content type and encoded name', async () => {
    const file = new File(['%PDF-1.4'], '简历.pdf', { type: 'application/pdf' })
    await uploadCardFeatured(file)
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/featured/upload',
      data: file,
      headers: {
        'Content-Type': 'application/pdf',
        'X-File-Name': encodeURIComponent('简历.pdf'),
      },
      timeout: 30_000,
    })
  })

  it('publishes and loads discovery/public data', async () => {
    await publishMyCard()
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/publish',
    })
    await getDiscoverCards({ page: 2, pageSize: 12 })
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/cards/discover',
      params: { page: 2, pageSize: 12 },
    })
    await getPublicCard('card-1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/public/cards/card-1',
    })
    await getPublicCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/public/card-projects/p1',
    })
  })
})
```

Run:

```bash
cd packages/web && npm test -- --run src/api/cards.test.ts
```

Expected: FAIL — 模块不存在。

- [ ] **Step 2: 改类型**

`packages/web/src/types/index.ts`：

```ts
export type CardFeatured =
  | { kind: 'project'; projectId: string }
  | {
      kind: 'file'
      fileType: 'pdf' | 'image'
      name: string
      size: number
      url: string
    }
  | null

export interface CardProjectItem {
  projectId: string
  name: string
  description: string
  coverUrl: string | null
  type: CardContentType
  shareUrl: string
  previewUrl?: string
  addedAt?: string
}

export interface DiscoverCardsResult {
  cards: BusinessCard[]
  total: number
  page: number
  pageSize: number
}

export interface PublicCardSummary {
  id: string
  ownerName: string
  avatarUrl: string
}

export interface PublicProjectCard {
  card: PublicCardSummary
  item: CardProjectItem
}
```

（`BusinessCard` 保持不变。）

- [ ] **Step 3: 加占位图常量**

`packages/web/src/utils/card.ts` 追加：

```ts
export const CARD_COVER_PLACEHOLDER =
  'data:image/svg+xml;charset=utf-8,' +
  encodeURIComponent(
    '<svg xmlns="http://www.w3.org/2000/svg" width="640" height="360"><rect width="640" height="360" fill="#f3f4f6"/><text x="320" y="190" text-anchor="middle" fill="#6b7280" font-size="28" font-family="sans-serif">Vibebiz</text></svg>',
  )
```

- [ ] **Step 4: 写 API 客户端**

Create `packages/web/src/api/cards.ts`：

```ts
import { cpHttp, request } from './client'
import type {
  BusinessCard,
  DiscoverCardsResult,
  PublicProjectCard,
} from '@/types'

export function getMyCard(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'get', url: '/me/card' })
}

export function updateMyCard(input: { headline: string; tags: string[] }): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'put', url: '/me/card', data: input })
}

export function addCardProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'post',
    url: '/me/card/items',
    data: { projectId },
  })
}

export function removeCardProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'delete',
    url: `/me/card/items/${encodeURIComponent(projectId)}`,
  })
}

export function setCardFeaturedProject(projectId: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'put',
    url: '/me/card/featured',
    data: { kind: 'project', projectId },
  })
}

export function uploadCardFeatured(file: File): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'post',
    url: '/me/card/featured/upload',
    data: file,
    headers: {
      'Content-Type': file.type,
      'X-File-Name': encodeURIComponent(file.name),
    },
    timeout: 30_000,
  })
}

export function clearCardFeatured(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'delete', url: '/me/card/featured' })
}

export function publishMyCard(): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, { method: 'post', url: '/me/card/publish' })
}

export function getDiscoverCards(
  params: { page?: number; pageSize?: number } = {},
): Promise<DiscoverCardsResult> {
  return request<DiscoverCardsResult>(cpHttp, {
    method: 'get',
    url: '/cards/discover',
    params,
  })
}

export function getPublicCard(id: string): Promise<BusinessCard> {
  return request<BusinessCard>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(id)}`,
  })
}

export function getPublicCardProject(projectId: string): Promise<PublicProjectCard> {
  return request<PublicProjectCard>(cpHttp, {
    method: 'get',
    url: `/public/card-projects/${encodeURIComponent(projectId)}`,
  })
}
```

- [ ] **Step 5: 跑测试**

```bash
cd packages/web && npm test -- --run src/api/cards.test.ts
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/types/index.ts packages/web/src/api/cards.ts packages/web/src/api/cards.test.ts packages/web/src/utils/card.ts
git commit -m "feat(web): add business card API client and types"
```

---

### Task 8: Web card store 改为 API 驱动

**Files:**
- Modify: `packages/web/src/stores/card.ts`
- Modify: `packages/web/src/stores/card.test.ts`

**Interfaces:**
- Consumes: Task 7 API 客户端、`addCardTag`、`validateCardUpload`。
- Produces: `useCardStore()`：

```ts
mine: Ref<BusinessCard | null>
loading/saving/uploadingFeatured: Ref<boolean>
error: Ref<string>
discoverCards: Ref<BusinessCard[]>
discoverTotal/discoverPage: Ref<number>
discoverLoading: Ref<boolean>
items: ComputedRef<CardProjectItem[]>
hasShareContent: ComputedRef<boolean>
loadMine(): Promise<void>
flushProfile(): Promise<void>
setHeadline(value: string): void
addTag(raw: string): string
removeTag(tag: string): void
addProject(projectId: string): Promise<void>
removeProject(projectId: string): Promise<void>  // 兼容别名 removeItem
setFeatured(featured: CardFeatured): Promise<void>
uploadFeatured(file: File): Promise<void>
clearFeatured(): Promise<void>
publish(): Promise<string>  // 返回 /share/card/<id>
loadDiscover(page?: number, append?: boolean): Promise<void>
fetchPublicCard(id: string): Promise<BusinessCard>
fetchPublicProject(projectId: string): Promise<PublicProjectCard>
```

- [ ] **Step 1: 写失败测试**

Replace `packages/web/src/stores/card.test.ts` with tests that mock `@/api/cards`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '@/api/cards'
import { useCardStore } from './card'

vi.mock('@/api/cards', () => ({
  getMyCard: vi.fn(),
  updateMyCard: vi.fn(),
  addCardProject: vi.fn(),
  removeCardProject: vi.fn(),
  setCardFeaturedProject: vi.fn(),
  uploadCardFeatured: vi.fn(),
  clearCardFeatured: vi.fn(),
  publishMyCard: vi.fn(),
  getDiscoverCards: vi.fn(),
  getPublicCard: vi.fn(),
  getPublicCardProject: vi.fn(),
}))

const card = (overrides = {}) => ({
  id: 'card-1',
  ownerUserId: 7,
  ownerName: 'alice',
  avatarUrl: '',
  headline: '',
  tags: [],
  items: [],
  featured: null,
  published: false,
  commentCount: 0,
  createdAt: '2026-11-09T00:00:00.000Z',
  updatedAt: '2026-11-09T00:00:00.000Z',
  ...overrides,
})

beforeEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
  setActivePinia(createPinia())
})

describe('card store', () => {
  it('loads my card', async () => {
    vi.mocked(api.getMyCard).mockResolvedValue(card({ headline: 'hi' }))
    const store = useCardStore()
    await store.loadMine()
    expect(store.mine?.headline).toBe('hi')
    expect(store.loading).toBe(false)
  })

  it('optimistically updates headline and flushes to API', async () => {
    vi.useFakeTimers()
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    vi.mocked(api.updateMyCard).mockResolvedValue(card({ headline: 'new' }))
    const store = useCardStore()
    await store.loadMine()
    store.setHeadline('new')
    expect(store.mine?.headline).toBe('new')
    expect(store.saving).toBe(true)
    await vi.advanceTimersByTimeAsync(400)
    expect(api.updateMyCard).toHaveBeenCalledWith({ headline: 'new', tags: [] })
    expect(store.saving).toBe(false)
  })

  it('adds and removes projects through the API', async () => {
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    vi.mocked(api.addCardProject).mockResolvedValue(card({ items: [{ projectId: 'p1' }] }))
    vi.mocked(api.removeCardProject).mockResolvedValue(card())
    const store = useCardStore()
    await store.loadMine()
    await store.addProject('p1')
    expect(store.items).toHaveLength(1)
    await store.removeProject('p1')
    expect(store.items).toHaveLength(0)
  })

  it('publishes and returns a share link', async () => {
    vi.mocked(api.publishMyCard).mockResolvedValue(card({ published: true }))
    const store = useCardStore()
    await expect(store.publish()).resolves.toBe('/share/card/card-1')
  })

  it('appends discovery pages', async () => {
    vi.mocked(api.getDiscoverCards)
      .mockResolvedValueOnce({ cards: [card()], total: 2, page: 1, pageSize: 12 })
      .mockResolvedValueOnce({ cards: [card({ id: 'card-2' })], total: 2, page: 2, pageSize: 12 })
    const store = useCardStore()
    await store.loadDiscover(1, false)
    await store.loadDiscover(2, true)
    expect(store.discoverCards.map((item) => item.id)).toEqual(['card-1', 'card-2'])
    expect(store.discoverTotal).toBe(2)
  })
})
```

Run:

```bash
cd packages/web && npm test -- --run src/stores/card.test.ts
```

Expected: FAIL — store 仍用 localStorage/mock。

- [ ] **Step 2: 重写 store**

Replace `packages/web/src/stores/card.ts` with API-driven implementation. Use the exact API method names from Task 7. Key points:

- `mine` starts `null`; `loadMine` sets it.
- `setHeadline`/`addTag`/`removeTag` mutate `mine` optimistically and call `scheduleProfile`.
- `scheduleProfile` saves `{headline, tags}`, sets `saving=true`, debounces 400ms; `flushProfile` clears timer and calls `updateMyCard`.
- `addProject`/`removeProject` call API and replace `mine` with the returned card; use `try/catch` set `error` and rethrow.
- `setFeatured(featured)`: if `featured?.kind === 'project'` call `setCardFeaturedProject`; else throw `new Error('unsupported featured')`.
- `uploadFeatured(file)`: client-side `validateCardUpload(file)`, then `uploadCardFeatured(file)`, set `uploadingFeatured`.
- `clearFeatured`: `clearCardFeatured`.
- `publish`: `publishMyCard`, set mine, return `/share/card/${card.id}`.
- `loadDiscover(page=1, append=false)`: `getDiscoverCards({page,pageSize:12})`; append or replace; set total/page.
- `fetchPublicCard`/`fetchPublicProject` delegate to API.
- Return `removeItem: removeProject` alias, `setFeaturedProject: (id) => setFeatured({kind:'project', projectId:id})` alias for older components.
- Remove all localStorage, mock imports, `decodeCard`, `pageCards`, `sortDiscoverCards`, `getCardById`, `findProject`, `listDiscover`, `discoverMock`.

- [ ] **Step 3: 跑测试**

```bash
cd packages/web && npm test -- --run src/stores/card.test.ts
```

Expected: PASS。

- [ ] **Step 4: Commit**

```bash
git add packages/web/src/stores/card.ts packages/web/src/stores/card.test.ts
git commit -m "feat(web): drive card store from backend API"
```

---

### Task 9: 我的名片页面接真实 store

**Files:**
- Modify: `packages/web/src/components/card/CardPreview.vue`
- Modify: `packages/web/src/components/card/CardPreview.test.ts`
- Modify: `packages/web/src/components/card/CardProfilePanel.vue`
- Modify: `packages/web/src/components/card/CardProfilePanel.test.ts`
- Modify: `packages/web/src/components/card/MyCardPanel.vue`
- Modify: `packages/web/src/components/card/MyCardPanel.test.ts`

**Interfaces:**
- Consumes: Task 8 store、`CARD_COVER_PLACEHOLDER`、`validateCardUpload`。
- Produces: `CardProfilePanel` 新 emit `upload-featured(file: File)`；`CardPreview` 使用 `featured.url`；`MyCardPanel` 异步加载真实名片。

- [ ] **Step 1: 更新 CardPreview**

- 把 `featured.dataUrl` 改为 `featured.url`。
- 删除 `DEMO_IMAGE` import，改用 `CARD_COVER_PLACEHOLDER`（`@/utils/card`）。
- PDF 继续使用 `useBlobUrl`（`https:` 原样透传，`data:` 转 blob）和无 sandbox iframe。

`CardPreview.test.ts` 中所有 `dataUrl:` 改成 `url:`（值保留 data URL 字符串，useBlobUrl 仍可处理）。

- [ ] **Step 2: 更新 CardProfilePanel**

- 移除 `readFileAsDataUrl` import 和调用。
- Props：`{ card: BusinessCard; uploadingFeatured?: boolean }`。
- Emits：新增 `'upload-featured': [file: File]`；移除 `set-featured` 的 file 分支（项目选择仍用 `set-featured`）。
- `onFileChange`：捕获 file，立即清空 input，调用 `validateCardUpload(file)`；不合法设置 `uploadError`；合法则 `emit('upload-featured', file)`。
- 上传按钮 `:disabled="uploadingFeatured"`，文案 `uploadingFeatured ? '上传中…' : '上传 PDF / 图片'`。
- 主预览仍用 `CardPreview`，`featured` 文件分支显示 `url`。

`CardProfilePanel.test.ts`：把 PDF featured 的 `dataUrl` 改成 `url`；保留“resume.pdf”断言。

- [ ] **Step 3: 更新 MyCardPanel**

- `onMounted`：

```ts
onMounted(async () => {
  await auth.ensure().catch(() => undefined)
  await store.loadMine()
})
onBeforeUnmount(() => void store.flushProfile())
```

- `store.mine` 可能为 `null`：模板外层加 `v-if="store.loading"` 加载态、`v-else-if="store.mine"` 内容、`v-else` 错误/空态。为避免模板类型错误，增加 `const card = computed(() => store.mine ?? emptyCard())`，`CardProfilePanel` 用 `:card="card"`，内容分支仍用 `v-if="store.mine"`。
- `share()` 改为 async：

```ts
async function share() {
  actionError.value = ''
  try {
    const link = await store.publish()
    shareUrl.value = `${window.location.origin}${link}`
    copied.value = false
    shareOpen.value = true
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '发布失败'
  }
}
```

`shareUrl` 改为 `ref('')`（不再 computed `/me`）。
- `@remove="store.removeItem"` 保留（store 提供别名）。
- `@set-featured="store.setFeatured"`。
- `@upload-featured="store.uploadFeatured"`，并传 `:uploading-featured="store.uploadingFeatured"`。
- `:card="store.mine"`（内容分支内）。
- 分享链接文案改为 “名片分享页已生成”，显示 `shareUrl`。

- [ ] **Step 4: 更新 MyCardPanel 测试**

Mock `@/api/cards` 的 `getMyCard` 返回一个空名片；mock `@/stores/auth`。测试：

```ts
vi.mock('@/api/cards', () => ({
  getMyCard: vi.fn().mockResolvedValue({
    id: 'card-1', ownerUserId: 7, ownerName: 'alice', avatarUrl: '',
    headline: '', tags: [], items: [], featured: null, published: false,
    commentCount: 0, createdAt: '2026-11-09T00:00:00.000Z', updatedAt: '2026-11-09T00:00:00.000Z',
  }),
  updateMyCard: vi.fn(),
  addCardProject: vi.fn(),
  removeCardProject: vi.fn(),
  setCardFeaturedProject: vi.fn(),
  uploadCardFeatured: vi.fn(),
  clearCardFeatured: vi.fn(),
  publishMyCard: vi.fn(),
  getDiscoverCards: vi.fn(),
  getPublicCard: vi.fn(),
  getPublicCardProject: vi.fn(),
}))
```

断言：加载后出现“我的项目”“个人信息”；分享按钮在无项目时 disabled；点击“添加项目”出现弹窗。

- [ ] **Step 5: 跑测试**

```bash
cd packages/web && npm test -- --run src/components/card/CardPreview.test.ts src/components/card/CardProfilePanel.test.ts src/components/card/MyCardPanel.test.ts
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/components/card/CardPreview.vue packages/web/src/components/card/CardPreview.test.ts packages/web/src/components/card/CardProfilePanel.vue packages/web/src/components/card/CardProfilePanel.test.ts packages/web/src/components/card/MyCardPanel.vue packages/web/src/components/card/MyCardPanel.test.ts
git commit -m "feat(web): connect my card page to backend"
```

---

### Task 10: 发现页接真实分页

**Files:**
- Modify: `packages/web/src/components/card/DiscoverPanel.vue`
- Modify: `packages/web/src/components/card/DiscoverPanel.test.ts`
- Modify: `packages/web/src/components/card/CardTile.vue`
- Modify: `packages/web/src/components/card/CardTile.test.ts`

**Interfaces:**
- Consumes: Task 8 store `discoverCards`、`discoverTotal`、`discoverLoading`、`discoverPage`、`loadDiscover`。
- Produces: 发现页使用真实 API；卡片点击规则不变。

- [ ] **Step 1: 更新 DiscoverPanel**

- 删除本地 `cards`/`page`/`loading`/`finished`/`error` 状态与 300ms 假延迟；直接使用 store：

```ts
const store = useCardStore()
const finished = computed(() => store.discoverCards.length >= store.discoverTotal)
const error = computed(() => store.error)

async function loadMore() {
  if (store.discoverLoading || finished.value) return
  const next = store.discoverCards.length === 0 ? 1 : store.discoverPage + 1
  await store.loadDiscover(next, next > 1)
}

onMounted(() => {
  store.loadDiscover(1, false)
  // IntersectionObserver 仍可保留；观察 sentinel
})
```

- 删除 `dataUrlToBlob` import；`openFile` 直接 `window.open(featured.url, '_blank', 'noopener,noreferrer')`（后端返回 https；若保留 data URL 兼容，可保留 blob 分支）。
- `open(id)` 用 `router.push({ name: 'card-share', params: { id } })`。
- 模板 `cards` 改为 `store.discoverCards`，`loading` 改为 `store.discoverLoading`，空态条件改为 `!store.discoverLoading && store.discoverCards.length === 0`。

- [ ] **Step 2: 更新 DiscoverPanel 测试**

Mock `@/api/cards`：

```ts
import * as api from '@/api/cards'
vi.mock('@/api/cards', () => ({
  getMyCard: vi.fn(),
  updateMyCard: vi.fn(),
  addCardProject: vi.fn(),
  removeCardProject: vi.fn(),
  setCardFeaturedProject: vi.fn(),
  uploadCardFeatured: vi.fn(),
  clearCardFeatured: vi.fn(),
  publishMyCard: vi.fn(),
  getDiscoverCards: vi.fn(),
  getPublicCard: vi.fn(),
  getPublicCardProject: vi.fn(),
}))
```

`getDiscoverCards` 第一次返回 6 条 + total 12，第二次返回 6 条 + page 2。mount 后 flush，断言 6 个 `.card-tile`；点击 `.discover-more` 后再 flush，断言 12 个和“已到底”。

卡片数据至少包含 `id/ownerName/headline/tags/items/featured/commentCount`。

- [ ] **Step 3: 更新 CardTile / 测试**

- `CardTile` 无需大改；`previewFeatured`/`isFileFeatured` 仍适用。
- `CardTile.test.ts` 的 item 可保留 `addedAt`（类型已改成可选）。
- 删除 `CardTile.test.ts` 的 `mockCards` import 与 “ships distinct preview covers” 测试（mock 文件已删除）。
- 新增/保留“预览打开项目、个人信息打开名片”的事件测试。

- [ ] **Step 4: 跑测试**

```bash
cd packages/web && npm test -- --run src/components/card/DiscoverPanel.test.ts src/components/card/CardTile.test.ts
```

Expected: PASS。

- [ ] **Step 5: Commit**

```bash
git add packages/web/src/components/card/DiscoverPanel.vue packages/web/src/components/card/DiscoverPanel.test.ts packages/web/src/components/card/CardTile.vue packages/web/src/components/card/CardTile.test.ts
git commit -m "feat(web): load discovery cards from backend"
```

---

### Task 11: 分享页接公开 API 并删除 mock

**Files:**
- Modify: `packages/web/src/views/CardShareView.vue`
- Modify: `packages/web/src/views/CardShareView.test.ts`
- Modify: `packages/web/src/views/ProjectShareView.vue`
- Modify: `packages/web/src/views/ProjectShareView.test.ts`
- Delete: `packages/web/src/mocks/cards.ts`

**Interfaces:**
- Consumes: Task 7 `getPublicCard`、`getPublicCardProject`；Task 9 `CARD_COVER_PLACEHOLDER`。
- Produces: 两个公开分享页从 CP 读取数据；不再引用任何 mock/DEMO 资源。

- [ ] **Step 1: 重写 CardShareView**

- 删除 `useCardStore`、`store.load()`、`store.getCardById`。
- 新增：

```ts
const card = ref<BusinessCard | null>(null)
const loading = ref(true)
const loadError = ref('')
onMounted(async () => {
  try {
    card.value = await getPublicCard(String(route.params.id ?? ''))
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '名片加载失败'
  } finally {
    loading.value = false
  }
  await auth.ensure().catch(() => undefined)
})
const isOwnCard = computed(() => Boolean(card.value && auth.user?.id === card.value.ownerUserId))
```

- `openFeatured`：

```ts
function openFeatured() {
  const current = card.value
  if (!current) return
  const featured = current.featured ?? (current.items[0] ? { kind: 'project' as const, projectId: current.items[0].projectId } : null)
  if (!featured) return
  if (featured.kind === 'project') {
    void router.push({ name: 'project-share', params: { projectId: featured.projectId } })
    return
  }
  window.open(featured.url, '_blank', 'noopener,noreferrer')
}
```

- 模板：`v-if="loading"` 加载中；`v-else-if="loadError || !card"` 空态；内容分支使用 `card`（ref 自动解包）。
- 保留 `.share-featured` 点击 + file overlay。
- `CardProjectItem` 仍 `:removable="false"`。
- 删除 `dataUrlToBlob` import。
- “编辑名片”按钮仍 `v-if="isOwnCard"`。

- [ ] **Step 2: 重写 ProjectShareView**

- 删除 `useCardStore`、`DEMO_*`、`findProject`。
- 新增：

```ts
const project = ref<PublicProjectCard | null>(null)
const loading = ref(true)
const loadError = ref('')
onMounted(async () => {
  try {
    project.value = await getPublicCardProject(String(route.params.projectId ?? ''))
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '项目加载失败'
  } finally {
    loading.value = false
  }
})
const item = computed(() => project.value?.item ?? null)
const card = computed(() => project.value?.card ?? null)
const type = computed<CardContentType>(() => item.value?.type ?? 'html')
const htmlSource = computed(() => {
  const url = item.value?.shareUrl ?? ''
  return url.startsWith('http') || url.startsWith('/p/') ? url : ''
})
const mediaSource = computed(() => item.value?.previewUrl ?? '')
const pdfSource = computed(() => (type.value === 'pdf' ? mediaSource.value : ''))
const pdfUrl = useBlobUrl(pdfSource)
const documentHTML = computed(() => (item.value?.previewUrl ? renderMarkdown(item.value.previewUrl) : ''))
```

- 模板渲染：
  - `type==='html' && htmlSource` → iframe（`sandbox="allow-scripts allow-forms"`）。
  - `type==='image' && mediaSource` → img。
  - `type==='pdf' && mediaSource` → iframe（`useBlobUrl`、无 sandbox）。
  - `type==='video' && mediaSource` → video。
  - `type==='document' && mediaSource` → article。
  - 否则显示 `.share-empty` “暂无可预览内容”。
- “打开原项目”只在 `htmlSource` 可用的真实 URL 时显示；“查看名片”只在 `card?.id` 非空时显示（项目不在任何名片中时后端会返回空 id）。
- 删除 demo 资源 import。

- [ ] **Step 3: 删除 mock 文件**

```bash
rm packages/web/src/mocks/cards.ts
```

检查并删除所有 `@/mocks/cards` import：

```bash
rg "@/mocks/cards" packages/web/src
```

应为空。

- [ ] **Step 4: 更新两个测试**

`CardShareView.test.ts`：

```ts
import * as api from '@/api/cards'
vi.mock('@/api/cards', () => ({ getPublicCard: vi.fn() }))
```

`getPublicCard` 返回一个 card；mock `@/stores/auth` 返回 `{ user: null, ensure: vi.fn() }`；`vue-router` mock 保留 `useRoute`/`useRouter`。断言渲染与评论/私信禁用；点击项目预览调用 `push({name:'project-share',...})`。

`ProjectShareView.test.ts`：

```ts
vi.mock('@/api/cards', () => ({ getPublicCardProject: vi.fn() }))
```

mock 返回 `{card:{id:'demo-1',ownerName:'林一',avatarUrl:''}, item:{...}}`；断言 html/image/pdf/video/document 渲染逻辑；PDF 仍断言 iframe 无 sandbox。

- [ ] **Step 5: 跑测试**

```bash
cd packages/web && npm test -- --run src/views/CardShareView.test.ts src/views/ProjectShareView.test.ts
```

Expected: PASS。

- [ ] **Step 6: Commit**

```bash
git add packages/web/src/views/CardShareView.vue packages/web/src/views/CardShareView.test.ts packages/web/src/views/ProjectShareView.vue packages/web/src/views/ProjectShareView.test.ts
git add -u packages/web/src/mocks/cards.ts
git commit -m "feat(web): load public card and project shares from backend"
```

---

### Task 12: ops-web 名片管理接真实 admin API

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/cards.ts`
- Modify: `packages/ops-web/src/api/cards.test.ts`
- Modify: `packages/ops-web/src/views/CardsView.vue`
- Modify: `packages/ops-web/src/views/CardsView.test.ts`
- Delete: `packages/ops-web/src/mocks/cards.ts`

**Interfaces:**
- Consumes: CP `GET /admin/cards`、`PUT /admin/cards/:id/hidden`、`DELETE /admin/cards/:id`。
- Produces: `AdminCardSummary` 真实字段；`CardsView` 真实状态与操作。

- [ ] **Step 1: 更新类型**

`packages/ops-web/src/types/index.ts`：

```ts
export interface AdminCardSummary {
  id: string
  ownerUserId: number
  ownerName: string
  headline: string
  tags: string[]
  projectCount: number
  featuredType: string
  published: boolean
  hidden: boolean
  status: string
  updatedAt: string
}
```

- [ ] **Step 2: 更新 API**

Replace `packages/ops-web/src/api/cards.ts`：

```ts
import { request } from './client'
import type { AdminCardSummary, PageQuery, PageResult } from '@/types'

export async function listCards(query: PageQuery = {}): Promise<PageResult<AdminCardSummary>> {
  return request<PageResult<AdminCardSummary>>({
    method: 'get',
    url: '/cards',
    params: { page: query.page, pageSize: query.pageSize, keyword: query.keyword },
  })
}

export async function updateCardHidden(id: string, hidden: boolean): Promise<void> {
  await request<unknown>({
    method: 'put',
    url: `/cards/${encodeURIComponent(id)}/hidden`,
    data: { hidden },
  })
}

export async function deleteCard(id: string): Promise<void> {
  await request<unknown>({
    method: 'delete',
    url: `/cards/${encodeURIComponent(id)}`,
  })
}
```

删除 `packages/ops-web/src/mocks/cards.ts`。

- [ ] **Step 3: 更新 API 测试**

```ts
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { request } from './client'
import { deleteCard, listCards, updateCardHidden } from './cards'

vi.mock('./client', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

beforeEach(() => requestMock.mockReset())

describe('cards api', () => {
  it('lists cards with query params', async () => {
    requestMock.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 10 })
    await listCards({ page: 2, pageSize: 10, keyword: 'alice' })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'get',
      url: '/cards',
      params: { page: 2, pageSize: 10, keyword: 'alice' },
    })
  })

  it('hides a card', async () => {
    requestMock.mockResolvedValue({})
    await updateCardHidden('c1', true)
    expect(requestMock).toHaveBeenCalledWith({
      method: 'put',
      url: '/cards/c1/hidden',
      data: { hidden: true },
    })
  })

  it('deletes a card', async () => {
    requestMock.mockResolvedValue({})
    await deleteCard('c1')
    expect(requestMock).toHaveBeenCalledWith({ method: 'delete', url: '/cards/c1' })
  })
})
```

- [ ] **Step 4: 更新 CardsView**

- 列：

```ts
const columns: TableColumn[] = [
  { key: 'ownerName', label: '名片', width: '140px' },
  { key: 'headline', label: '简介' },
  { key: 'tags', label: '标签', format: (value) => (Array.isArray(value) ? value.join(' / ') : '—') },
  { key: 'projectCount', label: '项目数', width: '90px' },
  { key: 'featuredType', label: '主预览', width: '100px' },
  { key: 'status', label: '状态', width: '100px' },
  { key: 'updatedAt', label: '更新时间', width: '180px' },
  { key: 'actions', label: '操作', width: '180px' },
]
```

- 删除本地 `mock` 相关 import。
- 隐藏/恢复按钮：`row.hidden ? '恢复' : '隐藏'`，调用 `updateCardHidden(row.id, !row.hidden)`。
- 删除：确认后 `deleteCard(row.id)`；删除最后一行时若 `page > 1 && rows.length === 1` 调 `setPage(page-1)`，否则 `load()`。
- 状态列显示 `值`（`正常/未发布/已隐藏`）。
- 搜索走接口。

- [ ] **Step 5: 更新 CardsView 测试**

Mock `@/api/cards`，行数据带 `status/headline/published`。断言：渲染 ownerName/status；隐藏调用 `updateCardHidden('c1', true)`；删除确认调用 `deleteCard('c1')`；分页总数。

- [ ] **Step 6: 跑测试**

```bash
cd packages/ops-web && npm test -- --run src/api/cards.test.ts src/views/CardsView.test.ts
cd packages/ops-web && npx vue-tsc --noEmit
```

Expected: PASS。

- [ ] **Step 7: Commit**

```bash
git add packages/ops-web/src/types/index.ts packages/ops-web/src/api/cards.ts packages/ops-web/src/api/cards.test.ts packages/ops-web/src/views/CardsView.vue packages/ops-web/src/views/CardsView.test.ts
git add -u packages/ops-web/src/mocks/cards.ts
git commit -m "feat(ops-web): load card management from admin API"
```

---

### Task 13: 全量验证与收尾

**Files:**
- Modify: `.ai/CURRENT_FOCUS`（`make checkpoint`）

**Interfaces:**
- Consumes: Task 1-12 全部产出。
- Produces: 全量测试、构建、lint、质量门禁通过。

- [ ] **Step 1: Backend**

```bash
cd packages/backend
go test ./...
go vet ./...
```

Expected: PASS。

- [ ] **Step 2: Web**

```bash
cd packages/web
npm test -- --run
npm run build
npm run lint
```

Expected: PASS。

- [ ] **Step 3: ops-web**

```bash
cd packages/ops-web
npm test -- --run
npm run build
npm run lint
```

Expected: PASS。

- [ ] **Step 4: Root**

```bash
make validate
```

Expected: errors=0 warnings=0。

- [ ] **Step 5: 手工冒烟**

1. `make dev`。
2. 数据库已迁移到 v33；登录 web。
3. `/my-card`：
   - 添加一个已发布项目；设置项目主预览。
   - 上传一个 ≤2MB PDF / 图片；确认 S3 存储与预览正常。
   - 添加 5 个标签，刷新后仍在。
   - 点击分享，打开 `/share/card/<uuid>`，匿名窗口可访问。
4. `/discover`：只看到真实已发布名片；滚动加载下一页；不再出现 mock 名片。
5. 点击名片预览：
   - 项目 → 项目分享页。
   - 图片/PDF → 新标签打开原文件。
6. 点击个人信息 → 名片分享页。
7. ops-web `/cards`：搜索、隐藏、恢复、删除；隐藏后公开链接 404；删除后用户重新进入 `/my-card` 得到新空名片。
8. 取消发布一个项目后，名片中该条目消失；若无名片项目，发现页不展示该名片。

- [ ] **Step 6: Harness checkpoint**

```bash
make checkpoint
```

- [ ] **Step 7: Commit**

```bash
git add .ai/CURRENT_FOCUS .ai/checkpoints
git commit -m "chore: checkpoint business card backend"
```

---

## Self-Review

**Spec coverage**

| Spec 要求 | 对应任务 |
|---|---|
| `business_card` / `business_card_item` 迁移 | Task 1 |
| 一个用户一张名片、UUID 分享 ID | Task 1 + 2 |
| 项目条目只存 ID，实时解析 | Task 2 + 3 |
| 只能添加自己已发布且可见项目 | Task 3 |
| 最多 5 项目 / 5 标签、12 字 | Task 3 |
| 主预览项目必须来自条目 | Task 3 |
| 发布要求至少一个可见项目 | Task 3 |
| 图片/PDF 上传 S3、2MB、magic bytes | Task 4 |
| 发现页真实分页、过滤隐藏/未发布/零项目 | Task 5 |
| 公开名片接口（未发布可直达、隐藏 404） | Task 5 + 6 |
| 公开项目接口 + 名片主人回退 | Task 5 + 6 |
| admin 列表、隐藏、删除、删除 S3 | Task 5 + 6 |
| CP 路由与装配 | Task 6 |
| web API 客户端与类型 | Task 7 |
| web store 去 localStorage/mock | Task 8 |
| 我的名片页真实加载/保存/上传/分享 | Task 9 |
| 发现页真实分页 | Task 10 |
| 分享页真实 API + 删除 mock | Task 11 |
| ops-web 真实 admin API | Task 12 |
| 全量验证 | Task 13 |

**Placeholder scan:** 无 TBD / TODO；每个代码步骤给出实际代码或明确的文件级改动。

**Type consistency:**
- `businesscard.Card` / `CardItem` / `Featured` 与 `packages/web/src/types/index.ts` 字段一致。
- `CardFeatured` 文件分支在 Task 7 统一改为 `url`，Task 9/11 全部使用 `url`。
- `Repository` 方法名在 Task 2 定义，Task 3/5 只调用这些名称。
- API 路径在 Task 6 注册、Task 7/12 调用，保持 `/me/card`、`/cards/discover`、`/public/*`、`/admin/cards*` 一致。

**已知风险:**
- 发现页 `total` 是数据库候选总数，过滤零可见项目后可能出现页内数量少于 `pageSize`；已写入 spec。
- 项目/头像解析按卡片逐个调用 `project.Get` / `OwnerProfile`，发现页最多 12×5 次查询；本期接受，后续可加批量接口。
- S3 未配置时上传返回 503；文本名片仍可用。

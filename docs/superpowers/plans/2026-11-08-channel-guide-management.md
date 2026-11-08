# 发布渠道与指引管理 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 ops-web 管理发布渠道与 markdown 指引，并让 CP 公开接口、web 设置页和渠道包生成统一使用 MySQL 中的最新内容。

**Architecture:** 新增 `channel_guide` 表与 `internal/channelguide` 领域包，CP 提供公开只读接口和 admin CRUD；ops-web 只调用 CP admin；`channelpkg` 通过 `GuideProvider` 注入数据库指引，适配器保留内置 markdown 兜底。

**Tech Stack:** Go 1.24 + Echo v4 + GORM + MySQL 8 + goose；Vue 3 + TypeScript + PrimeVue + vitest。

**Spec:** `docs/superpowers/specs/2026-11-08-channel-guide-management-design.md`

## Global Constraints

- 依赖方向固定：`ops-web → CP`，禁止 ops-web 访问 DP、sandbox 或 `/internal/projects`。
- 响应统一 `{ code, message, result }`，成功 `code=0`。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 不引入 markdown 渲染新依赖到 ops-web；web 继续使用现有 `renderMarkdown`。
- markdown 上限 512KB，必须合法 UTF-8，存储前去 BOM、CRLF 归一为 `\n`。
- 内置渠道 `wechat_minigame` / `douyin_minigame` / `steam_desktop` 可编辑、可停用、不可删除。
- 新增渠道仅指引，不参与打包；`packageable` 由服务端按三个适配器白名单计算。
- 停用只影响前台展示；生成接口仍可用，生成包始终取最新数据库指引，缺失时回退内置 markdown。
- 不写无意义注释；公共函数需要说明时写一行。

---

## Task 1: 数据库迁移、模型与仓储

**Files:**
- Create: `packages/backend/migrations/00030_channel_guide.sql`
- Create: `packages/backend/internal/dao/model/channel_guide.go`
- Create: `packages/backend/internal/dao/channel_guide_repository.go`
- Create: `packages/backend/internal/dao/channel_guide_repository_test.go`
- Modify: `packages/backend/migrations/migrations_test.go`
- Test: `packages/backend/migrations/migrations_test.go`、`packages/backend/internal/dao/channel_guide_repository_test.go`

**Interfaces:**
- Produces: `model.ChannelGuide`，字段 `ID int64`、`Channel string`、`Title string`、`Summary string`、`Markdown string`、`Steps model.JSON`、`Requirements model.JSON`、`Enabled bool`、`Builtin bool`、`SortOrder int`、`CreatedAt/UpdatedAt time.Time`。
- Produces: `dao.NewChannelGuideRepository(db *gorm.DB) *ChannelGuideRepository`，方法 `List(ctx, includeDisabled bool, keyword string) ([]model.ChannelGuide, error)`、`Get(ctx, channel string) (*model.ChannelGuide, error)`、`Create(ctx, *model.ChannelGuide) error`、`Update(ctx, *model.ChannelGuide) error`、`Delete(ctx, channel string) error`、`SeedDefaults(ctx, []model.ChannelGuide) error`。

- [ ] **Step 1: 写迁移与迁移测试**

创建 `00030_channel_guide.sql`：

```sql
-- channel guide content managed by ops-web

-- +goose Up

CREATE TABLE `channel_guide` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `channel` varchar(32) NOT NULL COMMENT '渠道标识',
  `title` varchar(64) NOT NULL COMMENT '渠道名称',
  `summary` varchar(255) NOT NULL DEFAULT '' COMMENT '渠道简介',
  `markdown` mediumtext NOT NULL COMMENT '指引 markdown',
  `steps` json DEFAULT NULL COMMENT '发布步骤',
  `requirements` json DEFAULT NULL COMMENT '资质要求',
  `enabled` tinyint(1) NOT NULL DEFAULT 1 COMMENT '是否在前台展示',
  `builtin` tinyint(1) NOT NULL DEFAULT 0 COMMENT '是否内置渠道',
  `sort_order` int NOT NULL DEFAULT 100 COMMENT '排序，越小越靠前',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_channel_guide_channel` (`channel`),
  KEY `idx_channel_guide_enabled_sort` (`enabled`, `sort_order`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='发布渠道指引';

-- +goose Down

DROP TABLE IF EXISTS `channel_guide`;
```

在 `migrations_test.go` 的业务表列表中加入 `"channel_guide"`，并新增：

```go
func TestChannelGuideMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00030_channel_guide.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `channel_guide`", "uk_channel_guide_channel",
		"idx_channel_guide_enabled_sort", "builtin", "sort_order",
		"DROP TABLE IF EXISTS `channel_guide`", "-- +goose Up", "-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("channel guide migration missing %q", want)
		}
	}
}
```

- [ ] **Step 2: 运行迁移测试确认失败**

Run: `cd packages/backend && go test ./migrations -run TestChannelGuideMigration -v`
Expected: FAIL，文件不存在或缺少表。

- [ ] **Step 3: 写模型与仓储**

`internal/dao/model/channel_guide.go`：

```go
package model

import "time"

type ChannelGuide struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Channel      string    `gorm:"column:channel" json:"channel"`
	Title        string    `gorm:"column:title" json:"title"`
	Summary      string    `gorm:"column:summary" json:"summary"`
	Markdown     string    `gorm:"column:markdown" json:"markdown"`
	Steps        JSON      `gorm:"column:steps;type:json" json:"steps"`
	Requirements JSON      `gorm:"column:requirements;type:json" json:"requirements"`
	Enabled      bool      `gorm:"column:enabled" json:"enabled"`
	Builtin      bool      `gorm:"column:builtin" json:"builtin"`
	SortOrder    int       `gorm:"column:sort_order" json:"sortOrder"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updatedAt"`
}

func (ChannelGuide) TableName() string { return "channel_guide" }
```

`internal/dao/channel_guide_repository.go`：

```go
package dao

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"vibebiz/internal/dao/model"
)

type ChannelGuideRepository struct {
	db *gorm.DB
}

func NewChannelGuideRepository(db *gorm.DB) *ChannelGuideRepository {
	return &ChannelGuideRepository{db: db}
}

func (r *ChannelGuideRepository) List(ctx context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error) {
	query := r.db.WithContext(ctx).Model(&model.ChannelGuide{})
	if !includeDisabled {
		query = query.Where("enabled = ?", true)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("(channel LIKE ? OR title LIKE ? OR summary LIKE ?)", like, like, like)
	}
	items := make([]model.ChannelGuide, 0)
	if err := query.Order("sort_order ASC, channel ASC").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("list channel guides: %w", err)
	}
	return items, nil
}

func (r *ChannelGuideRepository) Get(ctx context.Context, channel string) (*model.ChannelGuide, error) {
	var item model.ChannelGuide
	if err := r.db.WithContext(ctx).Where("channel = ?", channel).Take(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get channel guide: %w", err)
	}
	return &item, nil
}

func (r *ChannelGuideRepository) Create(ctx context.Context, item *model.ChannelGuide) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("create channel guide: %w", err)
	}
	return nil
}

func (r *ChannelGuideRepository) Update(ctx context.Context, item *model.ChannelGuide) error {
	if err := r.db.WithContext(ctx).Save(item).Error; err != nil {
		return fmt.Errorf("update channel guide: %w", err)
	}
	return nil
}

func (r *ChannelGuideRepository) Delete(ctx context.Context, channel string) error {
	result := r.db.WithContext(ctx).Where("channel = ?", channel).Delete(&model.ChannelGuide{})
	if result.Error != nil {
		return fmt.Errorf("delete channel guide: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *ChannelGuideRepository) SeedDefaults(ctx context.Context, items []model.ChannelGuide) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "channel"}}, DoNothing: true}).
		Create(&items).Error; err != nil {
		return fmt.Errorf("seed channel guides: %w", err)
	}
	return nil
}
```

说明：`channel_guide` 不使用 GORM 软删除，`Delete` 是硬删除，避免唯一键与软删除冲突。

- [ ] **Step 4: 写仓储测试**

`channel_guide_repository_test.go` 使用现有 `newMockDB`：

```go
package dao

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"vibebiz/internal/dao/model"
)

func TestChannelGuideRepositoryCreate(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `channel_guide`").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()
	if err := repo.Create(context.Background(), &model.ChannelGuide{Channel: "taptap_mobile", Title: "TapTap"}); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestChannelGuideRepositoryGetNotFound(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectQuery("SELECT \\* FROM `channel_guide` WHERE channel = \\?").
		WithArgs("missing", 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "channel"}))
	if _, err := repo.Get(context.Background(), "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestChannelGuideRepositoryListEnabled(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	rows := sqlmock.NewRows([]string{"id", "channel", "title", "enabled", "sort_order"}).
		AddRow(1, "wechat_minigame", "微信小游戏", 1, 10)
	mock.ExpectQuery("SELECT \\* FROM `channel_guide` WHERE enabled = \\? ORDER BY sort_order ASC, channel ASC").
		WithArgs(true).
		WillReturnRows(rows)
	items, err := repo.List(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Channel != "wechat_minigame" {
		t.Fatalf("items = %+v", items)
	}
}

func TestChannelGuideRepositorySeedDefaults(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewChannelGuideRepository(db)
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO `channel_guide`").
		WillReturnResult(sqlmock.NewResult(1, 3))
	mock.ExpectCommit()
	items := []model.ChannelGuide{{Channel: "wechat_minigame"}, {Channel: "douyin_minigame"}, {Channel: "steam_desktop"}}
	if err := repo.SeedDefaults(context.Background(), items); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/backend && go test ./migrations ./internal/dao`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add packages/backend/migrations/00030_channel_guide.sql packages/backend/migrations/migrations_test.go packages/backend/internal/dao/model/channel_guide.go packages/backend/internal/dao/channel_guide_repository.go packages/backend/internal/dao/channel_guide_repository_test.go
git commit -m "feat: add channel guide storage"
```

---

## Task 2: `channelguide` 领域服务与内存仓储

**Files:**
- Create: `packages/backend/internal/channelguide/repository.go`
- Create: `packages/backend/internal/channelguide/service.go`
- Create: `packages/backend/internal/channelguide/memory.go`
- Create: `packages/backend/internal/channelguide/service_test.go`

**Interfaces:**
- Consumes: Task 1 的 `dao/model.ChannelGuide`，以及现有 `internal/channelpkg/guides` 的内置 markdown 与元数据。
- Produces: `channelguide.Repository` 接口、`channelguide.NewService(repo Repository) *Service`、`channelguide.NewMemoryRepository() *MemoryRepository`。
- Produces: `Service.EnsureDefaults(ctx) error`、`List(ctx, includeDisabled bool, keyword string) ([]Item, error)`、`Get(ctx, channel string) (Item, error)`、`Create(ctx, Input) (Item, error)`、`Update(ctx, channel string, Input) (Item, error)`、`Delete(ctx, channel string) error`、`UploadMarkdown(ctx, channel, filename string, data []byte) (Item, error)`、`Markdown(ctx, channel string) (string, error)`。
- Produces: `Item` 字段 `Channel`、`Title`、`Summary`、`Markdown`、`Steps []string`、`Requirements []string`、`Enabled`、`Builtin`、`Packageable`、`SortOrder`、`UpdatedAt time.Time`。

- [ ] **Step 1: 写失败测试**

`service_test.go`：

```go
package channelguide

import (
	"context"
	"errors"
	"strings"
	"testing"

	"vibebiz/internal/channelpkg/core"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc := NewService(NewMemoryRepository())
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	return svc
}

func TestEnsureDefaultsSeedsBuiltinChannels(t *testing.T) {
	svc := newTestService(t)
	items, err := svc.List(context.Background(), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3", len(items))
	}
	for _, item := range items {
		if !item.Builtin || !item.Enabled || strings.TrimSpace(item.Markdown) == "" {
			t.Fatalf("bad builtin item: %+v", item)
		}
	}
}

func TestEnsureDefaultsDoesNotOverwriteEdits(t *testing.T) {
	svc := newTestService(t)
	item, err := svc.Update(context.Background(), core.ChannelWeChat, Input{
		Title: "编辑后的微信", Summary: "s", Markdown: "# edited", Enabled: false, SortOrder: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := svc.Get(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != item.Title || got.Markdown != "# edited" || got.Enabled {
		t.Fatalf("seed overwrote edits: %+v", got)
	}
}

func TestCreateCustomChannelAndPublicList(t *testing.T) {
	svc := newTestService(t)
	created, err := svc.Create(context.Background(), Input{
		Channel: "taptap_mobile", Title: "TapTap", Summary: "",
		Markdown: "# TapTap", Steps: []string{"准备资质"}, Requirements: []string{"企业主体"},
		Enabled: true, SortOrder: 40,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Packageable {
		t.Fatal("custom channel must not be packageable")
	}
	items, err := svc.List(context.Background(), false, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 4 {
		t.Fatalf("items = %d, want 4", len(items))
	}
	if _, err := svc.Create(context.Background(), Input{Channel: "taptap_mobile", Title: "dup"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func TestBuiltinCannotBeDeleted(t *testing.T) {
	svc := newTestService(t)
	if err := svc.Delete(context.Background(), core.ChannelWeChat); !errors.Is(err, ErrBuiltinDelete) {
		t.Fatalf("err = %v, want ErrBuiltinDelete", err)
	}
}

func TestDeleteCustomChannel(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.Create(context.Background(), Input{Channel: "custom_channel", Title: "自定义", Markdown: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(context.Background(), "custom_channel"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(context.Background(), "custom_channel"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUploadMarkdownNormalizesAndValidates(t *testing.T) {
	svc := newTestService(t)
	item, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.md", []byte("\xEF\xBB\xBF# 新指引\r\n第二行"))
	if err != nil {
		t.Fatal(err)
	}
	if item.Markdown != "# 新指引\n第二行" {
		t.Fatalf("markdown = %q", item.Markdown)
	}
	if _, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.txt", []byte("x")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := svc.UploadMarkdown(context.Background(), core.ChannelWeChat, "guide.md", []byte{0xff, 0xfe}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestMarkdownFallsBackToEmbedded(t *testing.T) {
	svc := NewService(NewMemoryRepository())
	got, err := svc.Markdown(context.Background(), core.ChannelWeChat)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "微信") {
		t.Fatalf("markdown = %q", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelguide -v`
Expected: FAIL，包或符号不存在。

- [ ] **Step 3: 实现 Repository 接口与错误**

`repository.go`：

```go
package channelguide

import (
	"context"
	"errors"

	"vibebiz/internal/dao/model"
)

var (
	ErrInvalid      = errors.New("invalid channel guide")
	ErrNotFound     = errors.New("channel guide not found")
	ErrConflict     = errors.New("channel guide already exists")
	ErrBuiltinDelete = errors.New("builtin channel guide cannot be deleted")
	ErrTooLarge     = errors.New("channel guide markdown is too large")
)

type Repository interface {
	List(ctx context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error)
	Get(ctx context.Context, channel string) (*model.ChannelGuide, error)
	Create(ctx context.Context, item *model.ChannelGuide) error
	Update(ctx context.Context, item *model.ChannelGuide) error
	Delete(ctx context.Context, channel string) error
	SeedDefaults(ctx context.Context, items []model.ChannelGuide) error
}
```

- [ ] **Step 4: 实现服务**

`service.go`：

```go
package channelguide

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"vibebiz/internal/channelpkg/core"
	"vibebiz/internal/channelpkg/guides"
	"vibebiz/internal/dao/model"
)

const (
	MaxMarkdownBytes = 512 << 10
	maxLines         = 20
	maxLineLength    = 120
)

var channelPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

var packageableChannels = map[string]bool{
	core.ChannelWeChat: true,
	core.ChannelDouyin: true,
	core.ChannelSteam:  true,
}

type Input struct {
	Channel      string
	Title        string
	Summary      string
	Markdown     string
	Steps        []string
	Requirements []string
	Enabled      bool
	SortOrder    int
}

type Item struct {
	Channel      string    `json:"channel"`
	Title        string    `json:"title"`
	Summary      string    `json:"summary"`
	Markdown     string    `json:"markdown"`
	Steps        []string  `json:"steps"`
	Requirements []string  `json:"requirements"`
	Enabled      bool      `json:"enabled"`
	Builtin      bool      `json:"builtin"`
	Packageable  bool      `json:"packageable"`
	SortOrder    int       `json:"sortOrder"`
	UpdatedAt    time.Time `json:"-"`
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) EnsureDefaults(ctx context.Context) error {
	items := make([]model.ChannelGuide, 0, 3)
	for _, def := range guides.List() {
		items = append(items, model.ChannelGuide{
			Channel: def.Channel, Title: def.Title, Summary: def.Summary,
			Markdown: def.Markdown, Steps: encodeLines(def.Steps), Requirements: encodeLines(def.Requirements),
			Enabled: true, Builtin: true, SortOrder: defaultSort(def.Channel),
		})
	}
	return s.repo.SeedDefaults(ctx, items)
}

func defaultSort(channel string) int {
	switch channel {
	case core.ChannelWeChat:
		return 10
	case core.ChannelDouyin:
		return 20
	case core.ChannelSteam:
		return 30
	default:
		return 100
	}
}

func (s *Service) List(ctx context.Context, includeDisabled bool, keyword string) ([]Item, error) {
	items, err := s.repo.List(ctx, includeDisabled, strings.TrimSpace(keyword))
	if err != nil {
		return nil, err
	}
	out := make([]Item, 0, len(items))
	for i := range items {
		out = append(out, toItem(&items[i]))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, channel string) (Item, error) {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Create(ctx context.Context, input Input) (Item, error) {
	item, err := s.normalizeNew(input)
	if err != nil {
		return Item{}, err
	}
	if err := s.repo.Create(ctx, item); err != nil {
		if isDuplicate(err) {
			return Item{}, ErrConflict
		}
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Update(ctx context.Context, channel string, input Input) (Item, error) {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		return Item{}, err
	}
	normalized, err := s.apply(item, input)
	if err != nil {
		return Item{}, err
	}
	if err := s.repo.Update(ctx, normalized); err != nil {
		return Item{}, err
	}
	return toItem(normalized), nil
}

func (s *Service) Delete(ctx context.Context, channel string) error {
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		return err
	}
	if item.Builtin {
		return ErrBuiltinDelete
	}
	return s.repo.Delete(ctx, channel)
}

func (s *Service) UploadMarkdown(ctx context.Context, channel, filename string, data []byte) (Item, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext != ".md" && ext != ".markdown" {
		return Item{}, ErrInvalid
	}
	text, err := normalizeMarkdown(data)
	if err != nil {
		return Item{}, err
	}
	if strings.TrimSpace(text) == "" {
		return Item{}, ErrInvalid
	}
	item, err := s.repo.Get(ctx, channel)
	if err != nil {
		return Item{}, err
	}
	item.Markdown = text
	if err := s.repo.Update(ctx, item); err != nil {
		return Item{}, err
	}
	return toItem(item), nil
}

func (s *Service) Markdown(ctx context.Context, channel string) (string, error) {
	item, err := s.repo.Get(ctx, channel)
	if errors.Is(err, ErrNotFound) {
		return guides.Markdown(channel), nil
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(item.Markdown) == "" {
		return guides.Markdown(channel), nil
	}
	return item.Markdown, nil
}

func (s *Service) normalizeNew(input Input) (*model.ChannelGuide, error) {
	channel := strings.TrimSpace(input.Channel)
	if !channelPattern.MatchString(channel) {
		return nil, ErrInvalid
	}
	item := &model.ChannelGuide{Channel: channel, Builtin: false}
	return s.apply(item, input)
}

func (s *Service) apply(item *model.ChannelGuide, input Input) (*model.ChannelGuide, error) {
	title := strings.TrimSpace(input.Title)
	summary := strings.TrimSpace(input.Summary)
	steps, err := normalizeLines(input.Steps)
	if err != nil {
		return nil, err
	}
	requirements, err := normalizeLines(input.Requirements)
	if err != nil {
		return nil, err
	}
	markdown, err := normalizeMarkdown([]byte(input.Markdown))
	if err != nil {
		return nil, err
	}
	if title == "" || len([]rune(title)) > 64 || len([]rune(summary)) > 255 {
		return nil, ErrInvalid
	}
	if input.SortOrder < 0 || input.SortOrder > 9999 {
		return nil, ErrInvalid
	}
	item.Title = title
	item.Summary = summary
	item.Markdown = markdown
	item.Steps = encodeLines(steps)
	item.Requirements = encodeLines(requirements)
	item.Enabled = input.Enabled
	item.SortOrder = input.SortOrder
	return item, nil
}

func normalizeLines(values []string) ([]string, error) {
	if len(values) > maxLines {
		return nil, ErrInvalid
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len([]rune(value)) > maxLineLength {
			return nil, ErrInvalid
		}
		out = append(out, value)
	}
	return out, nil
}

func normalizeMarkdown(data []byte) (string, error) {
	if len(data) > MaxMarkdownBytes {
		return "", ErrTooLarge
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	if !utf8.Valid(data) {
		return "", ErrInvalid
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return text, nil
}

func isDuplicate(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "duplicate") || strings.Contains(message, "1062")
}

func toItem(item *model.ChannelGuide) Item {
	return Item{
		Channel: item.Channel, Title: item.Title, Summary: item.Summary,
		Markdown: item.Markdown, Steps: decodeLines(item.Steps), Requirements: decodeLines(item.Requirements),
		Enabled: item.Enabled, Builtin: item.Builtin,
		Packageable: packageableChannels[item.Channel], SortOrder: item.SortOrder, UpdatedAt: item.UpdatedAt,
	}
}

func encodeLines(values []string) model.JSON {
	data, _ := json.Marshal(values)
	return data
}

func decodeLines(data model.JSON) []string {
	var out []string
	if len(data) > 0 {
		_ = json.Unmarshal(data, &out)
	}
	if out == nil {
		out = []string{}
	}
	return out
}
```

- [ ] **Step 5: 实现内存仓储**

`memory.go`：

```go
package channelguide

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"vibebiz/internal/dao/model"
)

type MemoryRepository struct {
	mu    sync.Mutex
	items map[string]model.ChannelGuide
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: make(map[string]model.ChannelGuide)}
}

func (r *MemoryRepository) List(_ context.Context, includeDisabled bool, keyword string) ([]model.ChannelGuide, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]model.ChannelGuide, 0, len(r.items))
	for _, item := range r.items {
		if !includeDisabled && !item.Enabled {
			continue
		}
		if keyword != "" && !strings.Contains(item.Channel+item.Title+item.Summary, keyword) {
			continue
		}
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].SortOrder != out[j].SortOrder {
			return out[i].SortOrder < out[j].SortOrder
		}
		return out[i].Channel < out[j].Channel
	})
	return out, nil
}

func (r *MemoryRepository) Get(_ context.Context, channel string) (*model.ChannelGuide, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[channel]
	if !ok {
		return nil, ErrNotFound
	}
	return &item, nil
}

func (r *MemoryRepository) Create(_ context.Context, item *model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.Channel]; ok {
		return ErrConflict
	}
	now := time.Now().UTC()
	item.CreatedAt = now
	item.UpdatedAt = now
	r.items[item.Channel] = *item
	return nil
}

func (r *MemoryRepository) Update(_ context.Context, item *model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[item.Channel]; !ok {
		return ErrNotFound
	}
	item.UpdatedAt = time.Now().UTC()
	r.items[item.Channel] = *item
	return nil
}

func (r *MemoryRepository) Delete(_ context.Context, channel string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[channel]; !ok {
		return ErrNotFound
	}
	delete(r.items, channel)
	return nil
}

func (r *MemoryRepository) SeedDefaults(_ context.Context, items []model.ChannelGuide) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	for _, item := range items {
		if _, ok := r.items[item.Channel]; ok {
			continue
		}
		item.CreatedAt = now
		item.UpdatedAt = now
		r.items[item.Channel] = item
	}
	return nil
}
```

- [ ] **Step 6: 运行测试**

Run: `cd packages/backend && go test ./internal/channelguide -v`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add packages/backend/internal/channelguide
git commit -m "feat: add channel guide service"
```

---

## Task 3: CP handler、路由与启动装配

**Files:**
- Create: `packages/backend/internal/handler/channel_guide.go`
- Create: `packages/backend/internal/handler/channel_guide_test.go`
- Modify: `packages/backend/internal/handler/channel_package.go`（删除旧 `Guides` 方法）
- Modify: `packages/backend/internal/handler/channel_package_test.go`（删除旧公开指引测试）
- Modify: `packages/backend/internal/router/control.go`
- Modify: `packages/backend/internal/router/helpers_test.go`
- Modify: `packages/backend/internal/router/channel_package_test.go`
- Create: `packages/backend/internal/router/channel_guide_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Test: 上述测试文件

**Interfaces:**
- Consumes: Task 2 的 `channelguide.Service`。
- Produces: `handler.NewChannelGuideHandler(service *channelguide.Service) *ChannelGuideHandler`，方法 `PublicList`、`List`、`Get`、`Create`、`Update`、`Delete`、`UploadMarkdown`。
- Produces: `router.RegisterControl(..., channelPackages *handler.ChannelPackageHandler, channelGuides *handler.ChannelGuideHandler, cloneHandler ...)`，新增参数位于 `channelPackages` 之后。

- [ ] **Step 1: 写 handler 测试**

`handler/channel_guide_test.go`：

```go
package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/channelguide"
)

func newChannelGuideHandler(t *testing.T) *ChannelGuideHandler {
	t.Helper()
	svc := channelguide.NewService(channelguide.NewMemoryRepository())
	if err := svc.EnsureDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	return NewChannelGuideHandler(svc)
}

func TestChannelGuideHandlerPublicList(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.GET("/guides", h.PublicList)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/guides", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "\"updatedAt\":\"0001") {
		t.Fatalf("updatedAt missing: %s", rec.Body.String())
	}
}

func TestChannelGuideHandlerCreateAndDeleteBuiltin(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides", h.Create)
	e.DELETE("/guides/:channel", h.Delete)

	body := `{"channel":"taptap_mobile","title":"TapTap","summary":"","markdown":"# TapTap","steps":["a"],"requirements":["b"],"enabled":true,"sortOrder":40}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/guides", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated || !strings.Contains(rec.Body.String(), "taptap_mobile") {
		t.Fatalf("create status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/guides/wechat_minigame", nil))
	if rec.Code != http.StatusConflict || !strings.Contains(rec.Body.String(), "40905") {
		t.Fatalf("delete builtin status = %d body = %s", rec.Code, rec.Body.String())
	}
}

func TestChannelGuideHandlerUploadMarkdown(t *testing.T) {
	e := echo.New()
	h := newChannelGuideHandler(t)
	e.POST("/guides/:channel/markdown", h.UploadMarkdown)

	body := "--boundary\r\nContent-Disposition: form-data; name=\"file\"; filename=\"guide.md\"\r\nContent-Type: text/markdown\r\n\r\n# Uploaded\r\n--boundary--\r\n"
	req := httptest.NewRequest(http.MethodPost, "/guides/wechat_minigame/markdown", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, "multipart/form-data; boundary=boundary")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "# Uploaded") {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler -run ChannelGuide -v`
Expected: FAIL，`NewChannelGuideHandler` 未定义。

- [ ] **Step 3: 实现 handler**

`handler/channel_guide.go`：

```go
package handler

import (
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"vibebiz/internal/channelguide"
	"vibebiz/internal/response"
)

type ChannelGuideHandler struct {
	Service *channelguide.Service
}

func NewChannelGuideHandler(service *channelguide.Service) *ChannelGuideHandler {
	return &ChannelGuideHandler{Service: service}
}

type channelGuideRequest struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	Enabled      *bool    `json:"enabled"`
	SortOrder    int      `json:"sortOrder"`
}

func (r channelGuideRequest) input() channelguide.Input {
	enabled := true
	if r.Enabled != nil {
		enabled = *r.Enabled
	}
	return channelguide.Input{
		Channel: r.Channel, Title: r.Title, Summary: r.Summary, Markdown: r.Markdown,
		Steps: r.Steps, Requirements: r.Requirements, Enabled: enabled, SortOrder: r.SortOrder,
	}
}

type channelGuideListResponse struct {
	Channel       string `json:"channel"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Enabled       bool   `json:"enabled"`
	Builtin       bool   `json:"builtin"`
	SortOrder     int    `json:"sortOrder"`
	MarkdownBytes int    `json:"markdownBytes"`
	UpdatedAt     string `json:"updatedAt"`
}

type channelGuideDetailResponse struct {
	channelGuideListResponse
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
}

type publicChannelGuideResponse struct {
	Channel      string   `json:"channel"`
	Title        string   `json:"title"`
	Summary      string   `json:"summary"`
	Markdown     string   `json:"markdown"`
	Steps        []string `json:"steps"`
	Requirements []string `json:"requirements"`
	UpdatedAt    string   `json:"updatedAt"`
	Enabled      bool     `json:"enabled"`
	Builtin      bool     `json:"builtin"`
	Packageable  bool     `json:"packageable"`
	SortOrder    int      `json:"sortOrder"`
}

func (h *ChannelGuideHandler) PublicList(c echo.Context) error {
	items, err := h.Service.List(c.Request().Context(), false, "")
	if err != nil {
		return channelGuideError(c, err)
	}
	out := make([]publicChannelGuideResponse, 0, len(items))
	for _, item := range items {
		out = append(out, publicChannelGuideResponse{
			Channel: item.Channel, Title: item.Title, Summary: item.Summary, Markdown: item.Markdown,
			Steps: item.Steps, Requirements: item.Requirements,
			UpdatedAt: item.UpdatedAt.UTC().Format("2006-01-02"),
			Enabled: item.Enabled, Builtin: item.Builtin, Packageable: item.Packageable, SortOrder: item.SortOrder,
		})
	}
	return response.OK(c, map[string]any{"guides": out})
}

func (h *ChannelGuideHandler) List(c echo.Context) error {
	status := c.QueryParam("status")
	includeDisabled := status != "enabled"
	items, err := h.Service.List(c.Request().Context(), includeDisabled, c.QueryParam("keyword"))
	if err != nil {
		return channelGuideError(c, err)
	}
	out := make([]channelGuideListResponse, 0, len(items))
	for _, item := range items {
		if status == "disabled" && item.Enabled {
			continue
		}
		out = append(out, toChannelGuideListResponse(item))
	}
	return response.OK(c, map[string]any{"guides": out})
}

func (h *ChannelGuideHandler) Get(c echo.Context) error {
	item, err := h.Service.Get(c.Request().Context(), c.Param("channel"))
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Create(c echo.Context) error {
	var req channelGuideRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	item, err := h.Service.Create(c.Request().Context(), req.input())
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.Created(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Update(c echo.Context) error {
	channel := c.Param("channel")
	var req channelGuideRequest
	if err := c.Bind(&req); err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid request body")
	}
	if req.Channel != "" && req.Channel != channel {
		return response.Fail(c, http.StatusBadRequest, 40001, "channel cannot be changed")
	}
	item, err := h.Service.Update(c.Request().Context(), channel, req.input())
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func (h *ChannelGuideHandler) Delete(c echo.Context) error {
	if err := h.Service.Delete(c.Request().Context(), c.Param("channel")); err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, map[string]any{"deleted": true})
}

func (h *ChannelGuideHandler) UploadMarkdown(c echo.Context) error {
	c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, channelguide.MaxMarkdownBytes+64<<10)
	file, err := c.FormFile("file")
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	src, err := file.Open()
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	defer func() { _ = src.Close() }()
	data, err := io.ReadAll(io.LimitReader(src, channelguide.MaxMarkdownBytes+1))
	if err != nil {
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid markdown file")
	}
	item, err := h.Service.UploadMarkdown(c.Request().Context(), c.Param("channel"), file.Filename, data)
	if err != nil {
		return channelGuideError(c, err)
	}
	return response.OK(c, toChannelGuideDetailResponse(item))
}

func toChannelGuideListResponse(item channelguide.Item) channelGuideListResponse {
	return channelGuideListResponse{
		Channel: item.Channel, Title: item.Title, Summary: item.Summary, Enabled: item.Enabled,
		Builtin: item.Builtin, SortOrder: item.SortOrder, MarkdownBytes: len(item.Markdown),
		UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toChannelGuideDetailResponse(item channelguide.Item) channelGuideDetailResponse {
	return channelGuideDetailResponse{
		channelGuideListResponse: toChannelGuideListResponse(item),
		Markdown: item.Markdown, Steps: item.Steps, Requirements: item.Requirements,
	}
}

func channelGuideError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, channelguide.ErrInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid channel guide")
	case errors.Is(err, channelguide.ErrNotFound):
		return response.Fail(c, http.StatusNotFound, 40411, "channel guide not found")
	case errors.Is(err, channelguide.ErrConflict), errors.Is(err, channelguide.ErrBuiltinDelete):
		return response.Fail(c, http.StatusConflict, 40905, "channel guide conflict")
	case errors.Is(err, channelguide.ErrTooLarge):
		return response.Fail(c, http.StatusRequestEntityTooLarge, 41303, "channel guide markdown is too large")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "channel guide request failed")
	}
}
```

删除 `handler/channel_package.go` 中的旧 `Guides` 方法及其不再需要的 `guides` import。同时删除 `handler/channel_package_test.go` 中的 `TestChannelGuidesReturnsThreeChannels`，公开指引已由 `channel_guide_test.go` 覆盖。

- [ ] **Step 4: 注册路由**

`router/control.go` 函数签名在 `channelPackages *handler.ChannelPackageHandler` 后加入 `channelGuides *handler.ChannelGuideHandler`。

把原来的公开路由：

```go
if channelPackages != nil {
	api.GET("/channel-guides", channelPackages.Guides, authn.RequireUser)
}
```

替换为：

```go
if channelGuides != nil {
	api.GET("/channel-guides", channelGuides.PublicList, authn.RequireUser)
}
```

然后在 `adminProtected := adminAPI.Group("", authn.RequireOps)` 与 `adminProtected.Use(...)` 之后（与 `skillManager` 等 admin 路由同区域）加入：

```go
if channelGuides != nil {
	adminProtected.GET("/channel-guides", channelGuides.List)
	adminProtected.GET("/channel-guides/:channel", channelGuides.Get)
	adminProtected.POST("/channel-guides", channelGuides.Create)
	adminProtected.PUT("/channel-guides/:channel", channelGuides.Update)
	adminProtected.DELETE("/channel-guides/:channel", channelGuides.Delete)
	adminProtected.POST("/channel-guides/:channel/markdown", channelGuides.UploadMarkdown)
}
```

- [ ] **Step 5: 更新 CP 启动装配**

`cmd/cp/main.go` 在 `channelPackageService := ...` 之前加入：

```go
channelGuideService := channelguide.NewService(dao.NewChannelGuideRepository(db))
if err := channelGuideService.EnsureDefaults(ctx); err != nil {
	obs.Logger.Warn("channel guide seed failed", "error", err)
}
channelGuideHandler := handler.NewChannelGuideHandler(channelGuideService)
```

在 `channelpkg.ServiceConfig` 中加入 `Guides: channelGuideService`。在 `router.RegisterControl` 调用中，把 `channelPackageHandler` 后面补上 `channelGuideHandler`。

- [ ] **Step 6: 更新 router 测试辅助**

`router/helpers_test.go` 的 `newCP` 中构造：

```go
guideService := channelguide.NewService(channelguide.NewMemoryRepository())
if err := guideService.EnsureDefaults(context.Background()); err != nil {
	t.Fatal(err)
}
guideHandler := handler.NewChannelGuideHandler(guideService)
```

并在 `RegisterControl` 的 `handler.NewChannelPackageHandler(nil, projects)` 后传入 `guideHandler`，其余 nil 参数数量保持不变。

- [ ] **Step 7: 增加路由隔离测试**

`router/channel_guide_test.go`：

```go
package router

import (
	"net/http"
	"strings"
	"testing"

	"vibebiz/internal/store"
)

func TestCPChannelGuideAdminRoutes(t *testing.T) {
	e, _, authState := newCP(t, store.NewMemoryProjectStore())

	rec := serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/channel-guides", authState.opsCookie)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "wechat_minigame") {
		t.Fatalf("admin list status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithAuthBody(e, http.MethodPost, "/cp/api/v1/admin/channel-guides", authState.opsCookie, `{"channel":"taptap_mobile","title":"TapTap","markdown":"# TapTap","enabled":true,"sortOrder":40}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create status = %d body = %s", rec.Code, rec.Body.String())
	}

	rec = serveWithAuth(e, http.MethodGet, "/cp/api/v1/admin/channel-guides", authState.userCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user admin status = %d, want 401", rec.Code)
	}
}

func TestDPDoesNotExposeChannelGuideAdminRoutes(t *testing.T) {
	e, _, authState := newDP(t, "http://127.0.0.1:1")
	rec := serveWithAuth(e, http.MethodGet, "/dp/api/v1/admin/channel-guides", authState.opsCookie)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}
```

在 `TestDPDoesNotExposeChannelPackageRoutes` 的路径列表中加入 `{http.MethodGet, "/dp/api/v1/channel-guides"}`（应已覆盖公开接口）。

- [ ] **Step 8: 运行测试**

Run: `cd packages/backend && gofmt -w . && go test ./internal/handler ./internal/router ./cmd/cp`
Expected: PASS。

- [ ] **Step 9: 提交**

```bash
git add packages/backend/internal/handler/channel_guide.go packages/backend/internal/handler/channel_guide_test.go packages/backend/internal/handler/channel_package.go packages/backend/internal/handler/channel_package_test.go packages/backend/internal/router/control.go packages/backend/internal/router/helpers_test.go packages/backend/internal/router/channel_guide_test.go packages/backend/internal/router/channel_package_test.go packages/backend/cmd/cp/main.go
git commit -m "feat: add channel guide admin API"
```

---

## Task 4: 渠道包生成注入数据库指引

**Files:**
- Modify: `packages/backend/internal/channelpkg/core/types.go`
- Modify: `packages/backend/internal/channelpkg/service.go`
- Modify: `packages/backend/internal/channelpkg/service_test.go`
- Modify: `packages/backend/internal/channelpkg/adapters/wechat/adapter.go`
- Modify: `packages/backend/internal/channelpkg/adapters/douyin/adapter.go`
- Modify: `packages/backend/internal/channelpkg/adapters/steam/adapter.go`
- Modify: `packages/backend/internal/channelpkg/adapters/wechat/adapter_test.go`

**Interfaces:**
- Produces: `core.SourceInfo.Guide string`。
- Produces: `channelpkg.GuideProvider` 接口与 `ServiceConfig.Guides GuideProvider`。
- Consumes: Task 2 的 `channelguide.Service.Markdown` 满足 `GuideProvider`。

- [ ] **Step 1: 写失败测试**

在 `service_test.go` 的 `newTestService` 附近加入：

```go
type fakeGuideProvider struct {
	markdown string
	err      error
}

func (f fakeGuideProvider) Markdown(context.Context, string) (string, error) {
	return f.markdown, f.err
}

func TestGenerateUsesProvidedGuide(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c')`)
	repo := newFakeRepo()
	var got string
	adapter := fakeAdapter{level: core.LevelB, build: func(_ context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error) {
		got = src.Guide
		return writeFakePackage(out, src)
	}}
	svc := newTestService(repo, root, adapter, func(cfg *ServiceConfig) {
		cfg.Guides = fakeGuideProvider{markdown: "database guide"}
	})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); err != nil {
		t.Fatal(err)
	}
	if got != "database guide" {
		t.Fatalf("guide = %q, want database guide", got)
	}
}

func TestGenerateFallsBackWhenGuideProviderErrors(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "p1")
	write(t, dir, "index.html", `<canvas></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('c')`)
	repo := newFakeRepo()
	var got string
	adapter := fakeAdapter{level: core.LevelB, build: func(_ context.Context, src core.SourceInfo, out io.Writer) (core.Manifest, error) {
		got = src.Guide
		return writeFakePackage(out, src)
	}}
	svc := newTestService(repo, root, adapter, func(cfg *ServiceConfig) {
		cfg.Guides = fakeGuideProvider{err: errors.New("db down")}
	})
	if _, err := svc.Generate(context.Background(), &model.Project{ID: "p1", WorkspaceID: 3}, GenerateInput{Channel: core.ChannelWeChat, SourceType: core.SourceHTML5}); err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Fatalf("guide = %q, want empty fallback marker", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/channelpkg -run Guide -v`
Expected: FAIL，`GuideProvider` 或 `SourceInfo.Guide` 不存在。

- [ ] **Step 3: 增加字段与接口**

`core/types.go` 的 `SourceInfo` 增加：

```go
Guide string `json:"-"`
```

`channelpkg/service.go` 顶部增加：

```go
type GuideProvider interface {
	Markdown(ctx context.Context, channel string) (string, error)
}
```

`ServiceConfig` 增加 `Guides GuideProvider`。

在 `Generate` 中，`info.Report = report` 之前加入：

```go
if s.cfg.Guides != nil {
	if guide, guideErr := s.cfg.Guides.Markdown(buildCtx, input.Channel); guideErr == nil && strings.TrimSpace(guide) != "" {
		info.Guide = guide
	}
}
```

- [ ] **Step 4: 适配器优先使用 source.Guide**

`wechat/adapter.go` 和 `douyin/adapter.go`：

- 把两处 `return writeGuide(written)` 改为 `return writeGuide(source, written)`。
- 把 `writeGuide` 改为：

```go
func writeGuide(source core.SourceInfo, written func(string, []byte) error) error {
	guide := source.Guide
	if strings.TrimSpace(guide) == "" {
		guide = guides.Markdown(core.ChannelWeChat) // douyin 包改成 core.ChannelDouyin
	}
	if strings.TrimSpace(guide) == "" {
		return errors.New("wechat: missing channel guide") // douyin 包改成 douyin:
	}
	return written("CHANNEL_GUIDE.md", []byte(guide))
}
```

`steam/adapter.go` 中把：

```go
guide := guides.Markdown(core.ChannelSteam)
```

改为：

```go
guide := source.Guide
if strings.TrimSpace(guide) == "" {
	guide = guides.Markdown(core.ChannelSteam)
}
```

- [ ] **Step 5: 增加适配器覆盖测试**

在 `wechat/adapter_test.go` 的第一个 Build 测试后增加：

```go
func TestWeChatBuildUsesProvidedGuide(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "index.html", `<canvas id="game"></canvas><script src="game.js"></script>`)
	write(t, dir, "game.js", `document.getElementById('game').getContext('2d')`)
	var buf bytes.Buffer
	_, err := New().Build(context.Background(), core.SourceInfo{
		Dir: dir, Entry: "index.html", Version: "v1",
		Report: core.Report{Level: core.LevelB},
		Guide:  "# database guide",
	}, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if got := zipContent(t, buf.Bytes(), "CHANNEL_GUIDE.md"); got != "# database guide" {
		t.Fatalf("guide = %q", got)
	}
}
```

- [ ] **Step 6: 运行测试**

Run: `cd packages/backend && gofmt -w . && go test ./internal/channelpkg/...`
Expected: PASS。

- [ ] **Step 7: 提交**

```bash
git add packages/backend/internal/channelpkg
git commit -m "feat: use managed guides in channel packages"
```

---

## Task 5: ops-web 类型、API 与工具函数

**Files:**
- Modify: `packages/ops-web/src/types/index.ts`
- Modify: `packages/ops-web/src/api/admin.ts`
- Create: `packages/ops-web/src/utils/channelGuides.ts`
- Create: `packages/ops-web/src/utils/channelGuides.test.ts`
- Create: `packages/ops-web/src/api/channelGuides.test.ts`

**Interfaces:**
- Produces: `AdminChannelGuide`、`AdminChannelGuideDetail`、`ChannelGuideInput`、`ChannelGuideCreateInput`、`ChannelGuideStatus`。
- Produces: `getChannelGuides`、`getChannelGuide`、`createChannelGuide`、`updateChannelGuide`、`deleteChannelGuide`、`uploadChannelGuideMarkdown`。
- Produces: `splitLines(value: string): string[]`、`isMarkdownFile(name: string): boolean`。

- [ ] **Step 1: 写失败测试**

`utils/channelGuides.test.ts`：

```ts
import { describe, expect, it } from 'vitest'

import { isMarkdownFile, splitLines } from './channelGuides'

describe('channel guide helpers', () => {
  it('splits non-empty trimmed lines', () => {
    expect(splitLines(' a \n\n b\r\nc ')).toEqual(['a', 'b', 'c'])
  })

  it('accepts markdown extensions only', () => {
    expect(isMarkdownFile('guide.md')).toBe(true)
    expect(isMarkdownFile('guide.MARKDOWN')).toBe(true)
    expect(isMarkdownFile('guide.txt')).toBe(false)
  })
})
```

`api/channelGuides.test.ts`：

```ts
import { describe, expect, it, vi } from 'vitest'

import {
  createChannelGuide,
  deleteChannelGuide,
  getChannelGuide,
  getChannelGuides,
  updateChannelGuide,
} from './admin'
import { http } from './client'

describe('channel guide admin api', () => {
  it('lists channel guides with filters', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { guides: [] } },
    })
    await getChannelGuides({ keyword: '微信', status: 'enabled' })
    expect(spy).toHaveBeenCalledWith({
      method: 'get',
      url: '/channel-guides',
      params: { keyword: '微信', status: 'enabled' },
    })
  })

  it('loads a detail', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'wechat_minigame', markdown: '# x' } },
    })
    await expect(getChannelGuide('wechat_minigame')).resolves.toMatchObject({
      channel: 'wechat_minigame',
    })
  })

  it('creates and updates a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'taptap_mobile' } },
    })
    await createChannelGuide({
      channel: 'taptap_mobile',
      title: 'TapTap',
      summary: '',
      markdown: '# TapTap',
      steps: [],
      requirements: [],
      enabled: true,
      sortOrder: 40,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'post',
      url: '/channel-guides',
      data: expect.objectContaining({ channel: 'taptap_mobile' }),
    })
  })

  it('deletes a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { deleted: true } },
    })
    await deleteChannelGuide('taptap_mobile')
    expect(spy).toHaveBeenCalledWith({
      method: 'delete',
      url: '/channel-guides/taptap_mobile',
    })
  })

  it('updates a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'wechat_minigame' } },
    })
    await updateChannelGuide('wechat_minigame', {
      title: '微信',
      summary: '',
      markdown: '# x',
      steps: [],
      requirements: [],
      enabled: false,
      sortOrder: 10,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/channel-guides/wechat_minigame',
      data: expect.objectContaining({ enabled: false }),
    })
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm test -- channelGuides`
Expected: FAIL，模块不存在。

- [ ] **Step 3: 增加类型与工具函数**

在 `types/index.ts` 增加：

```ts
export type ChannelGuideStatus = 'all' | 'enabled' | 'disabled'

export interface AdminChannelGuide {
  channel: string
  title: string
  summary: string
  enabled: boolean
  builtin: boolean
  sortOrder: number
  markdownBytes: number
  updatedAt: string
}

export interface AdminChannelGuideDetail extends AdminChannelGuide {
  markdown: string
  steps: string[]
  requirements: string[]
}

export interface ChannelGuideInput {
  title: string
  summary: string
  markdown: string
  steps: string[]
  requirements: string[]
  enabled: boolean
  sortOrder: number
}

export interface ChannelGuideCreateInput extends ChannelGuideInput {
  channel: string
}
```

`utils/channelGuides.ts`：

```ts
export function splitLines(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)
}

export function isMarkdownFile(name: string): boolean {
  return /\.(md|markdown)$/i.test(name)
}
```

- [ ] **Step 4: 增加 API 函数**

在 `api/admin.ts` 的类型 import 中加入 `AdminChannelGuide`、`AdminChannelGuideDetail`、`ChannelGuideCreateInput`、`ChannelGuideInput`、`ChannelGuideStatus`。文件末尾追加：

```ts
export async function getChannelGuides(query: {
  keyword?: string
  status?: ChannelGuideStatus
} = {}): Promise<AdminChannelGuide[]> {
  const result = await request<{ guides: AdminChannelGuide[] }>({
    method: 'get',
    url: '/channel-guides',
    params: { keyword: query.keyword || undefined, status: query.status || undefined },
  })
  return result.guides ?? []
}

export function getChannelGuide(channel: string): Promise<AdminChannelGuideDetail> {
  return request<AdminChannelGuideDetail>({
    method: 'get',
    url: `/channel-guides/${encodeURIComponent(channel)}`,
  })
}

export function createChannelGuide(input: ChannelGuideCreateInput): Promise<AdminChannelGuideDetail> {
  return request<AdminChannelGuideDetail>({ method: 'post', url: '/channel-guides', data: input })
}

export function updateChannelGuide(
  channel: string,
  input: ChannelGuideInput,
): Promise<AdminChannelGuideDetail> {
  return request<AdminChannelGuideDetail>({
    method: 'put',
    url: `/channel-guides/${encodeURIComponent(channel)}`,
    data: input,
  })
}

export async function deleteChannelGuide(channel: string): Promise<void> {
  await request<{ deleted: boolean }>({
    method: 'delete',
    url: `/channel-guides/${encodeURIComponent(channel)}`,
  })
}

export function uploadChannelGuideMarkdown(
  channel: string,
  file: File,
): Promise<AdminChannelGuideDetail> {
  const form = new FormData()
  form.set('file', file)
  return request<AdminChannelGuideDetail>({
    method: 'post',
    url: `/channel-guides/${encodeURIComponent(channel)}/markdown`,
    data: form,
  })
}
```

- [ ] **Step 5: 运行测试**

Run: `cd packages/ops-web && npm test -- channelGuides`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add packages/ops-web/src/types/index.ts packages/ops-web/src/api/admin.ts packages/ops-web/src/api/channelGuides.test.ts packages/ops-web/src/utils/channelGuides.ts packages/ops-web/src/utils/channelGuides.test.ts
git commit -m "feat: add ops channel guide api"
```

---

## Task 6: ops-web 发布渠道管理页、路由与导航

**Files:**
- Create: `packages/ops-web/src/views/ChannelGuidesView.vue`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: Task 5 的 API、类型与 `splitLines`/`isMarkdownFile`。
- Produces: 路由 `channel-guides` 与导航「发布渠道」。

- [ ] **Step 1: 更新测试**

`navigation.test.ts` 的路径列表在 `/templates` 后加入 `/channel-guides`，标签列表在 `模板管理` 后加入 `发布渠道`。路由名称列表会自动跟随 `navItems`，无需单独改断言。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/ops-web && npm test -- navigation`
Expected: FAIL，顺序不匹配或路由缺失。

- [ ] **Step 3: 实现页面**

`views/ChannelGuidesView.vue`：

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createChannelGuide,
  deleteChannelGuide,
  getChannelGuide,
  getChannelGuides,
  updateChannelGuide,
  uploadChannelGuideMarkdown,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import type {
  AdminChannelGuide,
  ChannelGuideCreateInput,
  ChannelGuideStatus,
  TableColumn,
  TableRow,
} from '@/types'
import { formatDateTime } from '@/utils/format'
import { isMarkdownFile, splitLines } from '@/utils/channelGuides'

interface GuideForm {
  channel: string
  title: string
  summary: string
  markdown: string
  stepsText: string
  requirementsText: string
  enabled: boolean
  sortOrder: number
}

const rows = ref<AdminChannelGuide[]>([])
const loading = ref(false)
const error = ref('')
const notice = ref('')
const keyword = ref('')
const status = ref<ChannelGuideStatus>('all')

const dialogVisible = ref(false)
const saving = ref(false)
const uploading = ref(false)
const actionError = ref('')
const editingChannel = ref('')
const form = ref<GuideForm>(emptyForm())
const fileInput = ref<HTMLInputElement | null>(null)

const statusOptions: { label: string; value: ChannelGuideStatus }[] = [
  { label: '全部状态', value: 'all' },
  { label: '已启用', value: 'enabled' },
  { label: '已停用', value: 'disabled' },
]

const columns: TableColumn[] = [
  { key: 'channel', label: '渠道标识', width: '180px' },
  { key: 'title', label: '名称', width: '140px' },
  {
    key: 'builtin',
    label: '类型',
    width: '80px',
    format: (value) => (value ? '内置' : '自定义'),
  },
  {
    key: 'enabled',
    label: '状态',
    width: '90px',
    tag: true,
    highlight: ['true'],
    format: (value) => (value ? '启用' : '停用'),
  },
  { key: 'sortOrder', label: '排序', width: '70px' },
  {
    key: 'markdownBytes',
    label: '正文大小',
    width: '100px',
    format: (value) => `${value ?? 0} B`,
  },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '220px' },
]

const markdownBytes = computed(() => new TextEncoder().encode(form.value.markdown).length)

function emptyForm(): GuideForm {
  return {
    channel: '',
    title: '',
    summary: '',
    markdown: '',
    stepsText: '',
    requirementsText: '',
    enabled: true,
    sortOrder: 100,
  }
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    rows.value = await getChannelGuides({
      keyword: keyword.value.trim() || undefined,
      status: status.value,
    })
  } catch (err) {
    error.value = errorMessage(err, '加载发布渠道失败')
  } finally {
    loading.value = false
  }
}

function search() {
  void load()
}

function openCreate() {
  editingChannel.value = ''
  form.value = emptyForm()
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

async function openEdit(row: TableRow) {
  const item = row as AdminChannelGuide
  actionError.value = ''
  notice.value = ''
  try {
    const detail = await getChannelGuide(item.channel)
    editingChannel.value = detail.channel
    form.value = {
      channel: detail.channel,
      title: detail.title,
      summary: detail.summary,
      markdown: detail.markdown,
      stepsText: detail.steps.join('\n'),
      requirementsText: detail.requirements.join('\n'),
      enabled: detail.enabled,
      sortOrder: detail.sortOrder,
    }
    dialogVisible.value = true
  } catch (err) {
    notice.value = errorMessage(err, '加载渠道详情失败')
  }
}

function payload() {
  return {
    title: form.value.title,
    summary: form.value.summary,
    markdown: form.value.markdown,
    steps: splitLines(form.value.stepsText),
    requirements: splitLines(form.value.requirementsText),
    enabled: form.value.enabled,
    sortOrder: form.value.sortOrder,
  }
}

async function submit() {
  if (!form.value.title.trim()) {
    actionError.value = '渠道名称不能为空'
    return
  }
  if (!editingChannel.value && !/^[a-z][a-z0-9_]{1,31}$/.test(form.value.channel.trim())) {
    actionError.value = '渠道标识需为小写字母开头，可包含小写字母、数字和下划线'
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    if (editingChannel.value) {
      await updateChannelGuide(editingChannel.value, payload())
      notice.value = '渠道已更新'
    } else {
      const input: ChannelGuideCreateInput = { channel: form.value.channel.trim(), ...payload() }
      await createChannelGuide(input)
      notice.value = '渠道已创建'
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = errorMessage(err, '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggle(row: TableRow) {
  const item = row as AdminChannelGuide
  try {
    const detail = await getChannelGuide(item.channel)
    await updateChannelGuide(item.channel, {
      title: detail.title,
      summary: detail.summary,
      markdown: detail.markdown,
      steps: detail.steps,
      requirements: detail.requirements,
      enabled: !detail.enabled,
      sortOrder: detail.sortOrder,
    })
    await load()
  } catch (err) {
    notice.value = errorMessage(err, '切换状态失败')
  }
}

async function remove(row: TableRow) {
  const item = row as AdminChannelGuide
  if (!window.confirm(`确定删除渠道「${item.title}」吗？`)) {
    return
  }
  try {
    await deleteChannelGuide(item.channel)
    notice.value = '渠道已删除'
    await load()
  } catch (err) {
    notice.value = errorMessage(err, '删除失败')
  }
}

function chooseFile() {
  fileInput.value?.click()
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!isMarkdownFile(file.name)) {
    actionError.value = '只支持 .md 或 .markdown 文件'
    return
  }
  actionError.value = ''
  if (editingChannel.value) {
    uploading.value = true
    try {
      const detail = await uploadChannelGuideMarkdown(editingChannel.value, file)
      form.value.markdown = detail.markdown
      notice.value = 'markdown 已上传并覆盖正文'
    } catch (err) {
      actionError.value = errorMessage(err, '上传失败')
    } finally {
      uploading.value = false
    }
    return
  }
  try {
    form.value.markdown = await file.text()
  } catch {
    actionError.value = '读取文件失败'
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>发布渠道</h1>
        <p class="page-desc">管理前台展示的渠道信息、资质、步骤和 markdown 指引。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 个</span>
        <Button label="新增渠道" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <div class="filters">
      <div class="search-row">
        <InputText v-model="keyword" placeholder="搜索标识、名称或简介" @keyup.enter="search" />
        <Select
          v-model="status"
          :options="statusOptions"
          option-label="label"
          option-value="value"
          aria-label="渠道状态"
          @change="search"
        />
        <Button label="搜索" icon="pi pi-search" @click="search" />
      </div>
    </div>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无发布渠道"
      @retry="load"
    >
      <template #cell-actions="{ row }">
        <Button label="编辑" size="small" text @click="openEdit(row)" />
        <Button
          :label="(row as AdminChannelGuide).enabled ? '停用' : '启用'"
          size="small"
          text
          @click="toggle(row)"
        />
        <Button
          v-if="!(row as AdminChannelGuide).builtin"
          label="删除"
          size="small"
          text
          @click="remove(row)"
        />
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      :header="editingChannel ? '编辑渠道' : '新增渠道'"
      modal
      :style="{ width: '760px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>渠道标识</span>
          <InputText
            v-model="form.channel"
            :disabled="Boolean(editingChannel)"
            maxlength="32"
            placeholder="taptap_mobile"
          />
          <small>创建后不可修改；内置渠道不可删除。</small>
        </label>
        <div class="field-grid">
          <label class="field">
            <span>名称</span>
            <InputText v-model="form.title" maxlength="64" />
          </label>
          <label class="field">
            <span>排序</span>
            <InputText v-model.number="form.sortOrder" type="number" min="0" max="9999" />
          </label>
        </div>
        <label class="field">
          <span>简介</span>
          <InputText v-model="form.summary" maxlength="255" />
        </label>
        <div class="field-grid">
          <label class="field">
            <span>发布步骤（每行一条）</span>
            <Textarea v-model="form.stepsText" rows="5" />
          </label>
          <label class="field">
            <span>资质要求（每行一条）</span>
            <Textarea v-model="form.requirementsText" rows="5" />
          </label>
        </div>
        <label class="field">
          <span>指引 markdown（{{ markdownBytes }} B / 524288 B）</span>
          <Textarea v-model="form.markdown" rows="14" />
        </label>
        <div class="upload-row">
          <input
            ref="fileInput"
            type="file"
            accept=".md,.markdown,text/markdown,text/plain"
            hidden
            @change="onFileChange"
          />
          <Button
            label="上传 markdown"
            icon="pi pi-upload"
            severity="secondary"
            :loading="uploading"
            @click="chooseFile"
          />
          <small>编辑已有渠道时上传会立即覆盖数据库正文；新增渠道时先读入文本，保存后生效。</small>
        </div>
        <label class="field-inline">
          <Checkbox v-model="form.enabled" binary />
          <span>在前台设置页展示</span>
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>

      <template #footer>
        <Button label="取消" severity="secondary" text @click="dialogVisible = false" />
        <Button label="保存" :loading="saving" @click="submit" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.inline-notice {
  margin-bottom: 16px;
  color: var(--blue);
  font-size: 13px;
}

.filters {
  margin-bottom: 12px;
}

.search-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.search-row :deep(input) {
  width: 280px;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--text);
  font-size: 13px;
}

.field span {
  color: var(--muted);
  font-size: 12px;
}

.field small,
.upload-row small {
  color: var(--muted);
  font-size: 11px;
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.field-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
}

.upload-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
</style>
```

- [ ] **Step 4: 添加路由与导航**

`routes.ts` 在 `TemplatesView` import 后加入 `ChannelGuidesView`；在 templates 路由后加入：

```ts
{
  path: 'channel-guides',
  name: 'channel-guides',
  component: ChannelGuidesView,
  meta: { title: '发布渠道' },
},
```

`navigation.ts` 在 `templates` 项后加入：

```ts
{ name: 'channel-guides', label: '发布渠道', path: '/channel-guides', icon: 'pi pi-share-alt' },
```

- [ ] **Step 5: 运行测试和构建**

Run: `cd packages/ops-web && npm test && npm run lint && npm run build`
Expected: PASS。

- [ ] **Step 6: 提交**

```bash
git add packages/ops-web/src/views/ChannelGuidesView.vue packages/ops-web/src/router/routes.ts packages/ops-web/src/router/navigation.ts packages/ops-web/src/router/navigation.test.ts
git commit -m "feat: add channel guide management page"
```

---

## Task 7: web 前台类型、发布弹窗与测试更新

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/utils/channelPackages.ts`
- Modify: `packages/web/src/utils/channelPackages.test.ts`
- Modify: `packages/web/src/components/editor/ChannelPackagePanel.vue`
- Modify: `packages/web/src/views/settings/ChannelGuideView.test.ts`
- Modify: `packages/web/src/api/channelPackages.test.ts`

**Interfaces:**
- Consumes: CP 公开接口新增的 `enabled`、`builtin`、`packageable`、`sortOrder`。
- Produces: `packageableChannelOptions(guides: ChannelGuide[]): ChannelOption[]`，接口失败或无包时回退内置三渠道。

- [ ] **Step 1: 写失败测试**

在 `utils/channelPackages.test.ts` 加入：

```ts
import type { ChannelGuide } from '@/types'
import { packageableChannelOptions } from './channelPackages'

const guides: ChannelGuide[] = [
  {
    channel: 'wechat_minigame', title: '微信小游戏', summary: '', markdown: '',
    steps: [], requirements: [], updatedAt: '2026-11-08',
    enabled: true, builtin: true, packageable: true, sortOrder: 10,
  },
  {
    channel: 'taptap_mobile', title: 'TapTap', summary: '', markdown: '',
    steps: [], requirements: [], updatedAt: '2026-11-08',
    enabled: true, builtin: false, packageable: false, sortOrder: 40,
  },
]

it('filters packageable channels and falls back to builtins', () => {
  expect(packageableChannelOptions(guides).map((item) => item.value)).toEqual(['wechat_minigame'])
  expect(packageableChannelOptions([]).map((item) => item.value)).toEqual([
    'wechat_minigame',
    'douyin_minigame',
    'steam_desktop',
  ])
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- channelPackages`
Expected: FAIL，`packageableChannelOptions` 不存在。

- [ ] **Step 3: 更新类型**

`types/index.ts` 的 `ChannelGuide` 改为：

```ts
export interface ChannelGuide {
  channel: string
  title: string
  summary: string
  markdown: string
  steps: string[]
  requirements: string[]
  updatedAt: string
  enabled: boolean
  builtin: boolean
  packageable: boolean
  sortOrder: number
}
```

- [ ] **Step 4: 增加过滤函数**

`utils/channelPackages.ts` 把现有 `import type { ChannelPackageChannel, ChannelPackageLevel } from '@/types'` 扩展为同时导入 `ChannelGuide`，然后增加：

```ts
export interface ChannelOption {
  value: ChannelPackageChannel
  label: string
}

const packageableValues = new Set<ChannelPackageChannel>(
  channelOptions.map((item) => item.value),
)

export function packageableChannelOptions(guides: ChannelGuide[]): ChannelOption[] {
  const options = guides
    .filter((guide) => guide.packageable && packageableValues.has(guide.channel as ChannelPackageChannel))
    .map((guide) => ({ value: guide.channel as ChannelPackageChannel, label: guide.title }))
  return options.length > 0 ? options : channelOptions
}
```

把 `channelOptions` 的显式类型改为 `ChannelOption[]`。

- [ ] **Step 5: 更新发布弹窗**

`ChannelPackagePanel.vue`：

- 从 `@/api/channelPackages` 增加 `listChannelGuides` import。
- 从 `@/utils/channelPackages` 增加 `packageableChannelOptions` 与 `ChannelOption`。
- 增加：

```ts
const options = ref<ChannelOption[]>(channelOptions)

async function loadChannels() {
  try {
    options.value = packageableChannelOptions(await listChannelGuides())
  } catch {
    options.value = channelOptions
  }
  if (!options.value.some((item) => item.value === channel.value)) {
    channel.value = options.value[0]?.value ?? 'wechat_minigame'
  }
}
```

- `onMounted` 改为：

```ts
onMounted(() => {
  void load()
  void loadChannels()
})
```

- 模板中 `v-for="item in channelOptions"` 改为 `v-for="item in options"`。

- [ ] **Step 6: 更新测试夹具**

在 `ChannelGuideView.test.ts` 与 `api/channelPackages.test.ts` 的每个 `ChannelGuide` 对象中补上：

```ts
enabled: true,
builtin: true,
packageable: true,
sortOrder: 10,
```

同一文件多个渠道按 10/20/30 递增。

- [ ] **Step 7: 运行 web 测试与构建**

Run: `cd packages/web && npm test && npm run lint && npm run build`
Expected: PASS。

- [ ] **Step 8: 提交**

```bash
git add packages/web/src/types/index.ts packages/web/src/utils/channelPackages.ts packages/web/src/utils/channelPackages.test.ts packages/web/src/components/editor/ChannelPackagePanel.vue packages/web/src/views/settings/ChannelGuideView.test.ts packages/web/src/api/channelPackages.test.ts
git commit -m "feat: drive channel options from managed guides"
```

---

## Task 8: 全量验证与手工验收

**Files:**
- 无新增文件；只验证。

- [ ] **Step 1: 后端全量测试**

Run: `cd packages/backend && gofmt -w . && go vet ./... && go test ./...`
Expected: PASS。

- [ ] **Step 2: 前端测试与构建**

Run:

```bash
cd packages/ops-web && npm test && npm run lint && npm run build
cd packages/web && npm test && npm run lint && npm run build
```

Expected: 全部 PASS。

- [ ] **Step 3: 仓库质量门禁**

Run: `make validate`
Expected: PASS。

- [ ] **Step 4: 手工验收（本地数据库可用时）**

```bash
make db-migrate-up
```

1. 启动 `make dev`。
2. ops-web 打开 `/channel-guides`，确认三个内置渠道存在。
3. 编辑微信指引，保存后 web 设置页刷新看到新内容。
4. 生成微信渠道包，解压检查 `CHANNEL_GUIDE.md` 与编辑内容一致。
5. 停用抖音，确认 web 设置页和发布弹窗都不再展示抖音。
6. 新增 `taptap_mobile`，确认只在设置页展示，不出现在发布弹窗。
7. 对内置渠道点击删除应提示冲突；自定义渠道可删除。
8. 上传 markdown 后正文立即更新。

- [ ] **Step 5: 任务收尾**

按仓库要求执行：

```bash
make checkpoint
```

有可复用经验时再执行 `make memory-add`。

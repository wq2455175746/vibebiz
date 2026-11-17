# 名片评论与点赞 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为发现页名片增加可编辑/删除的树形评论，以及先写 Redis Stream、每小时同步 MySQL 的点赞能力。

**Architecture:** 评论直接写 MySQL，按根评论游标分页、回复懒加载、深度上限 3；点赞写 Redis 计数/用户集合/Stream，asynq 每小时触发批量消费落库，MySQL 唯一键与 `COUNT(*)` 重算保证幂等。CP 提供评论与点赞 API，web 在名片详情页渲染评论区，发现页卡片只展示计数和点赞。

**Tech Stack:** Go 1.24 + Echo v4 + GORM + MySQL 8 + goose + Redis go-redis v9 + asynq；Vue 3 + Pinia + Vitest。

**Spec:** `docs/superpowers/specs/2026-11-16-business-card-comments-likes-design.md`

## Global Constraints

- 只改 `packages/backend` 与 `packages/web`；不改 sandbox、DP、ops-web。
- 评论入口必须在名片详情页；发现页卡片不展开评论。
- 当前版本只有作者本人可以编辑/删除自己的评论；名片主人和 ops 删除放二期。
- 评论最大深度 3；根评论 20 条/页；回复 10 条/页；单卡片最多渲染 200 个节点。
- 评论正文纯文本，去首尾空白，1..500 字；禁止 HTML/Markdown。
- 点赞先写 Redis；`PUT` 表示点赞，`DELETE` 表示取消点赞，必须幂等。
- Redis Stream 一期单流、单消费者；后续按 `cardId % N` 分片。
- 点赞同步每小时执行一次；MySQL 唯一键 `(card_id, user_id)`；受影响卡片用 `COUNT(*)` 重算。
- API 统一信封 `{code,message,result}`；成功 `code=0`。
- 公开评论读取不需要登录；写评论、点赞需要用户登录。
- 颜色只允许黑/白/灰/蓝。
- 计划中的 `git commit` 步骤只在用户明确要求提交时执行；否则跳过提交。
- 最终验证：`go test ./...`、`go vet ./...`、web `npm test -- --run && npm run build && npm run lint`、根目录 `make validate`。

---

## 文件结构

```
packages/backend/
├── migrations/00034_business_card_social.sql
├── migrations/migrations_test.go
├── internal/dao/model/business_card.go              # 增加 LikeCount
├── internal/dao/model/business_card_comment.go
├── internal/dao/model/business_card_like.go
├── internal/businesscard/
│   ├── comment_repository.go
│   ├── comment_repository_test.go
│   ├── comment_service.go
│   ├── comment_service_test.go
│   ├── comment_rate_limit.go
│   ├── like_store.go
│   ├── like_store_test.go
│   ├── like_service.go
│   ├── like_service_test.go
│   ├── like_repository.go
│   ├── like_sync.go
│   └── like_sync_test.go
├── internal/handler/card_comment.go
├── internal/handler/card_comment_test.go
├── internal/handler/card_like.go
├── internal/handler/card_like_test.go
├── internal/handler/card.go                          # 注入点赞状态
├── internal/router/control.go
├── internal/config/config.go
├── internal/config/config_test.go
└── cmd/cp/main.go

packages/web/src/
├── types/index.ts
├── utils/card.ts
├── api/cards.ts
├── api/cards.test.ts
├── utils/comment.ts
├── utils/comment.test.ts
├── stores/comment.ts
├── stores/comment.test.ts
├── components/card/CardCommentTree.vue
├── components/card/CardCommentNode.vue
├── components/card/CardCommentTree.test.ts
├── components/card/CardLikeButton.vue
├── components/card/CardLikeButton.test.ts
├── components/card/CardTile.vue
├── components/card/CardTile.test.ts
├── components/card/DiscoverPanel.vue
├── views/CardShareView.vue
└── views/CardShareView.test.ts
```

---

### Task 1: 迁移、DAO 模型与迁移测试

**Files:**
- Create: `packages/backend/migrations/00034_business_card_social.sql`
- Create: `packages/backend/internal/dao/model/business_card_comment.go`
- Create: `packages/backend/internal/dao/model/business_card_like.go`
- Modify: `packages/backend/internal/dao/model/business_card.go`
- Modify: `packages/backend/migrations/migrations_test.go`

**Interfaces:**
- Produces: `model.BusinessCardComment`、`model.BusinessCardLike`、`BusinessCard.LikeCount int`

- [ ] **Step 1: 写失败测试**

在 `migrations_test.go` 的表清单中加入 `business_card_comment`、`business_card_like`，并新增：

```go
func TestBusinessCardSocialMigration(t *testing.T) {
	body, err := fs.ReadFile(FS, "00034_business_card_social.sql")
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, want := range []string{
		"CREATE TABLE `business_card_comment`",
		"CREATE TABLE `business_card_like`",
		"ADD COLUMN `like_count`",
		"uk_business_card_comment_public",
		"idx_business_card_comment_roots",
		"idx_business_card_comment_replies",
		"uk_business_card_like_card_user",
		"-- +goose Up",
		"-- +goose Down",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("social migration missing %q", want)
		}
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./migrations -run TestBusinessCardSocialMigration -v`

Expected: FAIL，提示 `open 00034_business_card_social.sql: file does not exist`。

- [ ] **Step 3: 写迁移**

```sql
-- +goose Up

CREATE TABLE `business_card_comment` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '评论公开ID',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '评论用户ID',
  `parent_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '父评论ID，0为根评论',
  `root_id` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '根评论ID，0表示自身为根',
  `depth` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '展示深度，最大3',
  `content` varchar(500) NOT NULL COMMENT '纯文本评论内容',
  `status` tinyint UNSIGNED NOT NULL DEFAULT 1 COMMENT '1正常，2已删除占位',
  `edited_at` datetime DEFAULT NULL COMMENT '编辑时间',
  `reply_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '根评论回复总数',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_comment_public` (`public_id`),
  KEY `idx_business_card_comment_roots` (`card_id`, `parent_id`, `id`),
  KEY `idx_business_card_comment_replies` (`root_id`, `id`),
  KEY `idx_business_card_comment_user` (`user_id`, `id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片评论';

CREATE TABLE `business_card_like` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `card_id` bigint UNSIGNED NOT NULL COMMENT '名片ID',
  `user_id` bigint UNSIGNED NOT NULL COMMENT '用户ID',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_business_card_like_card_user` (`card_id`, `user_id`),
  KEY `idx_business_card_like_user` (`user_id`, `created_at`),
  CONSTRAINT `fk_business_card_like_card` FOREIGN KEY (`card_id`) REFERENCES `business_card` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='名片点赞';

ALTER TABLE `business_card`
  ADD COLUMN `like_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数' AFTER `comment_count`;

-- +goose Down

ALTER TABLE `business_card` DROP COLUMN `like_count`;

DROP TABLE IF EXISTS `business_card_like`;
DROP TABLE IF EXISTS `business_card_comment`;
```

- [ ] **Step 4: 写 DAO model**

`business_card_comment.go`：

```go
package model

import "time"

type BusinessCardComment struct {
	Base
	PublicID   string     `gorm:"column:public_id" json:"publicId"`
	CardID     int64      `gorm:"column:card_id" json:"cardId"`
	UserID     int64      `gorm:"column:user_id" json:"userId"`
	ParentID   int64      `gorm:"column:parent_id" json:"parentId"`
	RootID     int64      `gorm:"column:root_id" json:"rootId"`
	Depth      uint8      `gorm:"column:depth" json:"depth"`
	Content    string     `gorm:"column:content" json:"content"`
	Status     int8       `gorm:"column:status" json:"status"`
	EditedAt   *time.Time `gorm:"column:edited_at" json:"editedAt,omitempty"`
	ReplyCount int        `gorm:"column:reply_count" json:"replyCount"`
}

func (BusinessCardComment) TableName() string { return "business_card_comment" }
```

`business_card_like.go`：

```go
package model

type BusinessCardLike struct {
	Base
	CardID int64 `gorm:"column:card_id" json:"cardId"`
	UserID int64 `gorm:"column:user_id" json:"userId"`
}

func (BusinessCardLike) TableName() string { return "business_card_like" }
```

`business_card.go` 的 `BusinessCard` 增加：

```go
	LikeCount int `gorm:"column:like_count" json:"likeCount"`
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/backend && go test ./migrations ./internal/dao/model -v`

Expected: PASS。

- [ ] **Step 6: 提交（用户要求时）**

```bash
git add packages/backend/migrations/00034_business_card_social.sql packages/backend/migrations/migrations_test.go packages/backend/internal/dao/model/business_card.go packages/backend/internal/dao/model/business_card_comment.go packages/backend/internal/dao/model/business_card_like.go
git commit -m "feat(card): add comment and like schema"
```

---

### Task 2: 评论 Repository

**Files:**
- Create: `packages/backend/internal/businesscard/comment_repository.go`
- Create: `packages/backend/internal/businesscard/comment_repository_test.go`

**Interfaces:**
- Produces:
  - `CommentRecord`
  - `CommentInput`
  - `CommentRepository` 接口
  - `MySQLRepository.ListRoots/ListReplies/CreateComment/UpdateComment/DeleteComment/GetCommentByPublicID`

- [ ] **Step 1: 写失败测试**

```go
func TestListRootsUsesCursorAndReturnsOneExtra(t *testing.T) {
	db, mock := newMockDB(t)
	repo := NewMySQLRepository(db)
	rows := sqlmock.NewRows([]string{
		"id", "public_id", "card_id", "user_id", "parent_id", "root_id", "depth",
		"content", "status", "edited_at", "reply_count", "created_at",
		"author_name", "author_avatar_s3_path", "reply_to_name",
	}).
		AddRow(12, "c-12", 1, 7, 0, 0, 1, "hello", 1, nil, 3, time.Now(), "alice", "", "").
		AddRow(11, "c-11", 1, 8, 0, 0, 1, "world", 1, nil, 0, time.Now(), "bob", "", "")
	mock.ExpectQuery("SELECT c\\.\\*, bc\\.public_id AS card_public_id").
		WithArgs(int64(1), int64(10)).
		WillReturnRows(rows)

	items, hasMore, err := repo.ListRoots(context.Background(), 1, 10, 1)
	if err != nil || !hasMore || len(items) != 1 || items[0].PublicID != "c-12" {
		t.Fatalf("items=%+v hasMore=%v err=%v", items, hasMore, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
```

再补一个删除测试：无子评论走 `DELETE`，有子评论走 `UPDATE status=2`。

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/businesscard -run TestListRootsUsesCursor -v`

Expected: FAIL，编译错误 `repo.ListRoots undefined`。

- [ ] **Step 3: 写 Repository**

```go
type CommentRecord struct {
	ID                 int64
	PublicID           string
	CardID             int64
	CardPublicID       string
	UserID             int64
	ParentID           int64
	ParentPublicID     string
	RootID             int64
	RootPublicID       string
	Depth              int
	Content            string
	Status             int8
	EditedAt           *time.Time
	ReplyCount         int
	CreatedAt          time.Time
	AuthorName         string
	AuthorAvatarS3Path string
	ReplyToName        string
}

type CommentInput struct {
	PublicID string
	CardID   int64
	UserID   int64
	ParentID int64
	RootID   int64
	Depth    int
	Content  string
}

type CommentDeleteResult struct {
	Tombstoned   bool `json:"tombstoned"`
	CommentCount int  `json:"commentCount"`
}

type CommentRepository interface {
	ListRoots(ctx context.Context, cardID, cursor int64, limit int) ([]*CommentRecord, bool, error)
	ListReplies(ctx context.Context, rootID, cursor int64, limit int) ([]*CommentRecord, bool, error)
	GetCommentByPublicID(ctx context.Context, publicID string) (*CommentRecord, error)
	CreateComment(ctx context.Context, input CommentInput) (*CommentRecord, error)
	UpdateComment(ctx context.Context, cardID, commentID, userID int64, content string, at time.Time) (*CommentRecord, error)
	DeleteComment(ctx context.Context, cardID, commentID, userID int64) (CommentDeleteResult, error)
}
```

查询行与映射：

```go
type commentRow struct {
	ID                 int64      `gorm:"column:id"`
	PublicID           string     `gorm:"column:public_id"`
	CardID             int64      `gorm:"column:card_id"`
	UserID             int64      `gorm:"column:user_id"`
	ParentID           int64      `gorm:"column:parent_id"`
	RootID             int64      `gorm:"column:root_id"`
	ParentPublicID     string     `gorm:"column:parent_public_id"`
	RootPublicID       string     `gorm:"column:root_public_id"`
	CardPublicID       string     `gorm:"column:card_public_id"`
	Depth              int        `gorm:"column:depth"`
	Content            string     `gorm:"column:content"`
	Status             int8       `gorm:"column:status"`
	EditedAt           *time.Time `gorm:"column:edited_at"`
	ReplyCount         int        `gorm:"column:reply_count"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	AuthorName         string     `gorm:"column:author_name"`
	AuthorNickname     string     `gorm:"column:author_nickname"`
	AuthorAvatarS3Path string     `gorm:"column:author_avatar_s3_path"`
	ReplyToName        string     `gorm:"column:reply_to_name"`
	ReplyToNickname    string     `gorm:"column:reply_to_nickname"`
}

func (r commentRow) record() *CommentRecord {
	name := r.AuthorNickname
	if name == "" {
		name = r.AuthorName
	}
	replyName := r.ReplyToNickname
	if replyName == "" {
		replyName = r.ReplyToName
	}
	return &CommentRecord{
		ID: r.ID, PublicID: r.PublicID, CardID: r.CardID, UserID: r.UserID,
		ParentID: r.ParentID, RootID: r.RootID, Depth: r.Depth, Content: r.Content,
		ParentPublicID: r.ParentPublicID, RootPublicID: r.RootPublicID, CardPublicID: r.CardPublicID,
		Status: r.Status, EditedAt: r.EditedAt, ReplyCount: r.ReplyCount, CreatedAt: r.CreatedAt,
		AuthorName: name, AuthorAvatarS3Path: r.AuthorAvatarS3Path, ReplyToName: replyName,
	}
}
```

查询常量：

```go
const commentSelect = `c.*, bc.public_id AS card_public_id, pc.public_id AS parent_public_id,
	rc.public_id AS root_public_id, u.username AS author_name, u.nickname AS author_nickname,
	u.avatar_s3_path AS author_avatar_s3_path, pu.username AS reply_to_name, pu.nickname AS reply_to_nickname`

func (r *MySQLRepository) commentBase(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("business_card_comment AS c").
		Select(commentSelect).
		Joins("LEFT JOIN business_card AS bc ON bc.id = c.card_id").
		Joins("LEFT JOIN user AS u ON u.id = c.user_id").
		Joins("LEFT JOIN business_card_comment AS pc ON pc.id = c.parent_id").
		Joins("LEFT JOIN user AS pu ON pu.id = pc.user_id").
		Joins("LEFT JOIN business_card_comment AS rc ON rc.id = c.root_id").
		Where("c.deleted_at IS NULL")
}
```

`ListRoots` 关键逻辑：

```go
func (r *MySQLRepository) ListRoots(ctx context.Context, cardID, cursor int64, limit int) ([]*CommentRecord, bool, error) {
	if limit <= 0 {
		limit = 20
	}
	query := r.commentBase(ctx).
		Where("c.card_id = ? AND c.parent_id = 0", cardID).
		Where("(c.status = 1 OR c.reply_count > 0)")
	if cursor > 0 {
		query = query.Where("c.id < ?", cursor)
	}
	var rows []commentRow
	if err := query.Order("c.id DESC").Limit(limit + 1).Find(&rows).Error; err != nil {
		return nil, false, err
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	out := make([]*CommentRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.record())
	}
	return out, hasMore, nil
}
```

`ListReplies` 使用 `c.root_id = ? AND c.id > ?`、`(c.status = 1 OR EXISTS (...))`、`ORDER BY c.id ASC`。

`CreateComment` 在事务中插入评论、`business_card.comment_count + 1`；若 `RootID > 0` 再更新根评论 `reply_count + 1`。

`DeleteComment` 在事务中 `FOR UPDATE` 查评论并校验 `UserID`；无子评论硬删除并递减 `comment_count/reply_count`，有子评论更新 `status=2, content=''` 并只递减 `comment_count`。事务结束后读取 `business_card.comment_count`，返回 `CommentDeleteResult{Tombstoned: children > 0, CommentCount: count}`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/businesscard -run 'TestListRoots|TestDeleteComment' -v`

Expected: PASS。

- [ ] **Step 5: 提交（用户要求时）**

```bash
git add packages/backend/internal/businesscard/comment_repository.go packages/backend/internal/businesscard/comment_repository_test.go
git commit -m "feat(card): add comment repository"
```

---

### Task 3: 评论 Service 与频率限制

**Files:**
- Create: `packages/backend/internal/businesscard/comment_service.go`
- Create: `packages/backend/internal/businesscard/comment_service_test.go`
- Create: `packages/backend/internal/businesscard/comment_rate_limit.go`

**Interfaces:**
- Produces:
  - `CommentService.ListRoots/ListReplies/Create/Update/Delete`
  - `Comment`、`CommentPage`、`CommentAuthor`
  - `CommentRateLimiter`

- [ ] **Step 1: 写失败测试**

```go
func TestCreateCommentCapsDepthAtThree(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.byPublicID["parent"] = &CommentRecord{
		ID: 9, PublicID: "parent", CardID: 1, UserID: 8, RootID: 0, Depth: 3, Status: 1,
	}
	svc := newTestCommentService(repo)
	comment, err := svc.Create(context.Background(), "card-1", 7, "parent", "回复")
	if err != nil {
		t.Fatal(err)
	}
	if comment.Depth != 3 || repo.lastInput.ParentID != 9 {
		t.Fatalf("depth=%d parent=%d", comment.Depth, repo.lastInput.ParentID)
	}
}

func TestUpdateRejectsOtherAuthor(t *testing.T) {
	repo := newFakeCommentRepo()
	repo.updateAllowed = false
	svc := newTestCommentService(repo)
	_, err := svc.Update(context.Background(), "card-1", "c-1", 7, "改")
	if !errors.Is(err, ErrCommentForbidden) {
		t.Fatalf("err = %v", err)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/businesscard -run TestCreateCommentCapsDepth -v`

Expected: FAIL，`CommentService` 未定义。

- [ ] **Step 3: 写 Service**

```go
var (
	ErrCommentNotFound   = errors.New("comment not found")
	ErrCommentForbidden  = errors.New("comment forbidden")
	ErrCommentInvalid    = errors.New("invalid comment")
	ErrCommentRateLimit  = errors.New("comment rate limited")
)

type CommentAuthor struct {
	UserID    int64  `json:"userId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
}

type Comment struct {
	ID          string        `json:"id"`
	CardID      string        `json:"cardId"`
	ParentID    string        `json:"parentId,omitempty"`
	RootID      string        `json:"rootId,omitempty"`
	Depth       int           `json:"depth"`
	Content     string        `json:"content"`
	Author      CommentAuthor `json:"author"`
	ReplyToName string        `json:"replyToName,omitempty"`
	ReplyCount  int           `json:"replyCount"`
	Status      string        `json:"status"`
	Editable    bool          `json:"editable"`
	Deletable   bool          `json:"deletable"`
	EditedAt    *time.Time    `json:"editedAt,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	Replies     []Comment     `json:"replies,omitempty"`
}

type CommentPage struct {
	Comments   []Comment `json:"comments"`
	NextCursor string    `json:"nextCursor"`
	HasMore    bool      `json:"hasMore"`
}

type CommentService struct {
	Repo     CommentRepository
	Cards    interface{ GetByPublicID(context.Context, string) (*CardRecord, error) }
	Limiter  CommentRateLimiter
	Now      func() time.Time
	NewID    func() string
	MaxDepth int
}

```

核心规则：

```go
func (s *CommentService) Create(ctx context.Context, cardPublicID string, userID int64, parentPublicID, content string) (*Comment, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return nil, err
	}
	content, err = normalizeCommentContent(content)
	if err != nil {
		return nil, err
	}
	if s.Limiter != nil && !s.Limiter.Allow(ctx, userID, card.ID) {
		return nil, ErrCommentRateLimit
	}
	parentID, rootID, depth := int64(0), int64(0), 1
	if parentPublicID != "" {
		parent, err := s.Repo.GetCommentByPublicID(ctx, parentPublicID)
		if err != nil || parent.CardID != card.ID || parent.Status != 1 {
			return nil, ErrCommentNotFound
		}
		parentID = parent.ID
		rootID = parent.ID
		if parent.RootID > 0 {
			rootID = parent.RootID
		}
		depth = parent.Depth + 1
		if depth > s.MaxDepth {
			depth = s.MaxDepth
		}
	}
	rec, err := s.Repo.CreateComment(ctx, CommentInput{
		PublicID: s.NewID(), CardID: card.ID, UserID: userID,
		ParentID: parentID, RootID: rootID, Depth: depth, Content: content,
	})
	if err != nil {
		return nil, err
	}
	return s.toComment(rec, userID), nil
}
```

`ListRoots` 为每个根评论补齐前 2 条回复预览；`ListReplies` 校验根评论属于该名片；`Update` 在调用 Repo 前先 `normalizeCommentContent`；`Delete` 映射 `ErrCommentForbidden`，并把 Repo 的 `CommentDeleteResult` 转成 HTTP DTO。

```go
func (s *CommentService) Delete(ctx context.Context, cardPublicID, commentPublicID string, userID int64) (CommentDeleteResult, error) {
	card, err := s.publishedCard(ctx, cardPublicID)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	rec, err := s.Repo.GetCommentByPublicID(ctx, commentPublicID)
	if err != nil || rec.CardID != card.ID || rec.Status != 1 {
		return CommentDeleteResult{}, ErrCommentNotFound
	}
	if rec.UserID != userID {
		return CommentDeleteResult{}, ErrCommentForbidden
	}
	result, err := s.Repo.DeleteComment(ctx, card.ID, rec.ID, userID)
	if err != nil {
		return CommentDeleteResult{}, err
	}
	return CommentDeleteResult{Tombstoned: result.Tombstoned, CommentCount: result.CommentCount}, nil
}

func (s *CommentService) publishedCard(ctx context.Context, publicID string) (*CardRecord, error) {
	card, err := s.Cards.GetByPublicID(ctx, publicID)
	if err != nil || !card.Published || card.Hidden {
		return nil, ErrCommentNotFound
	}
	return card, nil
}

func normalizeCommentContent(raw string) (string, error) {
	content := strings.TrimSpace(raw)
	if content == "" || utf8.RuneCountInString(content) > 500 {
		return "", ErrCommentInvalid
	}
	for _, r := range content {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return "", ErrCommentInvalid
		}
	}
	return content, nil
}

func (s *CommentService) toComment(rec *CommentRecord, viewerID int64) *Comment {
	status := "normal"
	if rec.Status == 2 {
		status = "deleted"
	}
	return &Comment{
		ID: rec.PublicID, CardID: rec.CardPublicID, ParentID: rec.ParentPublicID,
		RootID: rec.RootPublicID, Depth: rec.Depth, Content: rec.Content,
		Author: CommentAuthor{UserID: rec.UserID, Name: rec.AuthorName, AvatarURL: rec.AuthorAvatarS3Path},
		ReplyToName: rec.ReplyToName, ReplyCount: rec.ReplyCount, Status: status,
		Editable: rec.Status == 1 && rec.UserID == viewerID,
		Deletable: rec.Status == 1 && rec.UserID == viewerID,
		EditedAt: rec.EditedAt, CreatedAt: rec.CreatedAt,
	}
}
```

`commentSelect` 同时带出卡片、父评论、根评论的公开 ID，`toComment` 不再需要额外查询。

`comment_rate_limit.go`：

```go
type CommentRateLimiter interface {
	Allow(ctx context.Context, userID, cardID int64) bool
}

type RedisCommentRateLimiter struct {
	Client *redis.Client
	Prefix string
	Now    func() time.Time
}

func (l *RedisCommentRateLimiter) Allow(ctx context.Context, userID, cardID int64) bool {
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	bucket := now().Unix() / 10
	shortKey := fmt.Sprintf("%s:comment:rate:short:%d:%d:%d", l.Prefix, userID, cardID, bucket)
	hourKey := fmt.Sprintf("%s:comment:rate:hour:%d", l.Prefix, userID)
	pipe := l.Client.TxPipeline()
	short := pipe.Incr(ctx, shortKey)
	pipe.Expire(ctx, shortKey, 15*time.Second)
	hour := pipe.Incr(ctx, hourKey)
	pipe.Expire(ctx, hourKey, time.Hour)
	_, err := pipe.Exec(ctx)
	return err == nil && short.Val() <= 5 && hour.Val() <= 30
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/businesscard -run 'TestCreateComment|TestUpdateRejects' -v`

Expected: PASS。

- [ ] **Step 5: 提交（用户要求时）**

```bash
git add packages/backend/internal/businesscard/comment_service.go packages/backend/internal/businesscard/comment_service_test.go packages/backend/internal/businesscard/comment_rate_limit.go
git commit -m "feat(card): add comment service and rate limit"
```

---

### Task 4: 评论 HTTP Handler 与路由

**Files:**
- Create: `packages/backend/internal/handler/card_comment.go`
- Create: `packages/backend/internal/handler/card_comment_test.go`
- Modify: `packages/backend/internal/router/control.go`

**Interfaces:**
- Consumes: `businesscard.CommentService`
- Produces: `handler.CardCommentHandler`、评论 API 路由

- [ ] **Step 1: 写失败测试**

```go
func TestCardCommentCreateRequiresLogin(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCommentService{})
	e.POST("/cp/api/v1/public/cards/:id/comments", h.Create)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/cp/api/v1/public/cards/card-1/comments", strings.NewReader(`{"content":"hi"}`)))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCardCommentListReturnsPage(t *testing.T) {
	e := echo.New()
	h := NewCardCommentHandler(&fakeCommentService{
		listRoots: func(context.Context, string, int64, string, int) (*businesscard.CommentPage, error) {
			return &businesscard.CommentPage{Comments: []businesscard.Comment{{ID: "c-1"}}}, nil
		},
	})
	e.GET("/cp/api/v1/public/cards/:id/comments", h.ListRoots)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1/comments", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"id":"c-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/handler -run TestCardComment -v`

Expected: FAIL，`NewCardCommentHandler` 未定义。

- [ ] **Step 3: 写 Handler**

```go
type CardCommentService interface {
	ListRoots(ctx context.Context, cardPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error)
	ListReplies(ctx context.Context, cardPublicID, rootPublicID string, userID int64, cursor string, limit int) (*businesscard.CommentPage, error)
	Create(ctx context.Context, cardPublicID string, userID int64, parentPublicID, content string) (*businesscard.Comment, error)
	Update(ctx context.Context, cardPublicID, commentPublicID string, userID int64, content string) (*businesscard.Comment, error)
	Delete(ctx context.Context, cardPublicID, commentPublicID string, userID int64) (businesscard.CommentDeleteResult, error)
}

type CardCommentHandler struct {
	Service CardCommentService
}
```

`Create` 读取 `auth.SessionFrom(c)`，未登录返回 401；绑定 `{content,parentId}`；调用 Service；错误用 `commentError` 映射 400/403/404/429。`Delete` 返回 `{tombstoned, commentCount}`，供前端决定是移除节点还是替换为「已删除」占位。

```go
func commentError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, businesscard.ErrCommentInvalid):
		return response.Fail(c, http.StatusBadRequest, 40001, "invalid comment")
	case errors.Is(err, businesscard.ErrCommentForbidden):
		return response.Fail(c, http.StatusForbidden, 40301, "comment forbidden")
	case errors.Is(err, businesscard.ErrCommentNotFound):
		return response.Fail(c, http.StatusNotFound, 40410, "comment not found")
	case errors.Is(err, businesscard.ErrCommentRateLimit):
		return response.Fail(c, http.StatusTooManyRequests, 42901, "too many comments")
	default:
		return response.Fail(c, http.StatusInternalServerError, 50016, "comment request failed")
	}
}
```

`ListRoots` / `ListReplies` 使用可选会话：

```go
func commentUserID(c echo.Context) int64 {
	if session, ok := auth.SessionFrom(c); ok {
		return session.UserID
	}
	return 0
}
```

`router/control.go`：

```go
if cardCommentHandler != nil {
	api.GET("/public/cards/:id/comments", cardCommentHandler.ListRoots, authHandler.AttachUserSession)
	api.GET("/public/cards/:id/comments/:commentId/replies", cardCommentHandler.ListReplies, authHandler.AttachUserSession)
	commentWrite := api.Group("/public/cards/:id/comments", authn.RequireUser)
	commentWrite.POST("", cardCommentHandler.Create)
	commentWrite.PUT("/:commentId", cardCommentHandler.Update)
	commentWrite.DELETE("/:commentId", cardCommentHandler.Delete)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/handler -run TestCardComment -v && go test ./internal/router -v`

Expected: PASS。

- [ ] **Step 5: 提交（用户要求时）**

```bash
git add packages/backend/internal/handler/card_comment.go packages/backend/internal/handler/card_comment_test.go packages/backend/internal/router/control.go
git commit -m "feat(card): add comment endpoints"
```

---

### Task 5: 点赞 Redis Store 与 Lua

**Files:**
- Create: `packages/backend/internal/businesscard/like_store.go`
- Create: `packages/backend/internal/businesscard/like_store_test.go`

**Interfaces:**
- Produces:
  - `LikeState{Cound int64; Liked bool; Found bool}`
  - `LikeStore.SetLiked/States/Counts/SeedCount`
  - `RedisLikeStore`

- [ ] **Step 1: 写失败测试**

```go
func TestRedisLikeStoreToggleIsIdempotent(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisLikeStore(client, "vibebiz", 1_000_000, 1)

	state, err := store.SetLiked(context.Background(), 10, 7, true)
	if err != nil || !state.Liked || state.Count != 1 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
	state, err = store.SetLiked(context.Background(), 10, 7, true)
	if err != nil || !state.Liked || state.Count != 1 {
		t.Fatalf("repeat state=%+v err=%v", state, err)
	}
	state, err = store.SetLiked(context.Background(), 10, 7, false)
	if err != nil || state.Liked || state.Count != 0 {
		t.Fatalf("unlike state=%+v err=%v", state, err)
	}
	if got := mr.Stream("vibebiz:card:like:stream"); len(got) != 2 {
		t.Fatalf("stream len = %d, want 2", len(got))
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/businesscard -run TestRedisLikeStore -v`

Expected: FAIL，`NewRedisLikeStore` 未定义。

- [ ] **Step 3: 写 Store**

```go
type LikeState struct {
	Count int64
	Liked bool
	Found bool
}

type LikeStore interface {
	SetLiked(ctx context.Context, cardID, userID int64, liked bool) (LikeState, error)
	States(ctx context.Context, cardIDs []int64, userID int64) (map[int64]LikeState, error)
	Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
	SeedCount(ctx context.Context, cardID, count int64) error
}

type RedisLikeStore struct {
	client *redis.Client
	prefix string
	maxLen int64
	shards int
	script *redis.Script
}

func (s *RedisLikeStore) countKey() string {
	return s.prefix + ":card:like:count"
}

func (s *RedisLikeStore) usersKey(cardID int64) string {
	return fmt.Sprintf("%s:card:like:users:%d", s.prefix, cardID)
}

func (s *RedisLikeStore) streamKey(cardID int64) string {
	if s.shards <= 1 {
		return s.prefix + ":card:like:stream"
	}
	return fmt.Sprintf("%s:card:like:stream:%d", s.prefix, cardID%int64(s.shards))
}
```

Lua：

```lua
local liked = redis.call('SISMEMBER', KEYS[2], ARGV[1])
local want = ARGV[2] == '1'
if want and liked == 0 then
  redis.call('SADD', KEYS[2], ARGV[1])
  redis.call('HINCRBY', KEYS[1], ARGV[3], 1)
  redis.call('XADD', KEYS[3], '*', 'eventId', ARGV[5], 'cardId', ARGV[3], 'userId', ARGV[1], 'action', 'like', 'ts', ARGV[4])
elseif (not want) and liked == 1 then
  redis.call('SREM', KEYS[2], ARGV[1])
  local count = redis.call('HINCRBY', KEYS[1], ARGV[3], -1)
  if count < 0 then
    redis.call('HSET', KEYS[1], ARGV[3], 0)
  end
  redis.call('XADD', KEYS[3], '*', 'eventId', ARGV[5], 'cardId', ARGV[3], 'userId', ARGV[1], 'action', 'unlike', 'ts', ARGV[4])
end
return {redis.call('SISMEMBER', KEYS[2], ARGV[1]), redis.call('HGET', KEYS[1], ARGV[3]) or '0'}
```

`States` 使用 pipeline 对每个 cardID 执行 `HGet` + `SIsMember`；`Counts` 使用 `HMGet`。

```go
func (s *RedisLikeStore) SetLiked(ctx context.Context, cardID, userID int64, liked bool) (LikeState, error) {
	action := "0"
	if liked {
		action = "1"
	}
	eventID := uuid.NewString()
	now := time.Now().UnixMilli()
	values, err := s.script.Run(ctx, s.client,
		[]string{s.countKey(), s.usersKey(cardID), s.streamKey(cardID)},
		userID, action, cardID, now, eventID,
	).Slice()
	if err != nil {
		return LikeState{}, err
	}
	return LikeState{Liked: values[0].(int64) == 1, Count: values[1].(int64), Found: true}, nil
}

func (s *RedisLikeStore) States(ctx context.Context, cardIDs []int64, userID int64) (map[int64]LikeState, error) {
	out := make(map[int64]LikeState, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	pipe := s.client.Pipeline()
	counts := make(map[int64]*redis.StringCmd, len(cardIDs))
	liked := make(map[int64]*redis.BoolCmd, len(cardIDs))
	for _, id := range cardIDs {
		counts[id] = pipe.HGet(ctx, s.countKey(), strconv.FormatInt(id, 10))
		liked[id] = pipe.SIsMember(ctx, s.usersKey(id), userID)
	}
	_, err := pipe.Exec(ctx)
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}
	for _, id := range cardIDs {
		count, countErr := counts[id].Int64()
		found := countErr == nil
		if errors.Is(countErr, redis.Nil) {
			count = 0
		}
		out[id] = LikeState{Count: count, Liked: liked[id].Val(), Found: found}
	}
	return out, nil
}

func (s *RedisLikeStore) Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	fields := make([]string, 0, len(cardIDs))
	for _, id := range cardIDs {
		fields = append(fields, strconv.FormatInt(id, 10))
	}
	values, err := s.client.HMGet(ctx, s.countKey(), fields...).Result()
	if err != nil {
		return nil, err
	}
	for i, id := range cardIDs {
		switch value := values[i].(type) {
		case int64:
			out[id] = value
		case string:
			out[id], _ = strconv.ParseInt(value, 10, 64)
		}
	}
	return out, nil
}

func (s *RedisLikeStore) SeedCount(ctx context.Context, cardID, count int64) error {
	return s.client.HSetNX(ctx, s.countKey(), strconv.FormatInt(cardID, 10), count).Err()
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/businesscard -run TestRedisLikeStore -v`

Expected: PASS。

- [ ] **Step 5: 提交（用户要求时）**

```bash
git add packages/backend/internal/businesscard/like_store.go packages/backend/internal/businesscard/like_store_test.go
git commit -m "feat(card): add redis like store"
```

---

### Task 6: 点赞 Service、Handler 与卡片状态注入

**Files:**
- Create: `packages/backend/internal/businesscard/like_service.go`
- Create: `packages/backend/internal/businesscard/like_service_test.go`
- Create: `packages/backend/internal/businesscard/like_repository.go`
- Create: `packages/backend/internal/handler/card_like.go`
- Create: `packages/backend/internal/handler/card_like_test.go`
- Modify: `packages/backend/internal/businesscard/service.go`
- Modify: `packages/backend/internal/handler/card.go`
- Modify: `packages/backend/internal/router/control.go`

**Interfaces:**
- Consumes: `LikeStore`、`CardRecord`
- Produces:
  - `LikeService.Set/States`
  - `Card.LikeCount int64`、`Card.Liked bool`
  - `PUT/DELETE /public/cards/:id/like`

- [ ] **Step 1: 写失败测试**

```go
func TestLikeServiceSetResolvesPublishedCard(t *testing.T) {
	store := &fakeLikeStore{state: LikeState{Liked: true, Count: 1}}
	svc := &LikeService{
		Store: store,
		Cards: fakeCardReader{card: &CardRecord{ID: 10, PublicID: "card-1", Published: true}},
	}
	state, err := svc.Set(context.Background(), "card-1", 7, true)
	if err != nil || !state.Liked || state.Count != 1 {
		t.Fatalf("state=%+v err=%v", state, err)
	}
}

func TestPublicCardIncludesLikeState(t *testing.T) {
	e, fake := newCardEcho(t)
	fake.likes = &fakeLikeEnricher{state: businesscard.LikeState{Liked: true, Count: 3}}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/cp/api/v1/public/cards/card-1", nil))
	if !strings.Contains(rec.Body.String(), `"likeCount":3`) || !strings.Contains(rec.Body.String(), `"liked":true`) {
		t.Fatalf("body = %s", rec.Body.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/businesscard ./internal/handler -run 'TestLikeService|TestPublicCardIncludesLike' -v`

Expected: FAIL，`LikeService` 未定义。

- [ ] **Step 3: 写 LikeService**

```go
type LikeReadRepository interface {
	Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
	Liked(ctx context.Context, cardID, userID int64) (bool, error)
}

type MySQLLikeRepository struct {
	db *gorm.DB
}

func NewMySQLLikeRepository(db *gorm.DB) *MySQLLikeRepository {
	return &MySQLLikeRepository{db: db}
}

type LikeService struct {
	Store LikeStore
	Cards interface{ GetByPublicID(context.Context, string) (*CardRecord, error) }
	Repo  LikeReadRepository
}

func (s *LikeService) Set(ctx context.Context, cardPublicID string, userID int64, liked bool) (LikeState, error) {
	card, err := s.Cards.GetByPublicID(ctx, cardPublicID)
	if err != nil || !card.Published || card.Hidden {
		return LikeState{}, ErrNotFound
	}
	return s.Store.SetLiked(ctx, card.ID, userID, liked)
}

func (s *LikeService) States(ctx context.Context, cards []Card, userID int64) error {
	ids := make([]int64, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.InternalID)
	}
	states, err := s.Store.States(ctx, ids, userID)
	if err != nil && s.Repo == nil {
		return err
	}
	if err != nil {
		states = map[int64]LikeState{}
	}
	missing := make([]int64, 0)
	for _, id := range ids {
		if state, ok := states[id]; !ok || !state.Found {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 && s.Repo != nil {
		counts, countErr := s.Repo.Counts(ctx, missing)
		if countErr != nil {
			return countErr
		}
		for _, id := range missing {
			liked, likedErr := s.Repo.Liked(ctx, id, userID)
			if likedErr != nil {
				return likedErr
			}
			states[id] = LikeState{Count: counts[id], Liked: liked, Found: true}
			_ = s.Store.SeedCount(ctx, id, counts[id])
		}
	}
	for i := range cards {
		state := states[cards[i].InternalID]
		cards[i].LikeCount = state.Count
		cards[i].Liked = state.Liked
	}
	return nil
}

func (r *MySQLLikeRepository) Counts(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	if len(cardIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ID        int64 `gorm:"column:id"`
		LikeCount int64 `gorm:"column:like_count"`
	}
	err := r.db.WithContext(ctx).Table("business_card").
		Select("id, like_count").
		Where("id IN ?", cardIDs).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.ID] = row.LikeCount
	}
	return out, nil
}

func (r *MySQLLikeRepository) Liked(ctx context.Context, cardID, userID int64) (bool, error) {
	if userID <= 0 {
		return false, nil
	}
	var count int64
	err := r.db.WithContext(ctx).Model(&model.BusinessCardLike{}).
		Where("card_id = ? AND user_id = ? AND deleted_at IS NULL", cardID, userID).
		Count(&count).Error
	return count > 0, err
}
```

`Card` 增加内部字段 `InternalID int64 `json:"-"``、`LikeCount int64`、`Liked bool`。`Service.resolve` 填充 `InternalID: rec.ID`。

`CardHandler` 增加可选字段：

```go
type CardLikeEnricher interface {
	States(ctx context.Context, cards []businesscard.Card, userID int64) error
}
```

`GetMine`、`Discover`、`GetPublic` 拿到结果后调用：

```go
func (h *CardHandler) enrichLikes(c echo.Context, cards []businesscard.Card) {
	if h.Likes == nil || len(cards) == 0 {
		return
	}
	userID := int64(0)
	if session, ok := auth.SessionFrom(c); ok {
		userID = session.UserID
	}
	_ = h.Likes.States(c.Request().Context(), cards, userID)
}
```

单卡片接口使用：

```go
cards := []businesscard.Card{*card}
h.enrichLikes(c, cards)
card = &cards[0]
```

列表接口直接 `h.enrichLikes(c, result.Cards)`。

`PublicCard` 使用 `AttachUserSession` 以支持 `liked`。

`handler/card_like.go` 提供 `Set` 处理 `PUT`（liked=true）与 `Delete` 处理 `DELETE`（liked=false）：

```go
func (h *CardLikeHandler) Set(c echo.Context) error { return h.apply(c, true) }
func (h *CardLikeHandler) Unset(c echo.Context) error { return h.apply(c, false) }
```

- [ ] **Step 4: 路由**

```go
if cardLikeHandler != nil {
	api.PUT("/public/cards/:id/like", cardLikeHandler.Set, authn.RequireUser)
	api.DELETE("/public/cards/:id/like", cardLikeHandler.Unset, authn.RequireUser)
}
```

`PublicCard` 路由增加 `authHandler.AttachUserSession`。

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/businesscard ./internal/handler -run 'TestLikeService|TestPublicCardIncludesLike|TestCardLike' -v`

Expected: PASS。

- [ ] **Step 6: 提交（用户要求时）**

```bash
git add packages/backend/internal/businesscard/like_service.go packages/backend/internal/businesscard/like_service_test.go packages/backend/internal/businesscard/service.go packages/backend/internal/handler/card_like.go packages/backend/internal/handler/card_like_test.go packages/backend/internal/handler/card.go packages/backend/internal/router/control.go
git commit -m "feat(card): add like service and card like state"
```

---

### Task 7: 点赞 MySQL 写库与 Stream 消费任务

**Files:**
- Modify: `packages/backend/internal/businesscard/like_repository.go`
- Create: `packages/backend/internal/businesscard/like_sync.go`
- Create: `packages/backend/internal/businesscard/like_sync_test.go`
- Modify: `packages/backend/internal/config/config.go`
- Modify: `packages/backend/internal/config/config_test.go`
- Modify: `packages/backend/cmd/cp/main.go`
- Modify: `packages/backend/.env.example`

**Interfaces:**
- Consumes: Redis Stream `vibebiz:card:like:stream`
- Produces:
  - `LikeEvent`
  - `LikeSyncRepository.Apply/Recount`
  - `LikeSync.Run/HandleTask`
  - asynq task `card:like:sync`

- [ ] **Step 1: 写失败测试**

```go
func TestLikeSyncAppliesBatchAndAcks(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	store := NewRedisLikeStore(client, "vibebiz", 1000, 1)
	if _, err := store.SetLiked(context.Background(), 10, 7, true); err != nil {
		t.Fatal(err)
	}
	repo := &fakeLikeSyncRepo{}
	sync := &LikeSync{
		Client: client, Repo: repo, Prefix: "vibebiz",
		Group: "card-like-sync", Consumer: "test", Batch: 100,
	}
	if err := sync.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(repo.events) != 1 || repo.events[0].CardID != 10 || repo.events[0].UserID != 7 || repo.events[0].Action != "like" {
		t.Fatalf("events = %+v", repo.events)
	}
	if pending, _ := client.XPending(context.Background(), "vibebiz:card:like:stream", "card-like-sync").Result(); pending.Count != 0 {
		t.Fatalf("pending = %d", pending.Count)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/backend && go test ./internal/businesscard -run TestLikeSync -v`

Expected: FAIL，`LikeSync` 未定义。

- [ ] **Step 3: 写 MySQL 写库**

```go
type LikeEvent struct {
	EventID string
	CardID  int64
	UserID  int64
	Action  string
	TS      int64
}

type LikeSyncRepository interface {
	Apply(ctx context.Context, events []LikeEvent) ([]int64, error)
	Recount(ctx context.Context, cardIDs []int64) (map[int64]int64, error)
}
```

`Apply` 在事务内按事件顺序执行：

- `like`：`INSERT INTO business_card_like (card_id,user_id) VALUES (?,?) ON DUPLICATE KEY UPDATE deleted_at=NULL, updated_at=NOW()`。
- `unlike`：`UPDATE business_card_like SET deleted_at=NOW() WHERE card_id=? AND user_id=? AND deleted_at IS NULL`。
- 返回去重后的 `cardID`。

```go
func (r *MySQLLikeRepository) Apply(ctx context.Context, events []LikeEvent) ([]int64, error) {
	affected := make(map[int64]struct{})
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, event := range events {
			switch event.Action {
			case "like":
				if err := tx.Exec(`INSERT INTO business_card_like (card_id, user_id)
					VALUES (?, ?)
					ON DUPLICATE KEY UPDATE deleted_at = NULL, updated_at = NOW()`,
					event.CardID, event.UserID).Error; err != nil {
					return err
				}
			case "unlike":
				if err := tx.Exec(`UPDATE business_card_like
					SET deleted_at = NOW()
					WHERE card_id = ? AND user_id = ? AND deleted_at IS NULL`,
					event.CardID, event.UserID).Error; err != nil {
					return err
				}
			}
			affected[event.CardID] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(affected))
	for id := range affected {
		out = append(out, id)
	}
	return out, nil
}
```

`Recount` 对每张卡片执行：

```sql
UPDATE business_card bc
SET like_count = (
  SELECT COUNT(*) FROM business_card_like l
  WHERE l.card_id = bc.id AND l.deleted_at IS NULL
)
WHERE bc.id = ?;
```

```go
func (r *MySQLLikeRepository) Recount(ctx context.Context, cardIDs []int64) (map[int64]int64, error) {
	out := make(map[int64]int64, len(cardIDs))
	for _, id := range cardIDs {
		var count int64
		err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if err := tx.Exec(`UPDATE business_card bc
				SET like_count = (SELECT COUNT(*) FROM business_card_like l WHERE l.card_id = bc.id AND l.deleted_at IS NULL)
				WHERE bc.id = ?`, id).Error; err != nil {
				return err
			}
			return tx.Model(&model.BusinessCard{}).Where("id = ?", id).Pluck("like_count", &count).Error
		})
		if err != nil {
			return nil, err
		}
		out[id] = count
	}
	return out, nil
}
```

- [ ] **Step 4: 写 Stream 消费**

```go
const TaskTypeLikeSync = "card:like:sync"

type LikeSync struct {
	Client   *redis.Client
	Repo     LikeSyncRepository
	Prefix   string
	Group    string
	Consumer string
	Batch    int
	MaxLen   int64
	TTL      time.Duration
	Shards   int
	Logger   *slog.Logger
}

func (s *LikeSync) streamKeys() []string {
	if s.Shards <= 1 {
		return []string{s.Prefix + ":card:like:stream"}
	}
	keys := make([]string, 0, s.Shards)
	for i := 0; i < s.Shards; i++ {
		keys = append(keys, fmt.Sprintf("%s:card:like:stream:%d", s.Prefix, i))
	}
	return keys
}

func (s *LikeSync) HandleTask(ctx context.Context, _ *asynq.Task) error {
	return s.Run(ctx)
}
```

`Run`：

1. `XGroupCreateMkStream(ctx, streamKey, group, "0")`，忽略 `BUSYGROUP`。
2. 处理 pending：`XAutoClaim` 超过 5 分钟的 pending 消息并复用 `process`。
3. 循环 `XReadGroup`，`Count: batch`、`Block: -1`；为空则退出。
4. 调 `Repo.Apply`，再 `Repo.Recount`，全部成功后 `XAck`；失败则 `return err`。
5. `XTrimMaxLenApprox(ctx, streamKey, maxLen, 0)`，并按 `TTL` 计算 `minID` 执行 `XTrimMinIDApprox`。

```go
func (s *LikeSync) Run(ctx context.Context) error {
	for _, key := range s.streamKeys() {
		if err := s.runStream(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (s *LikeSync) runStream(ctx context.Context, key string) error {
	group := s.Group
	if group == "" {
		group = "card-like-sync"
	}
	consumer := s.Consumer
	if consumer == "" {
		consumer = "cp"
	}
	if err := s.Client.XGroupCreateMkStream(ctx, key, group, "0").Err(); err != nil &&
		!strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	start := "0-0"
	for {
		claimed, next, err := s.Client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream: key, Group: group, Consumer: consumer,
			MinIdle: 5 * time.Minute, Start: start, Count: int64(s.Batch),
		}).Result()
		if err != nil {
			return err
		}
		if len(claimed) > 0 {
			if err := s.process(ctx, key, group, claimed); err != nil {
				return err
			}
		}
		if next == "0-0" {
			break
		}
		start = next
	}
	for {
		result, err := s.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: group, Consumer: consumer,
			Streams: []string{key, ">"}, Count: int64(s.Batch), Block: -1,
		}).Result()
		if errors.Is(err, redis.Nil) || len(result) == 0 || len(result[0].Messages) == 0 {
			break
		}
		if err != nil {
			return err
		}
		if err := s.process(ctx, key, group, result[0].Messages); err != nil {
			return err
		}
	}
	if s.MaxLen > 0 {
		_ = s.Client.XTrimMaxLenApprox(ctx, key, s.MaxLen, 0).Err()
	}
	if s.TTL > 0 {
		minID := strconv.FormatInt(time.Now().Add(-s.TTL).UnixMilli(), 10) + "-0"
		_ = s.Client.XTrimMinIDApprox(ctx, key, minID, 0).Err()
	}
	return nil
}

func (s *LikeSync) process(ctx context.Context, key, group string, messages []redis.XMessage) error {
	events := make([]LikeEvent, 0, len(messages))
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		events = append(events, LikeEvent{
			EventID: fmt.Sprint(message.Values["eventId"]),
			CardID:  parseInt64(message.Values["cardId"]),
			UserID:  parseInt64(message.Values["userId"]),
			Action:  fmt.Sprint(message.Values["action"]),
			TS:      parseInt64(message.Values["ts"]),
		})
		ids = append(ids, message.ID)
	}
	affected, err := s.Repo.Apply(ctx, events)
	if err != nil {
		return err
	}
	if _, err := s.Repo.Recount(ctx, affected); err != nil {
		return err
	}
	return s.Client.XAck(ctx, key, group, ids...).Err()
}

func parseInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case string:
		parsed, _ := strconv.ParseInt(typed, 10, 64)
		return parsed
	case []byte:
		parsed, _ := strconv.ParseInt(string(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
```

- [ ] **Step 5: 配置与 asynq 注册**

`config.go` 增加：

```go
CardLikeStreamShards int
CardLikeStreamMaxLen int64
CardLikeSyncBatch    int
CardLikeStreamTTL    time.Duration
```

默认值：`1`、`1000000`、`1000`、`7*24*time.Hour`。

`cmd/cp/main.go`：

```go
consumer, _ := os.Hostname()
if consumer == "" {
	consumer = "cp"
}
likeStore := businesscard.NewRedisLikeStore(redisClient, redisPrefix, cfg.CardLikeStreamMaxLen, cfg.CardLikeStreamShards)
likeRepo := businesscard.NewMySQLLikeRepository(db)
likeService := &businesscard.LikeService{Store: likeStore, Cards: businesscard.NewMySQLRepository(db), Repo: likeRepo}
likeSync := &businesscard.LikeSync{
	Client: redisClient, Repo: likeRepo,
	Prefix: redisPrefix, Group: "card-like-sync", Consumer: consumer,
	Batch: cfg.CardLikeSyncBatch, MaxLen: cfg.CardLikeStreamMaxLen,
	TTL: cfg.CardLikeStreamTTL, Shards: cfg.CardLikeStreamShards, Logger: obs.Logger,
}
asynqMux.HandleFunc(businesscard.TaskTypeLikeSync, likeSync.HandleTask)
if _, err := asynqScheduler.Register("@hourly", asynq.NewTask(businesscard.TaskTypeLikeSync, nil), asynq.MaxRetry(3), asynq.Timeout(10*time.Minute), asynq.Unique(time.Hour)); err != nil {
	obs.Logger.Error("card like sync scheduler register failed", "error", err)
	os.Exit(1)
}
```

`.env.example` 增加：

```bash
CARD_LIKE_STREAM_SHARDS=1
CARD_LIKE_STREAM_MAXLEN=1000000
CARD_LIKE_SYNC_BATCH=1000
CARD_LIKE_STREAM_TTL=168h
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd packages/backend && go test ./internal/businesscard ./internal/config -run 'TestLikeSync|TestConfig' -v && go test ./...`

Expected: PASS。

- [ ] **Step 7: 提交（用户要求时）**

```bash
git add packages/backend/internal/businesscard/like_repository.go packages/backend/internal/businesscard/like_sync.go packages/backend/internal/businesscard/like_sync_test.go packages/backend/internal/config/config.go packages/backend/internal/config/config_test.go packages/backend/cmd/cp/main.go packages/backend/.env.example
git commit -m "feat(card): sync likes from redis stream hourly"
```

---

### Task 8: Web 类型、API 与卡片模型

**Files:**
- Modify: `packages/web/src/types/index.ts`
- Modify: `packages/web/src/api/cards.ts`
- Modify: `packages/web/src/api/cards.test.ts`
- Modify: `packages/web/src/utils/card.ts`

**Interfaces:**
- Produces:
  - `CardComment`、`CardCommentPage`、`CardLikeState`
  - `getCardComments/getCardCommentReplies/createCardComment/updateCardComment/deleteCardComment/likeCard/unlikeCard`

- [ ] **Step 1: 写失败测试**

```ts
it('loads and mutates card comments', async () => {
  await getCardComments('card-1', { cursor: 'c1', limit: 20 })
  expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
    method: 'get',
    url: '/public/cards/card-1/comments',
    params: { cursor: 'c1', limit: 20 },
  })
  await createCardComment('card-1', { content: 'hi', parentId: 'c-0' })
  expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
    method: 'post',
    url: '/public/cards/card-1/comments',
    data: { content: 'hi', parentId: 'c-0' },
  })
})

it('likes and unlikes a card', async () => {
  await likeCard('card-1')
  expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
    method: 'put',
    url: '/public/cards/card-1/like',
  })
  await unlikeCard('card-1')
  expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
    method: 'delete',
    url: '/public/cards/card-1/like',
  })
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- --run src/api/cards.test.ts`

Expected: FAIL，`getCardComments` 未导出。

- [ ] **Step 3: 写类型与 API**

```ts
export interface CardCommentAuthor {
  userId: number
  name: string
  avatarUrl: string | null
}

export type CardCommentStatus = 'normal' | 'deleted'

export interface CardComment {
  id: string
  cardId: string
  parentId?: string
  rootId?: string
  depth: number
  content: string
  author: CardCommentAuthor
  replyToName?: string
  replyCount: number
  status: CardCommentStatus
  editable: boolean
  deletable: boolean
  editedAt: string | null
  createdAt: string
  replies?: CardComment[]
}

export interface CardCommentPage {
  comments: CardComment[]
  nextCursor: string
  hasMore: boolean
}

export interface CardLikeState {
  liked: boolean
  likeCount: number
}

export interface CardCommentDeleteResult {
  tombstoned: boolean
  commentCount: number
}
```

`BusinessCard` 增加 `likeCount: number`、`liked: boolean`。

`cards.ts`：

```ts
export function getCardComments(
  cardId: string,
  params: { cursor?: string; limit?: number } = {},
): Promise<CardCommentPage> {
  return request<CardCommentPage>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments`,
    params,
  })
}

export function createCardComment(
  cardId: string,
  input: { content: string; parentId?: string },
): Promise<CardComment> {
  return request<CardComment>(cpHttp, {
    method: 'post',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments`,
    data: input,
  })
}
```

```ts
export function deleteCardComment(
  cardId: string,
  commentId: string,
): Promise<CardCommentDeleteResult> {
  return request<CardCommentDeleteResult>(cpHttp, {
    method: 'delete',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`,
  })
}
```

```ts
export function getCardCommentReplies(
  cardId: string,
  commentId: string,
  params: { cursor?: string; limit?: number } = {},
): Promise<CardCommentPage> {
  return request<CardCommentPage>(cpHttp, {
    method: 'get',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}/replies`,
    params,
  })
}

export function updateCardComment(
  cardId: string,
  commentId: string,
  content: string,
): Promise<CardComment> {
  return request<CardComment>(cpHttp, {
    method: 'put',
    url: `/public/cards/${encodeURIComponent(cardId)}/comments/${encodeURIComponent(commentId)}`,
    data: { content },
  })
}

export function likeCard(cardId: string): Promise<CardLikeState> {
  return request<CardLikeState>(cpHttp, {
    method: 'put',
    url: `/public/cards/${encodeURIComponent(cardId)}/like`,
  })
}

export function unlikeCard(cardId: string): Promise<CardLikeState> {
  return request<CardLikeState>(cpHttp, {
    method: 'delete',
    url: `/public/cards/${encodeURIComponent(cardId)}/like`,
  })
}
```

`utils/card.ts` 的 `emptyCard()` 与 `normalizeCard()` 补 `likeCount: 0`、`liked: false`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/web && npm test -- --run src/api/cards.test.ts`

Expected: PASS。

- [ ] **Step 5: 提交（用户要求时）**

```bash
git add packages/web/src/types/index.ts packages/web/src/api/cards.ts packages/web/src/api/cards.test.ts packages/web/src/utils/card.ts
git commit -m "feat(web): add card comment and like api"
```

---

### Task 9: Web 评论树工具与 Store

**Files:**
- Create: `packages/web/src/utils/comment.ts`
- Create: `packages/web/src/utils/comment.test.ts`
- Create: `packages/web/src/stores/comment.ts`
- Create: `packages/web/src/stores/comment.test.ts`

**Interfaces:**
- Consumes: `CardComment`、评论 API
- Produces:
  - `findComment/patchComment/removeComment/mergeCommentPage`
  - `useCommentStore`

- [ ] **Step 1: 写失败测试**

```ts
it('patches a nested reply without touching siblings', () => {
  const tree: CardComment[] = [
    { ...baseComment('root'), replies: [{ ...baseComment('reply'), replies: [] }] },
  ]
  const next = patchComment(tree, 'reply', { content: '改过' })
  expect(next[0].replies?.[0].content).toBe('改过')
  expect(next[0].content).toBe('root')
})

it('removes a leaf reply and keeps the root', () => {
  const tree: CardComment[] = [
    { ...baseComment('root'), replies: [{ ...baseComment('reply') }] },
  ]
  const next = removeComment(tree, 'reply')
  expect(next[0].replies).toHaveLength(0)
})
```

Store 测试：

```ts
it('loads first page and appends next page', async () => {
  vi.mocked(getCardComments)
    .mockResolvedValueOnce({ comments: [baseComment('c1')], nextCursor: 'c1', hasMore: true })
    .mockResolvedValueOnce({ comments: [baseComment('c2')], nextCursor: '', hasMore: false })
  const store = useCommentStore()
  await store.loadRoots('card-1')
  await store.loadMore('card-1')
  expect(store.comments.map((item) => item.id)).toEqual(['c1', 'c2'])
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- --run src/utils/comment.test.ts src/stores/comment.test.ts`

Expected: FAIL，模块不存在。

- [ ] **Step 3: 写工具函数**

```ts
export function findComment(nodes: CardComment[], id: string): CardComment | null {
  for (const node of nodes) {
    if (node.id === id) return node
    const found = findComment(node.replies ?? [], id)
    if (found) return found
  }
  return null
}

export function patchComment(
  nodes: CardComment[],
  id: string,
  patch: Partial<CardComment>,
): CardComment[] {
  return nodes.map((node) => {
    if (node.id === id) return { ...node, ...patch }
    if (!node.replies?.length) return node
    return { ...node, replies: patchComment(node.replies, id, patch) }
  })
}

export function removeComment(nodes: CardComment[], id: string): CardComment[] {
  return nodes
    .filter((node) => node.id !== id)
    .map((node) =>
      node.replies?.length ? { ...node, replies: removeComment(node.replies, id) } : node,
    )
}

export function mergeCommentPage(
  current: CardComment[],
  page: CardComment[],
  append: boolean,
): CardComment[] {
  if (!append) return page
  const seen = new Set(current.map((item) => item.id))
  return [...current, ...page.filter((item) => !seen.has(item.id))]
}

export function countComments(nodes: CardComment[]): number {
  return nodes.reduce((total, node) => total + 1 + countComments(node.replies ?? []), 0)
}
```

- [ ] **Step 4: 写 Store**

```ts
function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

export const useCommentStore = defineStore('cardComment', () => {
  const comments = ref<CardComment[]>([])
  const nextCursor = ref('')
  const hasMore = ref(false)
  const loading = ref(false)
  const submitting = ref(false)
  const error = ref('')
  let activeCardId = ''

  async function loadRoots(cardId: string): Promise<void> {
    activeCardId = cardId
    loading.value = true
    try {
      const page = await getCardComments(cardId)
      comments.value = page.comments
      nextCursor.value = page.nextCursor
      hasMore.value = page.hasMore
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '评论加载失败')
    } finally {
      loading.value = false
    }
  }

  async function loadMore(cardId: string): Promise<void> {
    if (!hasMore.value || loading.value) return
    loading.value = true
    try {
      const page = await getCardComments(cardId, { cursor: nextCursor.value, limit: 20 })
      comments.value = mergeCommentPage(comments.value, page.comments, true)
      nextCursor.value = page.nextCursor
      hasMore.value = page.hasMore
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '评论加载失败')
    } finally {
      loading.value = false
    }
  }

  async function create(cardId: string, content: string, parentId?: string): Promise<CardComment> {
    submitting.value = true
    try {
      const created = await createCardComment(cardId, { content, parentId })
      if (parentId) {
        const parent = findComment(comments.value, parentId)
        const targetRootId = parent?.rootId || parentId
        comments.value = patchComment(comments.value, parentId, {
          replies: [...(parent?.replies ?? []), created],
        })
        comments.value = patchComment(comments.value, targetRootId, {
          replyCount: (findComment(comments.value, targetRootId)?.replyCount ?? 0) + 1,
        })
      } else {
        comments.value = [created, ...comments.value]
      }
      error.value = ''
      return created
    } catch (cause) {
      error.value = messageOf(cause, '评论发表失败')
      throw cause
    } finally {
      submitting.value = false
    }
  }

  async function loadReplies(cardId: string, rootId: string, cursor = ''): Promise<void> {
    const page = await getCardCommentReplies(cardId, rootId, { cursor, limit: 10 })
    const root = findComment(comments.value, rootId)
    if (!root) return
    comments.value = patchComment(comments.value, rootId, {
      replies: mergeCommentPage(root.replies ?? [], page.comments, Boolean(cursor)),
    })
  }

  async function update(cardId: string, commentId: string, content: string): Promise<void> {
    const updated = await updateCardComment(cardId, commentId, content)
    comments.value = patchComment(comments.value, commentId, updated)
  }

  async function remove(cardId: string, commentId: string): Promise<CardCommentDeleteResult> {
    const result = await deleteCardComment(cardId, commentId)
    if (result.tombstoned) {
      comments.value = patchComment(comments.value, commentId, {
        status: 'deleted',
        content: '',
        editable: false,
        deletable: false,
      })
    } else {
      comments.value = removeComment(comments.value, commentId)
    }
    return result
  }

  return { comments, nextCursor, hasMore, loading, submitting, error, loadRoots, loadMore, loadReplies, create, update, remove }
})
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/web && npm test -- --run src/utils/comment.test.ts src/stores/comment.test.ts`

Expected: PASS。

- [ ] **Step 6: 提交（用户要求时）**

```bash
git add packages/web/src/utils/comment.ts packages/web/src/utils/comment.test.ts packages/web/src/stores/comment.ts packages/web/src/stores/comment.test.ts
git commit -m "feat(web): add comment tree store"
```

---

### Task 10: 评论树组件与名片详情接入

**Files:**
- Create: `packages/web/src/components/card/CardCommentNode.vue`
- Create: `packages/web/src/components/card/CardCommentTree.vue`
- Create: `packages/web/src/components/card/CardCommentTree.test.ts`
- Modify: `packages/web/src/views/CardShareView.vue`
- Modify: `packages/web/src/views/CardShareView.test.ts`
- Modify: `packages/web/src/styles/card.css`

**Interfaces:**
- Consumes: `useCommentStore`
- Produces: 名片详情评论区

- [ ] **Step 1: 写失败测试**

```ts
it('renders root comments and only shows delete for own comments', async () => {
  vi.mocked(getCardComments).mockResolvedValue({
    comments: [
      {
        id: 'c1',
        cardId: 'card-1',
        depth: 1,
        content: '很不错的作品',
        author: { userId: 7, name: 'alice', avatarUrl: null },
        replyCount: 0,
        status: 'normal',
        editable: true,
        deletable: true,
        editedAt: null,
        createdAt: '2026-11-16T00:00:00Z',
      },
    ],
    nextCursor: '',
    hasMore: false,
  })
  await mount()
  expect(host.textContent).toContain('很不错的作品')
  expect(host.textContent).toContain('删除')
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- --run src/components/card/CardCommentTree.test.ts`

Expected: FAIL，组件不存在。

- [ ] **Step 3: 写节点组件**

`CardCommentNode.vue` 接收：

```vue
<script setup lang="ts">
import { ref } from 'vue'
import type { CardComment } from '@/types'

const props = defineProps<{ comment: CardComment; currentUserId: number | null }>()
const emit = defineEmits<{
  reply: [comment: CardComment]
  edit: [comment: CardComment, content: string]
  delete: [comment: CardComment]
  'load-replies': [comment: CardComment]
}>()

const editing = ref(false)
const draft = ref(props.comment.content)

function save() {
  if (!draft.value.trim()) return
  emit('edit', props.comment, draft.value)
  editing.value = false
}
</script>

<template>
  <article class="card-comment-node" :style="{ marginLeft: `${Math.min(props.comment.depth - 1, 2) * 20}px` }">
    <img v-if="props.comment.author.avatarUrl" class="card-comment-avatar" :src="props.comment.author.avatarUrl" alt="" />
    <span v-else class="card-comment-avatar">{{ props.comment.author.name.charAt(0).toUpperCase() }}</span>
    <div class="card-comment-body">
      <strong>{{ props.comment.author.name }}</strong>
      <span v-if="props.comment.depth === 3 && props.comment.replyToName" class="card-comment-reply-to">
        回复 @{{ props.comment.replyToName }}
      </span>
      <span v-if="props.comment.editedAt" class="card-comment-edited">已编辑</span>
      <p v-if="props.comment.status === 'deleted'" class="card-comment-deleted">该评论已删除</p>
      <template v-else>
        <textarea v-if="editing" v-model="draft" maxlength="500" />
        <p v-else class="card-comment-content">{{ props.comment.content }}</p>
        <div class="card-comment-actions">
          <button type="button" @click="emit('reply', props.comment)">回复</button>
          <button v-if="props.comment.editable" type="button" @click="editing = true">编辑</button>
          <button v-if="props.comment.deletable" type="button" @click="emit('delete', props.comment)">删除</button>
          <button v-if="editing" type="button" @click="save">保存</button>
          <button v-if="editing" type="button" @click="editing = false">取消</button>
        </div>
      </template>
      <button v-if="props.comment.replyCount > 0" type="button" class="card-comment-more" @click="emit('load-replies', props.comment)">
        查看 {{ props.comment.replyCount }} 条回复
      </button>
      <CardCommentNode
        v-for="reply in props.comment.replies"
        :key="reply.id"
        :comment="reply"
        :current-user-id="props.currentUserId"
        @reply="emit('reply', $event)"
        @edit="(item, content) => emit('edit', item, content)"
        @delete="(item) => emit('delete', item)"
        @load-replies="(item) => emit('load-replies', item)"
      />
    </div>
  </article>
</template>
```

模板规则：

- `status === 'deleted'` 显示「该评论已删除」，不显示操作按钮。
- `comment.editable` 显示「编辑」，`comment.deletable` 显示「删除」。
- `comment.replyCount > 0` 显示「查看 N 条回复」。
- 回复缩进 `:style="{ marginLeft: `${Math.min(comment.depth - 1, 2) * 20}px` }"`。
- `depth === 3` 的回复显示「回复 @{{ comment.replyToName }}」。
- 递归渲染 `comment.replies`：

```vue
<CardCommentNode
  v-for="reply in comment.replies"
  :key="reply.id"
  :comment="reply"
  :current-user-id="currentUserId"
  @reply="emit('reply', $event)"
  @edit="(item, content) => emit('edit', item, content)"
  @delete="(item) => emit('delete', item)"
  @load-replies="(item) => emit('load-replies', item)"
/>
```

- [ ] **Step 4: 写树组件**

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import CardCommentNode from './CardCommentNode.vue'
import { useCommentStore } from '@/stores/comment'
import { countComments } from '@/utils/comment'
import type { CardComment } from '@/types'

const props = defineProps<{ cardId: string; currentUserId: number | null; commentCount: number }>()
const emit = defineEmits<{ 'count-change': [count: number] }>()
const store = useCommentStore()
const draft = ref('')
const replyTo = ref<CardComment | null>(null)
const loadedCount = computed(() => countComments(store.comments))

onMounted(() => void store.loadRoots(props.cardId))

async function submit() {
  if (!draft.value.trim()) return
  await store.create(props.cardId, draft.value, replyTo.value?.id)
  draft.value = ''
  replyTo.value = null
  emit('count-change', props.commentCount + 1)
}

async function remove(item: CardComment) {
  const result = await store.remove(props.cardId, item.id)
  emit('count-change', result.commentCount)
}
</script>

<template>
  <section class="card-comments" id="comments">
    <header><h2>评论</h2></header>
    <form class="card-comment-form" @submit.prevent="submit">
      <textarea v-model="draft" maxlength="500" placeholder="写下你的评论" />
      <button type="submit" :disabled="store.submitting">发表</button>
    </form>
    <p v-if="store.error" class="error-text">{{ store.error }}</p>
    <p v-if="!store.loading && store.comments.length === 0" class="card-empty">还没有评论。</p>
    <CardCommentNode
      v-for="comment in store.comments"
      :key="comment.id"
      :comment="comment"
      :current-user-id="currentUserId"
      @reply="replyTo = $event"
      @edit="(item, content) => store.update(props.cardId, item.id, content)"
      @delete="remove"
      @load-replies="(item) => store.loadReplies(props.cardId, item.id)"
    />
    <p v-if="loadedCount >= 200" class="card-comments-hint">
      已加载 {{ loadedCount }} 条评论，继续加载可能影响页面性能。
    </p>
    <button v-if="store.hasMore" class="card-secondary" type="button" @click="store.loadMore(props.cardId)">
      加载更多评论
    </button>
  </section>
</template>
```

- [ ] **Step 5: 接入 CardShareView**

在项目列表后加入：

```vue
<CardCommentTree
  :card-id="card.id"
  :current-user-id="auth.user?.id ?? null"
  :comment-count="card.commentCount"
  @count-change="(count) => (card.commentCount = count)"
/>
```

底部按钮改为跳转评论区：

```vue
<button class="card-social-comment" type="button" @click="scrollToComments">
  <i class="pi pi-comment" />
  评论 · {{ card.commentCount }}
</button>
```

```ts
function scrollToComments() {
  document.getElementById('comments')?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
```

更新 `CardShareView.test.ts`：评论按钮不再 `disabled`；mock `getCardComments` 返回空页；删除旧的「评论按钮 disabled」断言。

`card.css` 增加：

```css
.card-comments {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 24px;
  padding-top: 18px;
  border-top: 1px solid var(--line);
}

.card-comment-node {
  display: flex;
  gap: 10px;
  content-visibility: auto;
  contain-intrinsic-size: auto 80px;
}

.card-comment-body {
  min-width: 0;
  flex: 1;
}

.card-comment-content {
  margin: 4px 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.card-comment-content.is-collapsed {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 8;
}

.card-comments-hint {
  color: var(--muted);
  font-size: 12px;
}
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd packages/web && npm test -- --run src/components/card/CardCommentTree.test.ts src/views/CardShareView.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交（用户要求时）**

```bash
git add packages/web/src/components/card/CardCommentNode.vue packages/web/src/components/card/CardCommentTree.vue packages/web/src/components/card/CardCommentTree.test.ts packages/web/src/views/CardShareView.vue packages/web/src/views/CardShareView.test.ts packages/web/src/styles/card.css
git commit -m "feat(web): render card comment tree"
```

---

### Task 11: 点赞按钮与发现页/详情页接入

**Files:**
- Create: `packages/web/src/components/card/CardLikeButton.vue`
- Create: `packages/web/src/components/card/CardLikeButton.test.ts`
- Modify: `packages/web/src/components/card/CardTile.vue`
- Modify: `packages/web/src/components/card/CardTile.test.ts`
- Modify: `packages/web/src/components/card/DiscoverPanel.vue`
- Modify: `packages/web/src/views/CardShareView.vue`
- Modify: `packages/web/src/stores/card.ts`
- Modify: `packages/web/src/stores/card.test.ts`

**Interfaces:**
- Consumes: `likeCard/unlikeCard`
- Produces: 心形按钮、乐观更新、卡片点赞状态

- [ ] **Step 1: 写失败测试**

```ts
it('emits toggle and renders liked state', () => {
  mount({ liked: true, count: 3 })
  expect(host.querySelector('.card-like-button.is-liked')).not.toBeNull()
  host.querySelector<HTMLButtonElement>('.card-like-button')?.click()
  expect(toggle).toHaveBeenCalled()
})
```

`CardTile` 测试：

```ts
it('emits like without opening the card', () => {
  const like = vi.fn()
  const open = vi.fn()
  mount({ ...emptyCard(), id: 'demo-1', items: [item], liked: false, likeCount: 2 }, {
    onLike: like,
    onOpen: open,
  })
  host.querySelector<HTMLButtonElement>('.card-like-button')?.click()
  expect(like).toHaveBeenCalledWith('demo-1')
  expect(open).not.toHaveBeenCalled()
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npm test -- --run src/components/card/CardLikeButton.test.ts src/components/card/CardTile.test.ts`

Expected: FAIL，组件/事件不存在。

- [ ] **Step 3: 写纯展示按钮**

```vue
<script setup lang="ts">
defineProps<{ liked: boolean; count: number; busy?: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
</script>

<template>
  <button
    class="card-like-button"
    :class="{ 'is-liked': liked }"
    type="button"
    :disabled="busy"
    @click.stop="emit('toggle')"
  >
    <i :class="liked ? 'pi pi-heart-fill' : 'pi pi-heart'" />
    {{ count }}
  </button>
</template>
```

- [ ] **Step 4: 接入 CardTile 与 DiscoverPanel**

`CardTile.vue`：

```vue
const emit = defineEmits<{
  open: [id: string]
  'open-featured': [id: string]
  like: [id: string]
}>()
```

footer 增加：

```vue
<CardLikeButton
  :liked="card.liked"
  :count="card.likeCount"
  @toggle="emit('like', card.id)"
/>
```

`DiscoverPanel.vue` 增加：

```ts
const router = useRouter()
const auth = useAuthStore()
const liking = ref<string | null>(null)

async function like(id: string) {
  if (!auth.isAuthenticated) {
    await router.push({ name: 'login', query: { redirect: '/discover' } })
    return
  }
  const card = store.discoverCards.find((item) => item.id === id)
  if (!card || liking.value) return
  liking.value = id
  try {
    const next = card.liked ? await unlikeCard(id) : await likeCard(id)
    card.liked = next.liked
    card.likeCount = next.likeCount
  } finally {
    liking.value = null
  }
}
```

- [ ] **Step 5: 详情页接入**

`CardShareView.vue` 头部操作区加入 `CardLikeButton`，并在 `auth` 未登录时跳转登录。点击后调用 `likeCard/unlikeCard`，用返回值校准 `card.liked` / `card.likeCount`。

```ts
const liking = ref(false)

async function toggleLike() {
  if (!card.value) return
  if (!auth.user) {
    await router.push({ name: 'login', query: { redirect: route.fullPath } })
    return
  }
  if (liking.value) return
  liking.value = true
  try {
    const next = card.value.liked ? await unlikeCard(card.value.id) : await likeCard(card.value.id)
    card.value.liked = next.liked
    card.value.likeCount = next.likeCount
  } finally {
    liking.value = false
  }
}
```

```vue
<CardLikeButton
  :liked="card.liked"
  :count="card.likeCount"
  :busy="liking"
  @toggle="toggleLike"
/>
```

- [ ] **Step 6: 运行测试确认通过**

Run: `cd packages/web && npm test -- --run src/components/card/CardLikeButton.test.ts src/components/card/CardTile.test.ts src/views/CardShareView.test.ts`

Expected: PASS。

- [ ] **Step 7: 提交（用户要求时）**

```bash
git add packages/web/src/components/card/CardLikeButton.vue packages/web/src/components/card/CardLikeButton.test.ts packages/web/src/components/card/CardTile.vue packages/web/src/components/card/CardTile.test.ts packages/web/src/components/card/DiscoverPanel.vue packages/web/src/views/CardShareView.vue packages/web/src/stores/card.ts packages/web/src/stores/card.test.ts
git commit -m "feat(web): add card like button"
```

---

### Task 12: 指标、文档、契约同步与全量验证

**Files:**
- Modify: `packages/backend/internal/observability/metrics.go`
- Modify: `packages/backend/internal/handler/card_comment.go`
- Modify: `packages/backend/internal/handler/card_like.go`
- Modify: `packages/backend/internal/businesscard/like_sync.go`
- Modify: `.ai/rules/architecture.md`
- Modify: `docs/superpowers/specs/2026-11-16-business-card-comments-likes-design.md`
- Modify: `.ai/CURRENT_FOCUS`

- [ ] **Step 1: 增加社交指标**

`observability.Metrics` 增加：

```go
Comments       *prometheus.CounterVec
Likes          *prometheus.CounterVec
LikeSyncSeconds prometheus.Histogram
LikeStreamPending prometheus.Gauge
```

注册与记录方法：

```go
m.Comments = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "vibebiz_card_comments_total",
	Help: "Total number of card comment mutations.",
}, []string{"action", "status"})
m.Likes = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "vibebiz_card_likes_total",
	Help: "Total number of card like mutations.",
}, []string{"action", "status"})
m.LikeSyncSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
	Name: "vibebiz_card_like_sync_duration_seconds",
	Help: "Card like stream sync duration.",
	Buckets: prometheus.DefBuckets,
})
m.LikeStreamPending = prometheus.NewGauge(prometheus.GaugeOpts{
	Name: "vibebiz_card_like_stream_pending",
	Help: "Card like stream pending messages.",
})

func (m *Metrics) RecordCardComment(action, status string) {
	m.Comments.WithLabelValues(action, status).Inc()
}

func (m *Metrics) RecordCardLike(action, status string) {
	m.Likes.WithLabelValues(action, status).Inc()
}

func (m *Metrics) ObserveCardLikeSync(seconds float64) {
	m.LikeSyncSeconds.Observe(seconds)
}

func (m *Metrics) SetCardLikeStreamPending(n float64) {
	m.LikeStreamPending.Set(n)
}
```

`CardCommentHandler` 与 `CardLikeHandler` 增加可选 `Metrics *observability.Metrics` 字段，在成功/失败分支调用 `RecordCardComment` / `RecordCardLike`。`LikeSync.Run` 用 `time.Since(start).Seconds()` 调 `ObserveCardLikeSync`，每轮读取 `XPending` 后调 `SetCardLikeStreamPending`。

- [ ] **Step 2: 更新架构规则**

在 `.ai/rules/architecture.md` 的 CP API 表中加入：

```text
| GET/POST | `/public/cards/:id/comments` | 评论列表 / 发表评论 |
| GET | `/public/cards/:id/comments/:commentId/replies` | 回复分页 |
| PUT/DELETE | `/public/cards/:id/comments/:commentId` | 编辑 / 删除自己的评论 |
| PUT/DELETE | `/public/cards/:id/like` | 点赞 / 取消点赞 |
```

在 Redis 说明中补充：

```text
- 名片点赞先写 Redis：`vibebiz:card:like:count`、`vibebiz:card:like:users:{cardID}`、`vibebiz:card:like:stream`。
- asynq `@hourly` 任务 `card:like:sync` 批量落库，MySQL 唯一键 + `COUNT(*)` 重算保证幂等。
```

- [ ] **Step 3: 标记 spec 已确认**

将 spec 头部状态改为 `已确认`。

- [ ] **Step 4: 更新 CURRENT_FOCUS**

将 `next_tasks` 中加入：

```text
"名片评论与点赞：树形评论 + 作者编辑删除 + Redis Stream 点赞小时同步",
```

- [ ] **Step 5: 运行全量验证**

```bash
cd packages/backend && gofmt -w . && go test ./... && go vet ./...
cd ../web && npm test -- --run && npm run build && npm run lint
cd ../.. && make validate
```

Expected: 全部通过。

- [ ] **Step 6: 提交（用户要求时）**

```bash
git add packages/backend/internal/observability/metrics.go packages/backend/internal/handler/card_comment.go packages/backend/internal/handler/card_like.go packages/backend/internal/businesscard/like_sync.go .ai/rules/architecture.md .ai/CURRENT_FOCUS docs/superpowers/specs/2026-11-16-business-card-comments-likes-design.md
git commit -m "docs: record card comments and likes contract"
```

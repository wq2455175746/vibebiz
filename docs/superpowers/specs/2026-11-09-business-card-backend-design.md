# 个人名片后端与真实数据接入设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-09 |
| 状态 | 设计稿 |
| 范围 | CP（Go + MySQL + S3）、web、ops-web；移除名片前端 mock 与 localStorage |
| 关联 | `docs/superpowers/specs/2026-11-08-business-card-social-design.md`、`.ai/rules/architecture.md`、`docs/superpowers/specs/2026-10-28-business-database-design.md` |
| 后续 | 评论 / 私信（IM） |

---

## 1. 背景与目标

当前名片功能只做了前端效果：名片、发现页、分享页的数据来自 localStorage 与 12 条 mock 名片，ops-web 名片管理也是独立 mock。本期把名片落到真实后端：

1. CP 新增名片聚合（用户名片、项目条目、主预览、标签、发布状态）。
2. MySQL 新增 `business_card`、`business_card_item` 两张表。
3. 主预览的图片 / PDF 上传到 S3，MySQL 只存 key、类型、文件名与大小。
4. 项目条目只存 `project_public_id`，读取时从 `project` 与发布信息实时解析。
5. web 的“我的名片”“发现”“名片分享页”“项目分享页”全部接 CP 真实接口，删除 mock 与 localStorage。
6. ops-web 名片管理接真实 admin 接口，支持搜索、隐藏/恢复、删除。
7. 公开名片分享页与项目分享页无需登录；发现页仍需登录。

非目标：

- 不做评论、点赞、关注、私信真实接口；`comment_count` 固定 0 占位。
- 不做多张名片；一个用户一张。
- 不做名片历史版本、草稿箱、审核流。
- 不做 PDF.js 渲染；主预览继续用浏览器原生 PDF 阅读器（无 sandbox iframe + `blob:`/`https:` URL）。
- 不做旧 localStorage 名片迁移；旧 mock 数据直接丢弃。
- 不做图片缩略图、CDN、病毒扫描。
- 不改变现有项目发布 `/p/:slug` 行为。

## 2. 关键决策

| 决策 | 结论 |
|---|---|
| 名片数量 | 一个用户一张，`user_id` 唯一 |
| 项目条目 | 只存 `project_public_id + sort_order`，读取时实时解析 |
| 主预览文件 | S3 存对象，MySQL 存 key/type/name/size |
| 分享链接 | 使用名片 `public_id`（UUID），形如 `/share/card/<uuid>` |
| 项目分享页 | 公开接口按 `projectId` 返回项目条目 + 名片主人；真实项目嵌入 `/p/:slug` |
| 发现页 | 登录可见，只返回已发布、未隐藏、至少一个可见项目的名片 |
| ops 删除 | 硬删除名片 + 条目，并删除主预览 S3 对象 |
| 旧 mock | web `mocks/cards.ts` 与 ops mock 全部删除 |

## 3. 总体架构

```
web /my-card ── GET/PUT /cp/api/v1/me/card ───────────────┐
web /discover ── GET /cp/api/v1/cards/discover ───────────┤
web /share/card/:id ── GET /cp/api/v1/public/cards/:id ───┤
web /share/project/:id ── GET /public/card-projects/:id ──┤
ops-web /cards ── /cp/api/v1/admin/cards ─────────────────┤
                                                          ▼
                                              handler.CardHandler
                                                          │
                                                          ▼
                                              internal/businesscard.Service
                                                │        │        │
                                                ▼        ▼        ▼
                                          Repository  ProjectStore  S3
                                          (MySQL)     publish.List  (Presigner)
```

依赖方向不变：

- `web → CP`、`ops-web → CP admin`。
- CP 使用 MySQL 与 S3；不新增外部服务。
- 不改 DP / sandbox。

## 4. 数据库设计

迁移：`packages/backend/migrations/00033_business_card.sql`。

### 4.1 `business_card`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint unsigned PK | 内部 ID |
| `public_id` | char(36) | 名片公开 ID，分享链接 |
| `user_id` | bigint unsigned | 所属用户，唯一 |
| `headline` | varchar(80) | 一句话简介 |
| `tags` | json | 标签数组，最多 5 个 |
| `featured_type` | varchar(16) | `''` / `project` / `image` / `pdf` |
| `featured_project_public_id` | char(36) | 项目主预览 |
| `featured_s3_key` | varchar(512) | 文件主预览 S3 key |
| `featured_file_name` | varchar(255) | 原始文件名 |
| `featured_file_size` | bigint unsigned | 文件大小 |
| `published` | tinyint unsigned | 是否已发布 |
| `published_at` | datetime | 首次/最近发布时间 |
| `comment_count` | int unsigned | 评论数占位 0 |
| `is_hidden` | tinyint unsigned | ops 隐藏 |
| `created_at` / `updated_at` / `deleted_at` / `extra_p` | 标准字段 | 与现有表一致 |

索引：

- `UNIQUE KEY uk_business_card_public (public_id)`
- `UNIQUE KEY uk_business_card_user (user_id)`
- `KEY idx_business_card_discover (published, is_hidden, updated_at)`

### 4.2 `business_card_item`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint unsigned PK | 内部 ID |
| `card_id` | bigint unsigned | `business_card.id` |
| `project_public_id` | char(36) | 项目 public_id |
| `sort_order` | int | 展示顺序 |
| `created_at` / `updated_at` / `deleted_at` / `extra_p` | 标准字段 | 与现有表一致 |

索引：

- `UNIQUE KEY uk_business_card_item (card_id, project_public_id)`
- `KEY idx_business_card_item_project (project_public_id)`
- `KEY idx_business_card_item_order (card_id, sort_order)`

CONSTRAINT：

- `business_card_item.card_id` 外键到 `business_card.id`，`ON DELETE CASCADE`。
- `business_card_item.project_public_id` 不建外键（项目以 public_id 关联，项目删除后名片读取时过滤）。
- 名片条目删除使用硬删除（`Unscoped().Delete`），避免软删除记录与 `(card_id, project_public_id)` 唯一键冲突；重新添加时直接插入。

### 4.3 约束

- 一个用户最多一张名片；`GET /me/card` 自动创建空名片。
- 名片最多 5 个项目、5 个标签；单标签 ≤12 字，去重。
- 项目条目只能添加：项目存在、未隐藏、已发布、属于当前用户可访问工作区。
- 主预览项目必须在当前条目中；项目失效时响应中 `featured` 置空。
- 发布要求至少一个可见项目。
- `comment_count` 本期固定 0。

## 5. 后端设计

### 5.1 模型

新增 `packages/backend/internal/dao/model/business_card.go`：

```go
type BusinessCard struct {
    Base
    PublicID              string `gorm:"column:public_id"`
    UserID                int64  `gorm:"column:user_id"`
    Headline              string `gorm:"column:headline"`
    Tags                  JSON   `gorm:"column:tags;type:json"`
    FeaturedType          string `gorm:"column:featured_type"`
    FeaturedProjectPublicID string `gorm:"column:featured_project_public_id"`
    FeaturedS3Key         string `gorm:"column:featured_s3_key"`
    FeaturedFileName      string `gorm:"column:featured_file_name"`
    FeaturedFileSize      int64  `gorm:"column:featured_file_size"`
    Published             int8   `gorm:"column:published"`
    PublishedAt           *time.Time `gorm:"column:published_at"`
    CommentCount          int    `gorm:"column:comment_count"`
    IsHidden              int8   `gorm:"column:is_hidden"`
}
func (BusinessCard) TableName() string { return "business_card" }

type BusinessCardItem struct {
    Base
    CardID          int64  `gorm:"column:card_id"`
    ProjectPublicID string `gorm:"column:project_public_id"`
    SortOrder       int    `gorm:"column:sort_order"`
}
func (BusinessCardItem) TableName() string { return "business_card_item" }
```

字段命名在实现时按仓库现有 model 风格统一（JSON tag 与 snake_case column）。

### 5.2 `internal/businesscard`

```go
type Repository interface {
    GetByUser(ctx context.Context, userID int64) (*CardRecord, error)
    GetOrCreate(ctx context.Context, userID int64) (*CardRecord, error)
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
    Discover(ctx context.Context, page, pageSize int) ([]CardRecord, int64, error)
    AdminList(ctx context.Context, keyword string, page, pageSize int) ([]CardRecord, int64, error)
    FindByProject(ctx context.Context, projectID string) (*CardRecord, error)
}
```

`CardRecord` 包含名片字段、主人用户名/昵称/头像 S3 path、条目列表；`ItemRecord` 包含 `ProjectPublicID`、`SortOrder`。

`Service` 依赖：

```go
type Service struct {
    Repo     Repository
    Projects store.ProjectStore
    Publish  interface{ List(ctx context.Context) ([]publish.Published, error) }
    S3       S3
    Identity interface{ UserProfile(ctx context.Context, userID int64) (*dao.UserProfile, error) }
    PublicBaseURL string
    PresignExpiry time.Duration
    MaxFeaturedBytes int64
    Now      func() time.Time
    NewID    func() string
}

type S3 interface {
    Put(ctx context.Context, key, contentType string, size int64, body io.Reader) error
    PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
    Delete(ctx context.Context, key string) error
}
```

### 5.3 响应模型

```go
type Card struct {
    ID          string        `json:"id"`
    OwnerUserID int64         `json:"ownerUserId"`
    OwnerName   string        `json:"ownerName"`
    AvatarURL   string        `json:"avatarUrl"`
    Headline    string        `json:"headline"`
    Tags        []string      `json:"tags"`
    Items       []CardItem    `json:"items"`
    Featured    *Featured     `json:"featured"`
    Published   bool          `json:"published"`
    CommentCount int          `json:"commentCount"`
    CreatedAt   time.Time     `json:"createdAt"`
    UpdatedAt   time.Time     `json:"updatedAt"`
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
    Kind     string `json:"kind"`      // project | file
    ProjectID string `json:"projectId,omitempty"`
    FileType string `json:"fileType,omitempty"` // pdf | image
    Name     string `json:"name,omitempty"`
    Size     int64  `json:"size,omitempty"`
    URL      string `json:"url,omitempty"`
}
```

### 5.4 解析规则

项目条目：

1. `projectStore.Get(projectID)`；不存在、隐藏、工作区不可见 → 过滤。
2. `publish.Service.List()` 构建 `publishedByProject[projectID]`；未发布 → 过滤。
3. `Name` 优先发布标题，回退项目名；`Description` 用发布简介。
4. `CoverURL`：`project.CoverS3Path` 预签名，回退 `project.CoverURL`。
5. `Type`：`previewPath` 后缀映射 `html/image/pdf/video/document`，默认 `html`。
6. `ShareURL`：`PublicBaseURL + "/p/" + projectID + "/"`；`PublicBaseURL` 为空时由 handler 用请求的 scheme/host 补全为绝对地址。

主预览：

- `featured_type=project` 且项目仍在可见条目中 → `{kind:"project",projectId}`；否则 `featured=null`。
- `featured_type=pdf/image` → `{kind:"file",fileType,name,size,url}`，`url` 为 `featured_s3_key` 的预签名 GET。
- 没有主预览 → `null`。

发现：

- 查询 `published=1 AND is_hidden=0 AND deleted_at IS NULL`，按 `updated_at DESC` 分页（默认 12，最大 50）。
- 解析条目后过滤零可见条目的名片；接受页内数量少于 `pageSize`。
- 返回 `total` 为数据库候选总数（可能略大于实际可展示数）。

管理员：

- 查询全部未删除名片（含未发布、已隐藏），支持按 Owner 用户名 / 昵称 / 名片 headline 关键词过滤。
- 行状态：`hidden` → `已隐藏`；`!published` → `未发布`；否则 `正常`。
- 隐藏/恢复更新 `is_hidden`；删除硬删并清理 S3。

### 5.5 S3 上传

`POST /me/card/featured/upload`：

- `Content-Type`：`application/pdf` / `image/png` / `image/jpeg` / `image/webp` / `image/gif`。
- `X-File-Name`：URL 编码原始文件名。
- 大小：`>0 && <= 2MB`。
- Magic bytes 校验：
  - PDF: `%PDF-`
  - PNG: `89 50 4E 47 0D 0A 1A 0A`
  - JPEG: `FF D8 FF`
  - GIF: `GIF87a` / `GIF89a`
  - WebP: `RIFF....WEBP`
- key：`cards/<userID>/<uuid>.<ext>`。
- 写 S3 成功后更新 `featured_type/file_name/file_size/s3_key`；旧 key 异步删除（失败只记日志）。
- S3 未配置 → HTTP 503，业务码 50304。
- 清除主预览 / ops 删除 / 替换文件时删除旧 S3 对象。

## 6. API 契约

统一信封 `{code,message,result}`。

### 6.1 我的名片

`GET /cp/api/v1/me/card`

- 登录用户；不存在则创建空名片。
- 返回 `Card`。

`PUT /cp/api/v1/me/card`

```json
{ "headline": "前端工程师", "tags": ["Vue", "游戏"] }
```

- headline ≤80 字；tags 最多 5、单标签 ≤12、去重。
- 返回更新后的 `Card`。

`POST /cp/api/v1/me/card/items`

```json
{ "projectId": "p1" }
```

- 校验项目存在、未隐藏、已发布、属于用户工作区；最多 5 个；不可重复。
- 返回更新后的 `Card`。

`DELETE /cp/api/v1/me/card/items/:projectId`

- 返回更新后的 `Card`。

`PUT /cp/api/v1/me/card/featured`

```json
{ "kind": "project", "projectId": "p1" }
```

- 项目必须在条目中；设置后清除旧文件 key（并删除 S3 对象）。
- 返回更新后的 `Card`。

`POST /cp/api/v1/me/card/featured/upload`

- raw body；返回更新后的 `Card`。
- 错误：400 类型/空文件/超限（41302）、503 未配置 S3。

`DELETE /cp/api/v1/me/card/featured`

- 清除项目/文件主预览；删除旧 S3 对象；返回更新后的 `Card`。

`POST /cp/api/v1/me/card/publish`

- 至少一个可见项目，否则 400；设置 published=1、published_at=now；返回更新后的 `Card`。

### 6.2 发现

`GET /cp/api/v1/cards/discover?page=1&pageSize=12`

- 登录用户。
- 返回：

```json
{
  "cards": [Card],
  "total": 20,
  "page": 1,
  "pageSize": 12
}
```

### 6.3 公开分享

`GET /cp/api/v1/public/cards/:id`

- 无登录。
- 未发布也允许直达；`is_hidden=1` 或已删除 → 404。
- 返回 `Card`。

`GET /cp/api/v1/public/card-projects/:projectId`

- 无登录。
- 项目未发布、隐藏、不存在 → 404。
- 返回：

```json
{
  "card": { "id": "uuid", "ownerName": "alice", "avatarUrl": "..." },
  "item": { "projectId": "p1", "name": "...", "description": "...", "coverUrl": "...", "type": "html", "shareUrl": "..." }
}
```

- 若项目在某个名片中，主人取该名片；否则取项目 owner。

### 6.4 ops 名片管理

`GET /cp/api/v1/admin/cards?keyword=&page=1&pageSize=10`

- RequireOps。
- 返回：

```json
{
  "items": [
    {
      "id": "uuid",
      "ownerUserId": 7,
      "ownerName": "alice",
      "headline": "前端工程师",
      "tags": ["Vue"],
      "projectCount": 3,
      "featuredType": "PDF",
      "published": true,
      "hidden": false,
      "status": "正常",
      "updatedAt": "2026-11-09T10:00:00Z"
    }
  ],
  "total": 1,
  "page": 1,
  "pageSize": 10
}
```

`PUT /cp/api/v1/admin/cards/:id/hidden`

```json
{ "hidden": true }
```

返回 `{ "id": "uuid", "hidden": true }`。

`DELETE /cp/api/v1/admin/cards/:id`

- 硬删除名片与条目，删除主预览 S3 对象。
- 返回 `{ "id": "uuid" }`。

## 7. 错误码

| 场景 | HTTP | code |
|---|---|---|
| 参数错误 / 超限 / 重复项目 | 400 | 40001 |
| 项目不存在 / 未发布 / 无权访问 | 400 | 40016 |
| 未登录 | 401 | 40101 |
| 名片不存在 | 404 | 40410 |
| 项目不存在 / 已下线 | 404 | 40411 |
| 上传文件过大 | 413 | 41302 |
| S3 未配置 | 503 | 50304 |
| 内部错误 | 500 | 50016 |

## 8. Web 改造

### 8.1 API 与类型

新增 `packages/web/src/api/cards.ts`：

- `getMyCard()`
- `updateMyCard({headline,tags})`
- `addCardProject(projectId)`
- `removeCardProject(projectId)`
- `setCardFeaturedProject(projectId)`
- `uploadCardFeatured(file)`
- `clearCardFeatured()`
- `publishMyCard()`
- `getDiscoverCards({page,pageSize})`
- `getPublicCard(id)`
- `getPublicCardProject(projectId)`

类型调整：

- `BusinessCard.featured` 文件分支改为 `{kind:'file', fileType:'pdf'|'image', name, size, url}`。
- `CardProjectItem` 增加 `shareUrl`；`previewUrl` 可移除。
- 新增 `DiscoverCardsResult`、`PublicCardProject`、`AdminCardSummary`（ops-web）。

### 8.2 Store

`useCardStore` 改为 API 驱动：

- 状态：`mine`、`loading`、`saving`、`error`、`discoverCards`、`discoverTotal`、`discoverLoading`。
- `loadMine()`：`getMyCard()`。
- `saveProfile(headline,tags)`：防抖 400ms 调用 `updateMyCard()`；失败回滚并提示；`flushProfile()` 在 `MyCardPanel` 卸载前强制提交最后一次修改。
- `addProject` / `removeProject` / `setFeaturedProject` / `uploadFeatured` / `clearFeatured` / `publish`：调用对应 API，用返回的 `Card` 替换 `mine`。
- `loadDiscover(page,pageSize)`：`getDiscoverCards()`，追加或替换。
- `getPublicCard(id)` / `getPublicProject(id)`：直接返回 API 数据。
- 删除 `vibebiz:cards:*` localStorage 读写与 mock 种子。
- 删除 `src/mocks/cards.ts`；`CardPreview` 只保留通用占位图（`CARD_COVER_PLACEHOLDER`，放在 `src/utils/card.ts`）。

### 8.3 页面

- `MyCardPanel`：`onMounted` 加载真实名片；加载/错误态；保存状态来自 API。
- `CardProfilePanel`：上传改为 `emit('upload-featured', file)`；不再读 base64；主预览显示 `featured.url`。
- `DiscoverPanel`：首屏调用 `loadDiscover(1)`；滚动/加载更多调用下一页；`total` 来自接口；无 mock。
- `CardShareView`：公开接口加载名片；评论/私信禁用；编辑名片仅 id 对应自己的名片时显示（后端返回 `isOwn` 可选；本期可用登录态 `auth.user.id === card.ownerUserId` 判断）。
- `ProjectShareView`：公开接口加载项目条目；HTML 用 `shareUrl` iframe。本期后端不返回 `previewUrl`，非 HTML 项目展示“暂无可预览内容”并保留“打开原项目”；前端保留 `previewUrl` 渲染分支（图片/PDF/视频/文档），供后续后端返回该字段时使用。
- 分享链接：`/share/card/${card.id}`。
- PDF 预览保持：`blob:`/`https:` URL、PDF iframe 不加 sandbox、HTML iframe 保持 `allow-scripts allow-forms`。

### 8.4 移除

- 删除 `src/mocks/cards.ts`。
- 删除 `CardTile`/`DiscoverPanel` 中的 mock 依赖。
- 删除旧 `DEMO_HTML` / `DEMO_PDF` / `DEMO_VIDEO` / `DEMO_DOCUMENT` 演示资源，仅保留通用封面占位图。
- 旧 localStorage 名片数据不读取、不迁移。

## 9. ops-web 改造

- `src/api/cards.ts` 改为真实 admin 请求：
  - `listCards({keyword,page,pageSize})` → `GET /cards`
  - `updateCardHidden(id,hidden)` → `PUT /cards/:id/hidden`
  - `deleteCard(id)` → `DELETE /cards/:id`
- 删除 `src/mocks/cards.ts`。
- `CardsView` 列：名片（ownerName/headline）、用户、标签、项目数、主预览、状态（正常/未发布/已隐藏）、更新时间、操作。
- 隐藏/恢复、删除二次确认；操作后重新加载。
- 只访问 `/cp/api/v1/admin/cards`，不访问 DP/sandbox。

## 10. 边界与异常

| 场景 | 行为 |
|---|---|
| S3 未配置 | 上传 503；已存在的文件主预览仍可显示；文本名片可用 |
| 项目取消发布 | 名片读取时过滤，`published` 仍为 true；发现页无可见项目时过滤该名片 |
| 项目改名/换封面 | 名片读取时实时更新 |
| 主预览项目被移除 | 响应中 `featured=null`；数据库在移除时清空 |
| 重复添加项目 | 400 |
| 超过 5 项目/标签 | 400 |
| 上传空文件 / 超 2MB / MIME 与 magic 不符 | 400 / 413 |
| 名片被 ops 隐藏 | 公开接口 404；发现页不返回 |
| 名片被 ops 删除 | 硬删除；公开接口 404；用户下次 `GET /me/card` 新建空名片 |
| 分享页未发布 | 允许直达，发现页不展示 |
| 公开项目不存在/未发布 | 404 |
| 名片 `comment_count` | 固定 0 |

## 11. 测试

### Go

- `internal/businesscard/service_test.go`：
  - 自动创建、更新简介/标签去重超限、添加/移除项目、5 项上限、重复项目。
  - 只能添加已发布且可见项目；未发布项目读取时过滤。
  - 主预览项目校验与失效置空。
  - 发布要求至少一个可见项目。
  - 发现过滤未发布/隐藏/零可见项目。
  - admin 隐藏/删除；删除返回 featured key。
  - S3 上传类型/magic/大小、旧 key 删除。
- `internal/businesscard/repository_test.go`：sqlmock 覆盖 GetOrCreate、ListItems、Discover、AdminList、FindByProject、Delete。
- `internal/handler/card_test.go`：httptest 覆盖用户/公开/admin 路由、400/401/404/413/503、信封。
- `internal/router` 现有测试确认新增路由与鉴权边界。
- `migrations/migrations_test.go`：新增两张表名。
- `go test ./...`、`go vet`。

### Web

- `api/cards.test.ts`：各接口 URL/方法/信封、上传 header。
- `stores/card.test.ts`：异步加载、保存、添加/移除、主预览、发布、发现分页、错误回滚。
- `DiscoverPanel` / `MyCardPanel` / `CardShareView` / `ProjectShareView`：真实 API mock，加载/空/错误态、点击行为。
- `vitest run`、`vue-tsc`、`eslint`。

### ops-web

- `api/cards.test.ts`：真实 URL/方法。
- `CardsView.test.ts`：状态列、隐藏/恢复、删除确认、分页。
- `vitest run`、`vue-tsc`、`eslint`。

### 最终

```bash
go test ./...
cd packages/web && npm test -- --run && npm run build && npm run lint
cd packages/ops-web && npm test -- --run && npm run build && npm run lint
make validate
```

## 12. 验收标准

1. `GET /me/card` 自动创建并返回唯一名片；再次调用不重复创建。
2. 可以更新简介与最多 5 个标签；超限返回 400。
3. 只能添加自己已发布的项目，最多 5 个；重复添加被拒绝。
4. 主预览可选已添加项目或上传 ≤2MB 的图片/PDF；文件写入 S3，分享页能正常预览。
5. 点击分享后名片 marked published，`/share/card/:id` 可匿名访问并展示内容模块 + 个人信息。
6. 发现页只展示真实已发布名片，滚动分页；不再出现 mock 名片。
7. 项目分享页按 `projectId` 获取真实项目条目；HTML 项目嵌入 `/p/:slug`；点击项目进入对应分享页。
8. ops-web 名片管理展示真实名片，搜索、隐藏/恢复、删除生效；隐藏后公开页 404。
9. 删除名片后，用户重新进入“我的名片”得到新的空名片。
10. 现有项目发布、排行榜、设置等功能不回归；`make validate` 通过。

## 13. 文件清单

### Backend

新增：

- `packages/backend/migrations/00033_business_card.sql`
- `packages/backend/internal/dao/model/business_card.go`
- `packages/backend/internal/businesscard/repository.go`
- `packages/backend/internal/businesscard/service.go`
- `packages/backend/internal/businesscard/service_test.go`
- `packages/backend/internal/businesscard/repository_test.go`
- `packages/backend/internal/handler/card.go`
- `packages/backend/internal/handler/card_test.go`

修改：

- `packages/backend/internal/router/control.go`
- `packages/backend/cmd/cp/main.go`
- `packages/backend/internal/router/helpers_test.go`
- `packages/backend/internal/router/span_test.go`
- `packages/backend/migrations/migrations_test.go`

### Web

新增：

- `packages/web/src/api/cards.ts`
- `packages/web/src/api/cards.test.ts`

修改：

- `packages/web/src/types/index.ts`
- `packages/web/src/utils/card.ts`（`CARD_COVER_PLACEHOLDER`）
- `packages/web/src/stores/card.ts`
- `packages/web/src/stores/card.test.ts`
- `packages/web/src/components/card/MyCardPanel.vue`
- `packages/web/src/components/card/MyCardPanel.test.ts`
- `packages/web/src/components/card/CardProfilePanel.vue`
- `packages/web/src/components/card/CardProfilePanel.test.ts`
- `packages/web/src/components/card/CardPreview.vue`
- `packages/web/src/components/card/CardPreview.test.ts`
- `packages/web/src/components/card/CardTile.vue`
- `packages/web/src/components/card/CardTile.test.ts`
- `packages/web/src/components/card/DiscoverPanel.vue`
- `packages/web/src/components/card/DiscoverPanel.test.ts`
- `packages/web/src/views/CardShareView.vue`
- `packages/web/src/views/CardShareView.test.ts`
- `packages/web/src/views/ProjectShareView.vue`
- `packages/web/src/views/ProjectShareView.test.ts`
- `packages/web/src/styles/card.css`（如需要）

删除：

- `packages/web/src/mocks/cards.ts`

### ops-web

修改：

- `packages/ops-web/src/api/cards.ts`
- `packages/ops-web/src/api/cards.test.ts`
- `packages/ops-web/src/types/index.ts`
- `packages/ops-web/src/views/CardsView.vue`
- `packages/ops-web/src/views/CardsView.test.ts`

删除：

- `packages/ops-web/src/mocks/cards.ts`

### 不修改

- `packages/backend/internal/publish` 的 `/p/:slug` 行为。
- DP / sandbox / 现有排行榜。
- 评论/私信后端。

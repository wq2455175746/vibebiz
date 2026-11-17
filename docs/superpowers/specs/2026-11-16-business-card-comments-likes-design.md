# 名片评论与点赞 · 设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-16 |
| 状态 | 待评审 |
| 范围 | 发现页名片评论树、评论编辑删除、名片点赞、Redis 小时级落库 |
| 关联文档 | `docs/superpowers/specs/2026-11-08-business-card-social-design.md`、`docs/superpowers/specs/2026-11-09-business-card-backend-design.md`、`.ai/rules/architecture.md` |
| 数据库 | MySQL 8；新增评论表、点赞表，`business_card` 增加点赞数 |
| 缓存 | Redis；点赞计数与事件流先写 Redis，每小时批量同步 MySQL |

---

## 1. 背景与目标

发现页已经具备名片列表与名片详情，但社交能力仍是占位：

- `business_card.comment_count` 字段已存在，但没有评论表与评论 API。
- 名片分享页的「评论」按钮是 `disabled` 占位。
- 名片没有点赞字段、点赞 API 或 Redis 计数。
- 发现页只展示名片卡片，没有评论和点赞交互。

本规格要达成四个目标：

1. **名片评论树**：支持根评论、回复、查看回复、分页加载，避免一次加载整棵树导致页面崩溃。
2. **个人评论管理**：作者可以编辑、删除自己的评论；删除后有回复的节点保留占位，树结构不断裂。
3. **名片点赞**：点赞/取消点赞先写 Redis，支持上万日活；每小时批量同步到 MySQL。
4. **发现页与名片详情联动**：卡片显示评论数、点赞数；详情页展示完整评论树和点赞按钮。

## 2. 非目标

- 不做私信。
- 不做评论点赞、评论表情、@ 用户、图片评论。
- 不做评论审核后台；ops-web 一期不新增评论管理页。
- 不做实时推送/WebSocket；评论与点赞通过重新拉取或操作后局部更新。
- 不做评论全文搜索。
- 不把评论和点赞放进 sandbox / DP；两者都属于 CP 业务数据。
- 不在本轮做 10 万日活以上的分库分表。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 评论直接写 MySQL，点赞先写 Redis | 评论是强一致内容，量级可控；点赞是高并发热点，适合写缓冲 |
| D2 | 评论树最大深度 3 层，根评论分页 20，回复分页 10 | 控制 DOM 与查询规模，避免整棵树加载 |
| D3 | 删除叶子评论直接删行；有回复的评论保留占位 | 既减少脏数据，又保证对话上下文不丢失 |
| D4 | 点赞使用 Redis Stream 记录增量事件，小时级消费 | 只处理增量，不扫描大集合；崩溃可重放 |
| D5 | 点赞接口使用 `PUT` / `DELETE` 表达期望状态 | 网络重试不会误反转点赞状态 |
| D6 | MySQL 点赞数按 `COUNT(*)` 重算受影响卡片 | 消费重放时保持幂等，不依赖增量累加 |
| D7 | 点赞读路径以 Redis 为主，MySQL 兜底 | 列表页需要低延迟；Redis 故障时不影响页面基本展示 |
| D8 | 评论公开读、登录写；公开读挂可选会话 | 未登录可以看评论，登录后返回 `editable` / `liked` |

## 4. 总体架构

```
web 发现页 / 名片详情页
        │
        │ /cp/api/v1/public/cards/*
        ▼
CP · CardHandler / CardCommentHandler / CardLikeHandler
        │
        ├── businesscard.CommentService ──> MySQL business_card_comment
        │
        ├── businesscard.LikeService
        │       ├── Redis: count hash + users set + stream
        │       └── asynq @hourly sync ──> MySQL business_card_like
        │
        └── businesscard.CardService ──> business_card / business_card_item
```

评论和点赞都不经过 DP、sandbox。发现页仍通过 CP 的 `/cards/discover` 获取名片列表。

## 5. 评论设计

### 5.1 数据模型

新增 `business_card_comment`：

```sql
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
```

字段约束：

- `parent_id = 0` 表示根评论，`root_id = 0`。
- 回复的 `root_id` 指向根评论 ID。
- `depth` 服务端计算，最大 3；客户端不能传入。
- `content` 为纯文本，去除首尾空白，拒绝空内容和控制字符。
- `status=2` 为删除占位，仅在有回复时保留。

### 5.2 树形加载与大小控制

评论树不做一次性全量返回，采用「根评论分页 + 回复懒加载 + 深度上限」：

| 限制 | 值 | 说明 |
|---|---|---|
| 最大深度 | 3 | 回复第 3 层评论时，新评论 `depth=3`，UI 平铺显示并带 `回复 @某某` |
| 根评论分页 | 20 条/页 | 游标分页，按 `id DESC` |
| 回复分页 | 10 条/页 | 按 `id ASC`，保持对话顺序 |
| 根评论回复预览 | 2 条 | 首屏只带前 2 条回复，避免额外请求 |
| 单卡片最大渲染节点 | 200 | 超过后必须手动「加载更多」 |
| 评论正文长度 | 500 字 | 前端与服务端双重校验 |
| 频率限制 | 5 条/10 秒，30 条/小时 | Redis 计数，超限返回 429 |

前端防卡死措施：

- 不自动展开全部回复。
- 每个根评论默认折叠回复，只显示「查看 N 条回复」。
- 回复分页按需加载，已加载超过 200 个节点后显示「继续加载」。
- 评论节点使用 `content-visibility: auto` 与 CSS containment。
- 长评论折叠，超过 8 行显示「展开」。
- 页面卸载时清理评论请求与 IntersectionObserver。

### 5.3 评论编辑

- 仅作者本人可编辑。
- 一期不限制编辑时间窗。编辑权属于作者，限制时间只会让用户困惑；用「已编辑」标识即可。
- 行内编辑，保存后局部更新，不刷新整棵树。
- 乐观更新失败时回滚，并提示保存失败。
- SQL 条件必须带 `id = ? AND user_id = ? AND status = 1`，防止越权与并发覆盖。
- 编辑不改变排序、层级、回复数。

### 5.4 评论删除

- 当前版本仅作者本人可删除。名片主人删除、ops 删除放二期。
- 叶子评论：直接删除行，`business_card.comment_count - 1`，父节点 `reply_count - 1`。
- 有回复的评论：`status=2`，清空正文，保留节点，显示「该评论已删除」，隐藏编辑/删除/回复按钮。
- 删除根评论后，根评论自身不计入 `comment_count`，已有回复继续展示。
- 计数更新与删除在同一事务，使用 `GREATEST(0, ...)` 防止负数。
- 名片已隐藏或未发布时，评论读写统一返回 404。

删除后的表现：

- 作者与访客刷新后看到一致结果。
- 叶子节点消失，列表自动收缩。
- 有回复的节点变为占位，树结构不塌陷。
- 卡片评论数同步减少。

### 5.5 评论 API

公开读取，写入需要登录：

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| GET | `/cp/api/v1/public/cards/:id/comments?cursor=&limit=20` | 公开，可选会话 | 根评论分页，带前 2 条回复预览 |
| GET | `/cp/api/v1/public/cards/:id/comments/:commentId/replies?cursor=&limit=10` | 公开，可选会话 | 某根评论的回复分页 |
| POST | `/cp/api/v1/public/cards/:id/comments` | 用户登录 | 发表评论或回复 |
| PUT | `/cp/api/v1/public/cards/:id/comments/:commentId` | 用户登录 | 编辑自己的评论 |
| DELETE | `/cp/api/v1/public/cards/:id/comments/:commentId` | 用户登录 | 删除自己的评论，返回 `{tombstoned, commentCount}` |

请求体：

```json
{ "content": "写得很清楚", "parentId": "可选，回复时传" }
```

评论返回结构：

```json
{
  "id": "comment-public-id",
  "cardId": "card-public-id",
  "parentId": "parent-public-id",
  "rootId": "root-public-id",
  "depth": 2,
  "content": "写得很清楚",
  "author": {
    "userId": 1,
    "name": "alice",
    "avatarUrl": "https://..."
  },
  "replyToName": "bob",
  "replyCount": 3,
  "status": "normal",
  "editable": true,
  "deletable": true,
  "editedAt": null,
  "createdAt": "2026-11-16T10:00:00Z"
}
```

分页返回：

```json
{
  "comments": [],
  "nextCursor": "opaque-cursor-or-empty",
  "hasMore": true
}
```

删除返回：

```json
{
  "tombstoned": true,
  "commentCount": 12
}
```

`tombstoned=true` 表示该评论有回复，已替换为「该评论已删除」占位；`false` 表示叶子节点已直接移除。

业务错误：

| HTTP | 业务码 | 场景 |
|---|---|---|
| 400 | 40001 | 内容为空/超长、父评论不属于该名片、深度非法 |
| 401 | 40101 | 未登录写操作 |
| 403 | 40301 | 编辑/删除非本人评论 |
| 404 | 40410 | 名片不存在、已隐藏或评论不存在 |
| 429 | 42901 | 评论频率超限 |

### 5.6 评论前端

- 发现页卡片显示 `commentCount`，但卡片本身不展开评论；必须点击进入名片详情页后才能评论。
- 名片详情页新增评论区：
  - 根评论列表 + 「加载更多评论」。
  - 每个根评论显示作者、时间、正文、「回复」「查看 N 条回复」。
  - 自己的评论显示「编辑」「删除」。
  - 回复按深度缩进，最大 3 层；超过 3 层平铺并显示「回复 @某某」。
- 编辑态为行内 textarea，保存/取消按钮。
- 删除二次确认。
- 未登录点击评论/回复时提示登录。
- 评论发表、编辑、删除后同步更新卡片评论数。

## 6. 点赞设计

### 6.1 方案对比

**方案 A（推荐）：Redis Stream 事件流 + 小时级批量落库**

- 点赞/取消点赞同时写 Redis 计数、用户集合和 Stream 事件。
- 每小时 asynq 消费 Stream，批量写入 MySQL。
- 优点：只处理增量，不扫描大集合；崩溃可重放；适合上万日活。
- 缺点：需要维护消费组、pending 和游标。

**方案 B：Redis dirty set + 全量对账**

- 记录脏卡片 ID，每小时按卡片重新读取用户集合覆盖 MySQL。
- 优点：实现简单。
- 缺点：热门卡片的用户集合可能很大，小时级全量扫描成本高，扩展性差。

一期采用方案 A。

### 6.2 Redis 结构

统一前缀 `vibebiz`：

```text
vibebiz:card:like:count              hash    cardID -> likeCount
vibebiz:card:like:users:{cardID}     set     userID
vibebiz:card:like:stream             stream  {cardId,userId,action,ts}
vibebiz:card:like:sync:lock          string  同步锁
```

点赞 Lua 脚本保证原子性：

- `like`：用户不在集合时 `SADD`，`HINCRBY +1`，`XADD like`。
- `unlike`：用户在集合时 `SREM`，`HINCRBY -1`，`XADD unlike`。
- 计数最小值为 0。
- 同一用户重复点赞不会重复计数。

一期所有点赞事件写入**同一个 Stream**，`cardId` 使用名片内部 bigint ID。1 万日活下单 Stream 足够；后续需要扩容时按 `cardId % N` 写入 `vibebiz:card:like:stream:{shard}`，配置项 `CARD_LIKE_STREAM_SHARDS` 默认 1。同一张名片的事件必须落在同一分片，保证消费顺序。

### 6.3 点赞 API

| 方法 | 路径 | 鉴权 | 说明 |
|---|---|---|---|
| PUT | `/cp/api/v1/public/cards/:id/like` | 用户登录 | 点赞，幂等 |
| DELETE | `/cp/api/v1/public/cards/:id/like` | 用户登录 | 取消点赞，幂等 |

返回：

```json
{ "liked": true, "likeCount": 42 }
```

名片列表与详情返回增加：

```json
{
  "likeCount": 42,
  "liked": true
}
```

### 6.4 每小时同步

- asynq 注册 `@hourly` 任务 `card:like:sync`。asynq 只负责定时触发，不存储点赞事件；事件存储在 Redis Stream 中。
- 任务处理器调用 `LikeSync.Run(ctx)`，由它消费 Stream。小时级任务会把当前已产生的事件分批全部处理完，而不是每个点赞一条 asynq 任务。
- 使用 `asynq.Unique` + Redis 分布式锁，避免多 CP 实例重复执行。
- 消费组 `card-like-sync`：
  - `XREADGROUP COUNT=1000` 读取事件。
  - 事务提交后 `XACK`。
  - 启动时用 `XAUTOCLAIM` 接管超时 pending 消息。
  - 一期使用单消费者，保证同一用户/名片的 like、unlike 按 Stream 顺序处理。
- MySQL 事务：
  - `like`：`INSERT ... ON DUPLICATE KEY UPDATE deleted_at = NULL`。
  - `unlike`：`UPDATE ... SET deleted_at = NOW()`。
  - 对受影响卡片执行：
    ```sql
    UPDATE business_card bc
    SET like_count = (
      SELECT COUNT(*) FROM business_card_like l
      WHERE l.card_id = bc.id AND l.deleted_at IS NULL
    )
    WHERE bc.id IN (...);
    ```
- 提交成功后再 ACK；失败不 ACK，由 asynq 重试。
- Stream 定期 trim，保留最近 7 天；Redis 开启 AOF `appendfsync everysec`。
- 可选夜间对账：Redis 计数与 MySQL 计数不一致时以 MySQL 为准修复。

#### 6.4.1 Stream 事件结构

每条点赞事件是一个 Redis Stream entry，字段固定为字符串：

```text
XADD vibebiz:card:like:stream MAXLEN ~ 1000000 * \
  eventId 8f0c2f1e-... \
  cardId 123 \
  userId 456 \
  action like \
  ts 1763270400000
```

| 字段 | 类型 | 说明 |
|---|---|---|
| `eventId` | string | UUID，用于日志与排查 |
| `cardId` | string/int | 名片内部 bigint ID |
| `userId` | string/int | 用户 ID |
| `action` | string | `like` / `unlike` |
| `ts` | int64 | 服务端毫秒时间戳 |

说明：

- `cardId` 用内部 ID 而不是公开 UUID，落库时直接写外键，避免消费阶段再按公开 ID 查一次。
- 事件按 Redis Stream 顺序处理；`like` / `unlike` 的最终状态由 MySQL 唯一键 + 重算计数保证，重复消费不会重复计数。
- asynq 任务本身不携带点赞明细，只触发 `card:like:sync`；这样不会为每次点赞创建一条 asynq 任务。
- 单 Stream 下用单消费者保证顺序；未来分片后按 `cardId` 路由，同一张名片仍在同一分片。

### 6.5 MySQL 点赞表

```sql
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
```

`business_card` 增加：

```sql
ALTER TABLE `business_card`
  ADD COLUMN `like_count` int UNSIGNED NOT NULL DEFAULT 0 COMMENT '点赞数' AFTER `comment_count`;
```

### 6.6 读路径与降级

- 发现页和名片详情批量读取：
  - `HMGET vibebiz:card:like:count` 取计数。
  - `SISMEMBER vibebiz:card:like:users:{cardID}` 取当前用户是否点赞。
  - 使用 pipeline 批量查询，避免 N+1。
- Redis 计数缺失时回退 MySQL `like_count` 并回填 Redis。
- Redis 不可用时：
  - 读：回退 MySQL `like_count`，`liked` 查 MySQL。
  - 写：返回 503，前端回滚并提示稍后重试。
- 评论不依赖 Redis，Redis 故障不影响评论功能。

### 6.7 前端点赞

- 发现页卡片与名片详情页显示心形按钮和数量。
- 点击后立即乐观更新：心形填充、数量 ±1、按钮短暂禁用。
- 服务端返回最终 `liked` / `likeCount` 后校准。
- 失败时回滚并 toast。
- 未登录点击点赞时提示登录。

### 6.8 容量估算

- 1 万日活，假设人均每天点赞 20 次：约 20 万事件/天，小时级峰值几万条。
- Redis Lua 均为 O(1)，单实例足够。
- MySQL 每小时按 1000 条/批处理，几十批可完成。
- 后续到 10 万日活时再按 `card_id` 分片或按月分区，一期不做。

## 7. 改动范围

### 7.1 backend

| 文件/目录 | 改动 |
|---|---|
| `migrations/00034_business_card_social.sql` | 新增评论表、点赞表，`business_card.like_count` |
| `internal/dao/model/business_card_comment.go` | 评论 model |
| `internal/dao/model/business_card_like.go` | 点赞 model |
| `internal/businesscard/repository.go` 或新增 `comment_repository.go` | 评论查询 |
| `internal/businesscard/comment_service.go` | 评论树、编辑、删除、计数 |
| `internal/businesscard/like_store.go` | Redis 计数、用户集合、Stream、Lua |
| `internal/businesscard/like_service.go` | 点赞/取消点赞、批量状态读取 |
| `internal/businesscard/like_sync.go` | 小时级同步、消费组、幂等落库 |
| `internal/handler/card_comment.go` | 评论 handler |
| `internal/handler/card_like.go` | 点赞 handler |
| `internal/handler/card.go` | `Card` / `DiscoverResult` 增加点赞字段 |
| `internal/router/control.go` | 注册评论与点赞路由 |
| `cmd/cp/main.go` | 装配 LikeStore、LikeService、asynq 小时任务 |

### 7.2 web

| 文件/目录 | 改动 |
|---|---|
| `src/types/index.ts` | `BusinessCard` 增加 `likeCount` / `liked`，新增评论类型 |
| `src/api/cards.ts` | 评论、回复、编辑、删除、点赞 API |
| `src/stores/card.ts` 或新增 `comment.ts` | 评论分页、编辑删除、点赞状态 |
| `src/components/card/CardCommentTree.vue` | 评论树容器 |
| `src/components/card/CardCommentNode.vue` | 单条评论/回复 |
| `src/components/card/CardLikeButton.vue` | 点赞按钮 |
| `src/views/CardShareView.vue` | 接入评论区和点赞 |
| `src/components/card/CardTile.vue` | 显示点赞数、评论数、快速点赞 |
| `src/styles/card.css` | 评论区与点赞样式 |

### 7.3 不改

- sandbox、DP。
- ops-web 一期不新增评论管理。
- 私信不做。

## 8. 测试与观测

### 8.1 测试

- Go：
  - 评论树根分页、回复分页、深度限制。
  - 越权编辑/删除返回 403。
  - 叶子删除 vs 有回复占位删除。
  - 评论计数事务一致性。
  - 点赞幂等（重复 PUT/DELETE）。
  - Stream 消费重放幂等。
  - Redis 故障时点赞读降级。
  - 未登录写操作 401。
- web vitest：
  - 评论节点按深度拍平。
  - 游标追加与去重。
  - 编辑/删除后的本地状态。
  - 点赞乐观更新与失败回滚。
  - 未登录时动作按钮隐藏。
- 迁移测试：
  - 新表、索引、外键。
  - `business_card.like_count` 默认值。

### 8.2 观测

- 指标：
  - 评论创建/删除次数。
  - 点赞/取消点赞次数。
  - 点赞同步耗时、处理事件数。
  - Stream pending 数。
  - Redis 错误数。
- 日志字段：`card_id`、`comment_id`、`user_id`、`action`。
- 禁止记录评论正文、用户敏感信息。
- 同步任务错误写 `failures.log` 风格的结构化日志，便于排查。

## 9. 风险与后续

| 风险 | 处理 |
|---|---|
| 评论树过大导致页面卡顿 | 深度 3、根 20/页、回复 10/页、200 节点上限、懒加载 |
| 热门名片点赞集合过大 | Stream 增量消费，不做全量扫描 |
| Redis 故障导致点赞丢失 | 读降级 MySQL；写返回 503；AOF + 夜间对账 |
| 同步重放导致重复计数 | `COUNT(*)` 重算受影响卡片，唯一键保证单用户单名片一条 |
| 恶意评论/刷屏 | 频率限制、长度限制、纯文本、后续接入审核 |
| 删除评论破坏对话 | 有回复保留占位节点 |

后续可扩展：

- 名片主人删除评论、ops 评论审核。
- 评论点赞。
- 评论通知。
- 按热度排序根评论。
- 10 万日活后的分片与归档。

## 10. 已确认决策

1. 评论入口：必须点击进入名片详情页后才能评论；发现页卡片不展开评论，也不做右侧 Drawer。
2. 删除权限：当前版本只允许作者本人删除；名片主人删除、ops 删除放二期。
3. 编辑时间窗：不限制编辑时间，只显示「已编辑」。
4. 点赞同步：采用 Redis Stream；asynq 每小时触发批量消费。一期单 Stream、单消费者，后续按 `cardId` 分片。

## 11. 实现默认值

| 配置项 | 默认值 | 说明 |
|---|---|---|
| `CARD_LIKE_STREAM_SHARDS` | `1` | 一期单 Stream，后续可扩分片 |
| `CARD_LIKE_STREAM_MAXLEN` | `1000000` | Stream 近似长度上限 |
| `CARD_LIKE_SYNC_BATCH` | `1000` | 每批消费事件数 |
| `CARD_LIKE_SYNC_INTERVAL` | `@hourly` | asynq 定时表达式 |
| `CARD_LIKE_STREAM_TTL` | `7d` | Stream 保留时间 |
| 评论根分页 | `20` | 最大 50 |
| 回复分页 | `10` | 最大 50 |
| 评论最大深度 | `3` | 服务端强制 |
| 评论正文长度 | `500` | 前后端一致 |

# Ops 消息通知与 Web 实时提醒 · 设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-29 |
| 状态 | 待评审 |
| 范围 | CP 通知模块、MySQL 表与迁移、Redis 发布缓存、WebSocket 广播、web 首页铃铛弹窗、ops-web 消息管理、帮助入口 |
| 关联文档 | `docs/AI-Coding-Platform-技术设计文档.md`、`docs/superpowers/specs/2026-10-28-business-database-design.md`、`docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`、`.ai/rules/architecture.md` |
| 数据库 | MySQL 8；新增 `ops_notification` 表与 `00008` 迁移 |
| 依赖 | 后端新增 `github.com/gorilla/websocket`；web 新增 `marked`、`dompurify` |

---

## 1. 背景与目标

web 首页左侧栏底部已有“帮助”和“通知”两个占位按钮，当前没有行为。本轮要完成：

1. 帮助按钮直接跳转官方文档站 `https://help.atoms.dev/zh`。
2. 新增 ops 消息通知表，运营可在 ops-web 创建、编辑、发布、取消发布、删除消息。
3. 消息分两类：版本发布（`version_release`）与收集箱（`inbox`），两者都由运营创建、面向全体用户群发。
4. web 点击铃铛打开弹窗，按类型分 Tab 展示已发布消息；登录后从 CP 拉取 Redis 中的发布缓存。
5. ops 发布/取消发布/删除时，CP 通过 WebSocket 通知已登录 web 端静默刷新列表。
6. 已发布消息在 Redis 中按消息过期时间自动消失，最长停留 30 天。

## 2. 非目标

- 不做未读角标、已读状态、消息回执或按用户/工作区定向。
- 不做用户提交反馈；收集箱与版本发布一样是运营群发通知。
- 不做消息多语言、定时发布、草稿协作或审批流。
- 不做多 CP 实例 WebSocket 广播；本轮以单 CP 实例为前提，后续可替换为 Redis Pub/Sub。
- 不改动 DP、sandbox 的文件/SSE/预览契约。
- 不引入统一站内信中心或独立消息服务。

## 3. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | MySQL `ops_notification` 是唯一事实源，Redis 只缓存“已发布”消息 | 运营需要草稿、历史与过期后的管理能力；Redis 只服务 web 读取 |
| D2 | Redis 用 ZSET 索引 + 每消息独立 key，`EXPIREAT` 使用消息过期时间 | 每条消息停留时间不同；单 key TTL 无法表达；过期自动清理 |
| D3 | WebSocket 只广播“变更事件”，正文仍由 HTTP 拉取 | 事件轻量，重连/漏事件时靠 HTTP 全量刷新兜底 |
| D4 | WS 事件到达后静默刷新，不弹窗、不打断 | 用户确认的交互；弹窗打开时列表自动更新 |
| D5 | 已发布消息禁止编辑，必须先取消发布 | 避免 DB 与 Redis 双写不一致 |
| D6 | 过期时间在创建/编辑时选绝对日期，范围 `(now, now+30d]` | 用户确认按日期选择；后端统一校验上限 |
| D7 | 消息全体广播，不建用户消息关系表 | 当前需求是全员公告 |
| D8 | WebSocket 使用 Cookie 鉴权，复用 `RequireUser` | 与现有 web 登录态一致，不引入额外 token |
| D9 | 单 CP 实例进程内 Hub 广播 | 当前部署单实例；保持实现最小 |
| D10 | 帮助按钮使用普通外链，新窗口打开 | 静态外链，无需后端 |

## 4. 目标架构

```
ops-web (:5174)
  └─ /cp/api/v1/admin/notifications/*  (RequireOps)
        │
        ▼
CP (:8080) ── internal/notification.Service
  │   ├─ MySQL: ops_notification（事实源）
  │   ├─ Redis: index ZSET + item JSON，EXPIREAT expireAt
  │   └─ Hub: 进程内 WebSocket 客户端集合
  │
  └─ /cp/api/v1/notifications      (RequireUser, 读 Redis)
  └─ /cp/api/v1/notifications/ws   (RequireUser, WebSocket)

web (:5173)
  ├─ 登录后 GET /notifications
  ├─ WS 收到 notifications.changed 后重新 GET /notifications
  └─ 铃铛 Dialog 按类型 Tab 展示
```

发布链路：

1. ops-web 调用 `POST /admin/notifications/:id/publish`。
2. CP 校验状态与过期时间。
3. CP 写入 Redis（item key + index），再更新 MySQL 状态为 `published`。
4. 两步都成功后，Hub 广播 `notifications.changed`。
5. web 收到事件，重新拉取 `/notifications`。

取消发布/删除链路：更新 MySQL → 删除 Redis item 与 index 成员 → 广播变更事件。

## 5. 数据模型

### 5.1 `ops_notification`

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint unsigned PK | 主键 |
| `title` | varchar(200) not null | 标题 |
| `content` | mediumtext not null | Markdown 正文 |
| `type` | varchar(32) not null | `version_release` / `inbox` |
| `version` | varchar(64) not null default '' | 版本号，仅版本发布使用 |
| `link` | varchar(1024) not null default '' | 可选跳转链接，仅允许 http/https |
| `status` | varchar(24) not null default 'draft' | `draft` / `published` / `unpublished` |
| `expire_at` | datetime not null | 消息停留截止时间 |
| `published_at` | datetime null | 最近一次发布时间 |
| `created_by` | bigint unsigned not null | 创建人 `ops_user.id` |
| `created_by_name` | varchar(64) not null | 创建人用户名快照 |
| `created_at` | timestamp not null | 创建时间 |
| `updated_at` | timestamp not null | 更新时间 |
| `deleted_at` | datetime null | GORM 软删除 |
| `extra_p` | json null | 扩展字段 |

索引：

- `idx_ops_notification_status (status, deleted_at)`
- `idx_ops_notification_type (type, status, deleted_at)`
- `idx_ops_notification_expire (expire_at)`
- `idx_ops_notification_published (published_at)`

### 5.2 状态与过期

- 持久化状态只有 `draft`、`published`、`unpublished`。
- API 额外派生展示状态 `expired`：`status=published && expire_at <= now`。
- 过期消息不自动改 DB 状态；Redis item 已按 TTL 消失，web 不再返回。
- 过期消息可取消发布或删除；如需重新发布，必须编辑新的 `expire_at` 并先回到 `draft`/`unpublished`。
- 发布时若 `expire_at <= now`，返回 `409`。

### 5.3 字段校验

- `title`：trim 后 1–200 字符。
- `content`：1–512 KB。
- `type`：仅 `version_release`、`inbox`。
- `version`：≤64 字符；`type=inbox` 时忽略并存储空串。
- `link`：空或 `http://` / `https://` 开头，≤1024 字符。
- `expire_at`：必须晚于当前时间，且不晚于当前时间 + 30 天。
- 创建人从 `auth.SessionFrom(c)` 取 `UserID` 与 `Username`，不信任客户端。

## 6. Redis 设计

Key 前缀沿用 CP 的 `vibebiz`：

- 索引：`vibebiz:notification:index`，ZSET，member 为消息 ID，score 为 `publishedAt` 的 Unix 毫秒。
- 消息：`vibebiz:notification:item:{id}`，STRING，JSON 值：

```json
{
  "id": 12,
  "title": "v1.2.0 发布",
  "content": "## 更新内容\n- ...",
  "type": "version_release",
  "version": "v1.2.0",
  "link": "https://help.atoms.dev/zh",
  "publishedAt": "2026-10-29T08:00:00Z",
  "expireAt": "2026-11-05T08:00:00Z"
}
```

写入：

1. `SET item JSON`，`EXPIREAT expireAt.Unix()`。
2. `ZADD index publishedAt.UnixMilli() id`。

删除：`DEL item`，`ZREM index id`。

读取（每次请求都做读时对账）：

1. 查询 MySQL `status=published AND expire_at > now`，得到事实源列表。
2. `ZREVRANGE index 0 -1` 得到 ID 列表，再 `MGET item:{id}`；跳过 nil（已过期）与非法 JSON，并把对应 ID 从 ZSET 惰性 `ZREM`。
3. 若索引 key 不存在（`ErrIndexMissing`），用第 1 步的 DB 结果 `Rebuild` 缓存。该回填覆盖 Redis 被清空/首次部署场景。
4. 若缓存存在，但其 ID 集合、`publishedAt` 或 `expireAt` 与 DB 结果不一致，用 DB 结果 `Rebuild` 缓存。这样能清除取消发布/删除时 Redis 清理失败留下的残留 item/index。
5. 重建成功后直接返回由 DB 行派生的列表，不依赖第二次缓存读取；重建失败返回 `503`。
6. 缓存读取出现非 `ErrIndexMissing` 错误时返回 `503`。

失败处理：

- 发布时 Redis 写入失败：不更新 DB，返回 `503`。
- Redis 已写入但 DB 更新失败：删除刚写入的 Redis item 与 index 成员；清理失败时记录 `notification_id`、`action` 与错误，返回 `500`。
- 取消发布/删除时始终删除 Redis item 与 index 成员（不区分此前状态），不依赖“是否已发布”判断。Redis 失败时记录 `notification_id`、`action` 与错误并返回 `503`；DB 状态已更新，下一次 web 读取会通过读时对账清除残留。客户端可重试。

## 7. 后端接口

统一响应 `{ code, message, result }`，成功 `code=0`。

### 7.1 ops-web（`RequireOps`）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/admin/notifications` | 列表，支持 `type`、`status` 查询参数；默认按 `created_at DESC` |
| POST | `/cp/api/v1/admin/notifications` | 创建草稿 |
| PUT | `/cp/api/v1/admin/notifications/:id` | 编辑，仅 `draft` / `unpublished` |
| POST | `/cp/api/v1/admin/notifications/:id/publish` | 发布 |
| POST | `/cp/api/v1/admin/notifications/:id/unpublish` | 取消发布 |
| DELETE | `/cp/api/v1/admin/notifications/:id` | 软删除；已发布先清 Redis |

返回对象：

```json
{
  "id": 12,
  "title": "v1.2.0 发布",
  "content": "## 更新内容",
  "type": "version_release",
  "version": "v1.2.0",
  "link": "https://help.atoms.dev/zh",
  "status": "published",
  "expireAt": "2026-11-05T08:00:00Z",
  "publishedAt": "2026-10-29T08:00:00Z",
  "createdBy": 1,
  "createdByName": "admin",
  "createdAt": "2026-10-29T07:50:00Z",
  "updatedAt": "2026-10-29T08:00:00Z"
}
```

错误码：

- `40001` 参数校验失败。
- `40401` 消息不存在。
- `40901` 状态不允许（已发布编辑、过期发布、重复发布等）。
- `50301` Redis 不可用。

### 7.2 web（`RequireUser`）

`GET /cp/api/v1/notifications`

```json
{
  "notifications": [
    {
      "id": 12,
      "title": "v1.2.0 发布",
      "content": "## 更新内容",
      "type": "version_release",
      "version": "v1.2.0",
      "link": "https://help.atoms.dev/zh",
      "publishedAt": "2026-10-29T08:00:00Z",
      "expireAt": "2026-11-05T08:00:00Z"
    }
  ]
}
```

只返回 Redis 中当前有效的已发布消息，不返回草稿与已取消发布消息。

### 7.3 WebSocket

`GET /cp/api/v1/notifications/ws`

- 升级前经过 `RequireUser`。
- 校验 `Origin` 在 `CORS_ORIGINS` 允许列表内；与 Host 同源也允许。
- 升级成功后服务端只推送变更事件：

```json
{ "type": "notifications.changed", "action": "published", "id": 12 }
```

- `action` 取值：`published`、`unpublished`、`deleted`。
- 心跳：服务端每 30 秒 ping，读超时 60 秒；写超时 10 秒。
- 慢客户端发送缓冲超过 16 条时断开，由前端重连并全量拉取。
- 客户端断线后指数退避重连（1s、2s、4s，上限 30s）。

## 8. 后端代码结构

| 路径 | 职责 |
|---|---|
| `internal/dao/model/notification.go` | GORM 模型 `OpsNotification` |
| `internal/dao/notification_repository.go` | 列表、详情、创建、更新、状态变更、软删除、查询有效已发布消息 |
| `internal/notification/cache.go` | Redis 读写、索引回填、TTL |
| `internal/notification/hub.go` | WebSocket 客户端注册、广播、心跳、关闭 |
| `internal/notification/service.go` | 状态机、校验、发布/取消发布/删除编排 |
| `internal/handler/notification.go` | ops 与 web HTTP handler、WS 升级 |
| `internal/router/control.go` | 注册 admin 与 web 通知路由 |
| `cmd/cp/main.go` | 装配 repository、cache、service、hub、handler |

复用现有 `dao.OpenMySQL`、`auth.NewRedisClient`、`auth.RequireUser`、`auth.RequireOps`、`response.OK/Fail`。WS 关闭时由 `cmd/cp` 的 shutdown 流程调用 `hub.Close()`。

## 9. 前端设计

### 9.1 web 首页

- `HomeSidebar.vue`
  - 帮助按钮改为 `<a href="https://help.atoms.dev/zh" target="_blank" rel="noopener noreferrer">`。
  - 铃铛按钮打开 `NotificationsDialog.vue`。
- `NotificationsDialog.vue`
  - PrimeVue `Dialog` + `Tabs`，两个 Tab：版本发布、收集箱。
  - 打开时调用 `listNotifications()`；已有数据时先展示缓存再刷新。
  - 每条显示标题、发布时间、版本号（版本发布）、Markdown 正文、“查看详情”外链。
  - Markdown 通过 `marked` 转 HTML，再经 `DOMPurify.sanitize` 后 `v-html` 渲染。
  - 空状态：暂无消息。
  - 收到 WS 变更事件时若弹窗打开则刷新；弹窗关闭时也刷新 store，保证下次打开最新。
- 新增 `src/api/notifications.ts`、`src/stores/notifications.ts`、`src/composables/useNotificationSocket.ts`。
- `buildNotificationSocketUrl(base, location)` 为纯函数，支持相对 base（`/cp/api/v1`）与绝对 base。
- 登录成功后启动 WS；登出/401 时关闭 WS 并清空列表。

### 9.2 ops-web

- 导航新增“消息通知”，路由 `/notifications`，图标 `pi pi-bell`。
- `NotificationsView.vue`
  - 表格列：标题、类型、状态、版本号、过期时间、创建人、创建时间、操作。
  - 新建/编辑弹窗字段：标题、类型、版本号（仅 `version_release` 显示）、链接、Markdown 正文、过期日期。
  - 操作：编辑（仅草稿/已取消发布）、发布、取消发布、删除（二次确认）。
  - 过期派生状态显示“已过期”；发布按钮对过期消息禁用。
- 新增 `src/api/notifications.ts`，类型加入 `src/types/index.ts`。
- 日期选择使用 PrimeVue `DatePicker`，提交 ISO 字符串。

### 9.3 代理与部署

- `packages/web/vite.config.ts` 的 `/cp` 代理增加 `ws: true`。
- `packages/web/nginx.conf` 为 `/cp/api/v1/notifications/ws` 增加：

```nginx
proxy_http_version 1.1;
proxy_set_header Upgrade $http_upgrade;
proxy_set_header Connection "upgrade";
proxy_read_timeout 3600s;
```

- ops-web 不建立 WS 连接，无需改代理。

## 10. 迁移与配置

新增 `packages/backend/migrations/00008_ops_notification.sql`，包含 `-- +goose Up` 建表与 `-- +goose Down` 删表。

无需新增环境变量：

- MySQL、Redis、CORS 均复用现有配置。
- 30 天上限、心跳间隔、客户端缓冲上限作为包内常量。

## 11. 测试

### 11.1 后端

- DAO：sqlmock 覆盖创建、更新、列表过滤、软删除、查询有效已发布；`Update` / `UpdateStatus` 在 `RowsAffected=0` 时通过 `Get` 区分“行不存在”与“无字段变化”。
- Cache：miniredis 覆盖写入后 TTL、ZSET 顺序、过期后读取过滤（同时包含一条存活与一条过期成员）、非法 JSON 清理、已过期 item 不写入、取消发布删除、索引缺失回填。
- Service：状态机（草稿发布、重复发布、已发布编辑、过期发布、取消发布、删除已发布）、校验边界（30 天、标题/内容/版本长度、链接协议与 host、类型、inbox 清空版本）；读时对账覆盖“缓存有残留但 DB 没有”“DB 有但缓存缺失”“缓存读取失败”三类场景；取消发布/删除的缓存清理失败返回 503。
- Handler：httptest 覆盖 ops 权限、web 权限、参数错误、状态冲突、Redis 503；WebSocket `CheckOrigin` 覆盖配置来源、同 Host、空 Origin 与拒绝场景，未登录升级返回 401，升级失败只记录日志不二次写响应。
- Hub：httptest + gorilla dialer 覆盖连接、广播、断开；慢客户端发送缓冲占满后被移除用单元测试。
- Router：CP 注册通知路由，DP 不注册；管理员 POST 通知缺少请求体返回 400，WS 路由未登录返回 401。

### 11.2 web

- `notifications` API 解包。
- Tab 过滤与空状态。
- Markdown 渲染清洗：`<script>` 被移除。
- `buildNotificationSocketUrl` 相对/绝对 base。
- WS 重连退避纯函数。

### 11.3 ops-web

- 通知 API 请求方法与路径。
- 状态/过期展示映射。
- 导航包含“消息通知”且路由可达。

### 11.4 验证

- `make validate`
- 手工冒烟：
  1. ops 创建草稿 → 编辑 → 发布。
  2. web 打开铃铛看到消息；保持弹窗打开时 ops 再发一条，列表静默新增。
  3. ops 取消发布 → web 列表消失。
  4. ops 删除 → DB 软删除、Redis 清理。
  5. Redis 中 item TTL 等于 `expire_at`。
  6. 帮助按钮新窗口打开 `https://help.atoms.dev/zh`。

## 12. 风险与后续

- 单实例 Hub 无法跨 CP 实例广播；多实例时改用 Redis Pub/Sub 或消息队列。
- WS 依赖反向代理正确透传 `Upgrade`；已在本轮修改 Vite 与 nginx。
- Markdown 渲染必须经过 DOMPurify，禁止直接 `v-html` 原始内容。
- 过期消息不自动清理 DB 行；如需归档策略，后续增加定时任务或运营筛选。

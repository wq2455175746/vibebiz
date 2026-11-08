# 发布作品排行榜与发布数据统计设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-07 |
| 状态 | 设计稿 |
| 范围 | CP / web / ops-web；Redis 排行榜、发布页访问埋点、用户发布统计 |
| 关联 | `.ai/rules/architecture.md`、`docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`、`docs/superpowers/specs/2026-11-06-user-usage-metrics-design.md` |

---

## 1. 背景与目标

当前系统已经支持把项目发布为静态分享页（`/p/:slug`），但缺少发布作品的热度排行和发布者数据反馈。用户无法看到哪些作品受欢迎，也无法在账户设置里查看自己发布作品的曝光与点击。

本期目标：

1. 只对已发布作品统计访问量（PV），使用 Redis ZSET 保存，榜单始终只保留累计 PV 最高的 50 条。
2. web 左侧菜单新增「排行榜」，以卡片网格展示名次、项目名、作者、访问数，每行 2 个卡片。
3. ops-web 新增「排行榜」页，用表格展示榜单，并支持隐藏 / 取消隐藏榜单条目、删除榜单数据。
4. web 账户设置「我发布的项目」区域新增数据统计：曝光量、点击数、停留时长、消费额度。本期只实现曝光量与点击数，停留时长与消费额度固定为 0。

非目标：

- 不做按天 / 按周滚动榜，本期为累计总榜。
- 不做作品详情页、评论、点赞、收藏。
- 不做真实停留时长采集与计费流水；相关字段只占位。
- 不做排行榜分页之外的历史归档；Redis PV 候选集最多保留 200 条，对外榜单只展示前 50。
- 不修改数据库 schema，不新增迁移。
- ops-web 的隐藏 / 删除只作用于排行榜条目，不影响已发布应用 `/p/:slug` 的访问。

## 2. 统计口径

| 指标 | 口径 | 触发点 |
|---|---|---|
| PV / 访问数 | 访问发布分享页根文档，每次 +1 | `GET /p/:slug` 或 `GET /p/:slug/` 或 `GET /p/:slug/index.html`；`game/*` 等静态资源不计 |
| 曝光量 | 用户端排行榜接口返回该项目卡片，每次 +1 | `GET /cp/api/v1/leaderboard` 返回的可见条目 |
| 点击数 | 分享页点击「开始体验」或带 `?play=1` 自动播放，每次 +1 | 分享页 `sendBeacon` 上报 `POST /p/:slug/track/click` |
| 停留时长 | 本期固定 0 | 不采集 |
| 消费额度 | 本期固定 0 | 不采集 |

说明：

- ops-web 查看排行榜不增加曝光量。
- 被 ops 隐藏的榜单条目仍继续统计 PV / 点击；它不会出现在用户榜，因此不再增加曝光量。
- 项目全局隐藏（`project.is_hidden=1`）时只从榜单过滤，不清理 PV 或 ops 隐藏标记；解除全局隐藏后原榜单数据仍可恢复。
- 项目取消发布或因项目删除而不再存在时，在读取榜单时惰性清理排名与元数据，保留曝光 / 点击计数供重新发布后继续累计。
- 用户主动删除发布（`DELETE /projects/:id/publish`）时，按产品语义清除该项目的全部榜单数据（PV / 曝光 / 点击 / 元数据 / 隐藏标记）。
- ops 删除榜单数据后，已发布应用不受影响；后续再被访问会重新进入榜单。

## 3. 总体架构

```
web 排行榜页 ── GET /cp/api/v1/leaderboard ──────────────┐
                                                          ▼
ops-web 排行榜页 ── /cp/api/v1/admin/leaderboard ──> CP LeaderboardHandler
                                                          │
                                                          ▼
                                              internal/leaderboard.Service
                                                   │            │
                                                   ▼            ▼
                                       Redis（ZSET/HASH/SET）  publish.Service
                                                                store.ProjectStore
                                                                identity.Repository

分享页 /p/:slug ── Preview ──> RecordView (PV)
分享页「开始体验」── POST /p/:slug/track/click ──> RecordClick
用户设置页 ── GET /me/published-projects/stats ──> Stats
```

依赖方向不变：

- `web → CP`
- `ops-web → CP admin`
- CP 内部使用 Redis 与 MySQL，不新增外部服务。
- sandbox / DP 不参与本功能。

## 4. Redis 数据结构

统一使用现有 `vibebiz` 前缀。

| Key | 类型 | 说明 |
|---|---|---|
| `vibebiz:leaderboard:pv` | ZSET | member=项目公开 ID，score=累计 PV；候选集最多 200 条，对外榜单取前 50 |
| `vibebiz:leaderboard:hidden` | SET | 被 ops 隐藏的榜单项目 ID |
| `vibebiz:leaderboard:exposure` | HASH | field=项目 ID，value=累计曝光量 |
| `vibebiz:leaderboard:clicks` | HASH | field=项目 ID，value=累计点击数 |
| `vibebiz:leaderboard:meta` | HASH | field=项目 ID，value=JSON `{title, author}` |

写入规则：

- PV：`ZINCRBY vibebiz:leaderboard:pv 1 <projectId>`，随后 `ZREMRANGEBYRANK vibebiz:leaderboard:pv 0 -201` 将候选集裁剪到最多 200 条。对外排行榜只读取前 50，保留候选缓冲是为了让新项目在低 PV 阶段积累访问后仍有机会上榜。
- 曝光：`HINCRBY vibebiz:leaderboard:exposure <projectId> 1`。
- 点击：`HINCRBY vibebiz:leaderboard:clicks <projectId> 1`。
- 隐藏：`SADD` / `SREM vibebiz:leaderboard:hidden <projectId>`。
- 元数据：发布成功、读取榜单发现缺失时 `HSET vibebiz:leaderboard:meta <projectId> <json>`。
- 删除榜单数据：`ZREM pv`、`HDEL exposure clicks meta`、`SREM hidden`。
- 惰性清理未发布项目：`ZREM pv`、`HDEL meta`、`SREM hidden`，保留 exposure / clicks。

所有计数操作失败只记录日志，不阻断发布页访问、榜单读取或设置页加载。

## 5. Backend 设计

### 5.1 新增 `internal/leaderboard`

```go
type Entry struct {
    ProjectID string
    Title     string
    Author    string
    Visits    int64
    Hidden    bool
}

type Counters struct {
    Exposure int64
    Clicks   int64
}

type Store struct {
    client *redis.Client
    prefix string
    now    func() time.Time
}
```

`Store` 提供：

- `RecordView(ctx, projectID)`：PV +1，并把候选集裁剪到最多 200 条；对外榜单读取时取前 50。
- `RecordClick(ctx, projectID)`：点击数 +1。
- `RecordExposure(ctx, projectIDs)`：批量曝光 +1。
- `Top(ctx, limit)`：按分数倒序返回最多 `limit` 条原始条目及分数。
- `Counters(ctx, projectIDs)`：批量读取曝光与点击。
- `Meta(ctx, projectIDs)` / `PutMeta(ctx, projectID, Meta)`。
- `Hide(ctx, projectID, hidden)`。
- `Delete(ctx, projectID)`。
- `PruneRanking(ctx, projectIDs)`：只清理排名与元数据，保留计数。

`Service` 组合 Redis Store、`publish.Service`、`store.ProjectStore`、用户名解析器和 `PublicBaseURL`，提供：

- `ListPublic(ctx, limit)`：用户榜；过滤未发布、全局隐藏、榜单隐藏；补齐缺失元数据；返回后累加曝光。
- `ListAdmin(ctx, query)`：ops 表格；过滤未发布；支持关键词、分页；不累加曝光。
- `Stats(ctx, projectIDs)`：返回指定项目的曝光、点击、停留 0、消费 0。
- `UpsertMeta(ctx, project, title)`：发布成功时写入元数据。
- `Hide(ctx, projectID, hidden)` / `Delete(ctx, projectID)`。
- `RecordView(ctx, projectID)` / `RecordClick(ctx, projectID)`：失败只记日志。

作者解析规则：

- 项目作者使用 `project.OwnerUserID` 对应的 `user.username`。
- 批量接口 `IdentityRepository.Usernames(ctx, ids)` 返回 `map[int64]string`。
- 作者解析失败时回退为空字符串，不让榜单请求失败。

预览图解析规则：

- 优先使用 `project.CoverS3Path` 经 S3 预签名得到 `coverUrl`，每次请求重新签名。
- 没有 S3 封面时回退 `project.CoverURL`；都没有时返回空字符串，由前端显示占位。

元数据补齐：

1. 从 ZSET 取最多 200 条候选。
2. 用 `publish.Service.List` 得到当前已发布集合。
3. 用 `store.ProjectStore.List` 得到项目投影，过滤不存在、`IsHidden` 的项目。
4. 对仍未发布或已删除的项目执行 `PruneRanking`；全局隐藏只跳过，不清理 Redis。
5. 对缺失 meta 的项目，用发布标题、项目 owner 用户名补齐并写回 Redis；用户名解析失败时不写回，留待下次重试。
6. 过滤 `hidden` 集合，按排名返回最多 50 条。这样隐藏或失效条目不会挤占对外榜单名额。

### 5.2 API 契约

#### 用户排行榜

`GET /cp/api/v1/leaderboard?limit=50`

- 登录用户 Cookie + Redis Session。
- `limit` 默认 50，最大 50。

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "items": [
      {
        "rank": 1,
        "projectId": "p1",
        "name": "小游戏",
        "author": "wanghg11",
        "visits": 123,
        "coverUrl": "https://cdn.example.com/covers/p1.png",
        "url": "https://vibebiz-dev.com/p/p1/"
      }
    ]
  }
}
```

#### ops 排行榜

`GET /cp/api/v1/admin/leaderboard?keyword=&page=1&pageSize=20`

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "items": [
      {
        "rank": 1,
        "projectId": "p1",
        "name": "小游戏",
        "author": "wanghg11",
        "visits": 123,
        "exposure": 456,
        "clicks": 78,
        "hidden": false,
        "publishedAt": "2026-11-07T10:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20
  }
}
```

#### 隐藏 / 取消隐藏

`PUT /cp/api/v1/admin/leaderboard/:projectId/hidden`

```json
{ "hidden": true }
```

返回：

```json
{ "code": 0, "message": "ok", "result": { "projectId": "p1", "hidden": true } }
```

#### 删除榜单数据

`DELETE /cp/api/v1/admin/leaderboard/:projectId`

返回：

```json
{ "code": 0, "message": "ok", "result": { "projectId": "p1" } }
```

#### 用户发布统计

`GET /cp/api/v1/me/published-projects/stats`

统计范围为当前用户所在工作区内、当前已发布且未全局隐藏的项目，与 `GET /me/published-projects` 保持一致。

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "exposure": 456,
    "clicks": 78,
    "dwellSeconds": 0,
    "spend": 0,
    "projects": [
      {
        "projectId": "p1",
        "name": "小游戏",
        "exposure": 456,
        "clicks": 78,
        "dwellSeconds": 0,
        "spend": 0
      }
    ]
  }
}
```

#### 点击上报

`POST /p/:slug/track/click`

- 公开，无 Cookie 要求。
- 校验 slug 合法且已发布；成功返回 `204 No Content`。
- 只累加点击数，不累加 PV。

### 5.3 发布链路改动

`internal/handler/publish.go`：

- `Preview`：
  - 在成功解析文件路径后，仅当请求路径为空、`/` 或 `index.html` 时调用 `RecordView`。
  - 保持现有 404、隐藏项目、CSP、缓存头行为不变。
- `Publish`：
  - 发布成功后调用 `UpsertMeta`，写入标题与作者。
- `DeletePublished`：
  - 删除发布成功后调用 `Delete` 清理该项目的榜单数据。
- 新增 `TrackClick` handler。
- 所有 `Leaderboard` 依赖允许为 `nil`，便于现有测试与无 Redis 场景降级。

`internal/router/control.go`：

- `api.GET("/leaderboard", leaderboardHandler.ListPublic, authn.RequireUser)`
- `meAPI.GET("/published-projects/stats", publishApp.PublishedStats)`
- `e.POST("/p/:slug/track/click", publishApp.TrackClick)`
- `adminProtected.GET("/leaderboard", leaderboardHandler.ListAdmin)`
- `adminProtected.PUT("/leaderboard/:projectId/hidden", leaderboardHandler.Hide)`
- `adminProtected.DELETE("/leaderboard/:projectId", leaderboardHandler.Delete)`

`cmd/cp/main.go`：

- 创建 `leaderboard.NewStore(redisClient, redisPrefix)` 与 `leaderboard.NewService(...)`。
- 注入 `publishService`、`projectStore`、`identity` 用户解析器、`cfg.PublicBaseURL`、`obs.Logger`。
- 将 service 传给 `PublishHandler` 与 `NewLeaderboardHandler`。

### 5.4 分享页上报

`internal/publish/share.html`：

- 在 `play()` 中通过 `navigator.sendBeacon` 或 `fetch(..., {keepalive:true})` 上报相对地址 `track/click`。
- 同一页面会话去重，避免重复点击重复计数。
- 保持现有「开始体验」、`?play=1`、全屏与退出逻辑不变。
- `RefreshSharePages` 会在 CP 启动时重新渲染分享页，已有发布作品自动获得上报脚本。

## 6. Web 设计

### 6.1 左侧菜单与路由

- `HomeSidebar.vue` 导航新增：`{ label: '排行榜', icon: 'pi pi-trophy', route: 'leaderboard' }`。
- `router/index.ts` 新增 `/leaderboard`，`name: 'leaderboard'`，复用 `HomeView`。
- `HomeView.vue` 新增 `isLeaderboardPage` 分支，渲染 `LeaderboardPanel`。
- 文档标题增加「排行榜」。

### 6.2 排行榜页

新增 `components/home/LeaderboardPanel.vue`：

- 顶部标题「发布作品排行榜」与说明。
- 加载 / 错误 / 空态。
- 卡片网格：桌面每行 2 个，窄屏 1 个。
- 每张卡片展示：
  - 项目预览图（项目 `coverS3Path` 的预签名 URL，回退 `coverUrl`；都没有时显示占位）。
  - 名次徽标（1-3 名使用蓝色强调），叠加在封面左上角。
  - 项目名。
  - 作者。
  - 访问数。
- 点击卡片新窗口打开发布地址，`noopener,noreferrer`。
- 预览图 URL 每次请求重新签名，避免过期；加载失败时回退占位图。

### 6.3 账户设置统计

`views/settings/ProfileView.vue` 在「我发布的项目」上方新增：

- 4 个汇总指标卡片：曝光量、点击数、停留时长、消费额度。
- 每个已发布项目的明细表：项目名、曝光量、点击数、停留时长、消费额度。
- 停留时长显示 `0 秒`，消费额度显示 `0`。
- 加载与错误状态独立，失败不影响现有发布项目列表。

新增 API：

- `src/api/leaderboard.ts`：`getLeaderboard(limit?)`。
- `src/api/publish.ts`：`getPublishedStats()`。
- `src/types/index.ts`：`LeaderboardEntry`、`PublishedStats`、`PublishedProjectStats`。

## 7. ops-web 设计

- `router/navigation.ts` 新增：`{ name: 'leaderboard', label: '排行榜', path: '/leaderboard', icon: 'pi pi-trophy' }`。
- `router/routes.ts` 新增 `/leaderboard` → `LeaderboardView.vue`。
- 新增 `views/LeaderboardView.vue`：
  - 复用 `DataTable`、`TableSearch`。
  - 列：排名、项目名、作者、访问数、曝光量、点击数、状态、操作。
  - 状态列显示「正常 / 已隐藏」。
  - 操作列：隐藏 / 取消隐藏、删除榜单数据。
  - 删除前 `window.confirm` 二次确认。
  - 操作成功后重新加载列表并显示提示。
- `api/admin.ts` 新增：
  - `getLeaderboard(params)`
  - `updateLeaderboardHidden(projectId, hidden)`
  - `deleteLeaderboardEntry(projectId)`
- `types` 增加 `AdminLeaderboardRow` 等类型。
- ops-web 仍只访问 `/cp/api/v1/admin/*`，不访问 DP / sandbox。

## 8. 边界与异常

| 场景 | 行为 |
|---|---|
| Redis 计数失败 | 记录日志，发布页 / 榜单 / 设置页仍可用 |
| 榜单读取失败 | 返回 500 信封，前端展示错误态 |
| 项目未发布或已删除 | 从榜单过滤，并惰性清理排名与 meta |
| 项目全局隐藏 | 从榜单过滤 |
| 榜单条目被 ops 隐藏 | 用户榜不展示；ops 榜展示「已隐藏」 |
| ops 删除后再次访问 | 重新进入榜单，PV / 曝光 / 点击从 0 开始 |
| 分享页连续点击 | 同一页面会话只上报一次点击 |
| 分享页静态资源请求 | 不计 PV |
| 作者查询失败 | 作者为空字符串，不阻断榜单 |
| `limit > 50` | 按 50 处理 |
| 非法的 `projectId` / `slug` | 400 / 404，不写 Redis |

## 9. 测试

### Go

- `internal/leaderboard/store_test.go`（miniredis）：
  - PV 累加、排序、裁剪到 50。
  - 曝光 / 点击批量累加。
  - 隐藏与取消隐藏。
  - 删除清理 zset / hash / hidden。
  - 计数读取与缺失默认 0。
- `internal/leaderboard/service_test.go`：
  - 只返回已发布、未全局隐藏、未榜单隐藏的项目。
  - 元数据缺失时补齐。
  - 未发布项目被过滤并清理排名。
  - 用户榜累加曝光、admin 榜不累加。
- `internal/handler/leaderboard_test.go`：
  - 用户榜成功 / 未登录 / Redis 错误。
  - admin 列表、隐藏、删除的成功 / 参数错误。
- `internal/handler/publish_test.go`：
  - 根文档 PV +1，`game/*` 不计数。
  - 点击上报 204，非法 slug 404。
  - 发布写 meta，删除发布清理榜单。
  - Leaderboard 为 nil 时不影响原行为。
- `internal/dao/identity_repository_test.go`：
  - `Usernames` 批量查询与空结果。

### Web

- `src/api/leaderboard.test.ts`：信封解包与 limit 参数。
- `src/api/publish.test.ts`：统计接口。
- `src/components/home/LeaderboardPanel.test.ts`：加载、错误、空态、卡片数量与字段渲染。
- `ProfileView` 统计区：汇总与项目明细渲染，停留 / 消费显示 0。

### ops-web

- `src/api/admin.test.ts`：三个榜单接口的 URL、方法与信封解包。
- `src/router/navigation.test.ts` / `routes.test.ts`：新增排行榜项。
- `LeaderboardView`：表格渲染、隐藏 / 删除调用与确认。

### 验证命令

```bash
go test ./...
cd packages/web && npm test -- --run && npm run build && npm run lint
cd packages/ops-web && npm test -- --run && npm run build && npm run lint
make validate
```

## 10. 验收标准

1. web 左侧菜单出现「排行榜」，进入后每行 2 个卡片，展示名次、项目名、作者、访问数。
2. 访问已发布分享页根文档，PV 增加；访问 `game/*` 静态资源不增加。
3. 用户在排行榜页刷新，榜单项目曝光量增加；ops-web 查看不增加。
4. 分享页点击「开始体验」，点击数增加。
5. Redis PV 候选集最多 200 条，用户榜按 PV 倒序始终返回前 50。
6. 未发布 / 已删除 / 全局隐藏项目不出现在榜单。
7. ops-web 排行榜以表格展示，可隐藏 / 取消隐藏、删除榜单数据；操作不影响 `/p/:slug`。
8. 账户设置「我发布的项目」显示曝光量、点击数、停留时长 0、消费额度 0，并提供项目明细。
9. 现有发布、隐藏、删除、排行榜以外的功能与测试全部通过。
10. `make validate` 通过。

## 11. 文件清单

### Backend

- 新增 `internal/leaderboard/store.go`、`service.go`、`store_test.go`、`service_test.go`
- 新增 `internal/handler/leaderboard.go`、`leaderboard_test.go`
- 修改 `internal/handler/publish.go`、`publish_test.go`
- 修改 `internal/router/control.go`
- 修改 `cmd/cp/main.go`
- 修改 `internal/dao/identity_repository.go`、`identity_repository_test.go`
- 修改 `internal/publish/share.html`

### Web

- 新增 `src/api/leaderboard.ts`、`leaderboard.test.ts`
- 新增 `src/components/home/LeaderboardPanel.vue`、`LeaderboardPanel.test.ts`
- 修改 `src/api/publish.ts`、`publish.test.ts`
- 修改 `src/components/home/HomeSidebar.vue`
- 修改 `src/views/HomeView.vue`
- 修改 `src/router/index.ts`
- 修改 `src/views/settings/ProfileView.vue`
- 修改 `src/types/index.ts`
- 修改 `src/styles/home.css`（如需要）

### ops-web

- 新增 `src/views/LeaderboardView.vue`
- 修改 `src/api/admin.ts`、`admin.test.ts`
- 修改 `src/types/index.ts`
- 修改 `src/router/routes.ts`、`navigation.ts` 及对应测试

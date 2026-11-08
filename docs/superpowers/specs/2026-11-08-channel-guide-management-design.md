# 发布渠道与指引管理设计

- 日期：2026-11-08
- 状态：待评审
- 范围：ops-web 管理发布渠道列表与指引内容；CP 动态读取渠道指引；渠道包生成使用最新指引
- 关联文档：`docs/superpowers/specs/2026-11-07-game-channel-publishing-design.md`、`docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md`

## 1. 背景与目标

渠道包能力已经支持微信小游戏、抖音小游戏和 Steam 桌面三个固定渠道，渠道指引通过 `internal/channelpkg/guides/` 的 `go:embed` markdown 提供，设置页只读展示。

当前问题：

- 渠道信息和指引内容写死在代码中，运营无法维护。
- 无法新增仅提供接入说明的渠道（例如后续的 TapTap、华为快游戏等）。
- 修改指引必须发版，且生成包内的 `CHANNEL_GUIDE.md` 无法与前台展示保持同一份内容。

本期目标：

1. ops-web 新增「发布渠道」管理页，支持渠道列表的新增、编辑、停用和删除（内置渠道除外）。
2. 渠道指引 markdown 支持在线编辑，也支持直接上传 `.md` 文件覆盖。
3. CP 的公开指引接口、web 设置页和渠道包生成统一读取 MySQL 中的最新内容。
4. 内置微信、抖音、Steam 渠道可编辑、可停用、不可删除。
5. 新增渠道默认仅提供指引，不参与自动打包。

非目标：

- 不新增渠道包适配器，不让运营配置打包脚本。
- 不做渠道账号授权、自动上传和提审状态回传。
- 不引入 markdown 渲染依赖到 ops-web；ops-web 只负责编辑与上传。
- 不改变现有渠道包 zip 结构、下载接口和已有产物。

## 2. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 渠道指引存 MySQL 文本字段 | 在线编辑和上传共用同一数据源，多实例一致，无文件同步问题 |
| D2 | ops-web 只调用 CP admin API | 遵守现有依赖方向，ops-web 不访问 DP |
| D3 | 内置渠道可编辑、可停用、不可删除 | 保留三个适配器的稳定标识，避免删除后打包无指引 |
| D4 | 新增渠道仅指引，不参与打包 | 打包依赖代码适配器，运营新增内容不应隐式获得打包能力 |
| D5 | `packageable` 由服务端按适配器白名单计算 | 前端不需要硬编码渠道能力，新增适配器时自动生效 |
| D6 | 停用只影响前台展示 | 已存在的生成接口仍可调用；生成包始终能取到指引 |
| D7 | 生成包优先使用数据库指引，缺失时回退内置 markdown | 兼容旧数据、数据库异常和首次部署 |
| D8 | 内置 markdown 仅作为首次种子 | CP 启动时只补种缺失渠道，不覆盖运营已编辑内容 |

## 3. 数据模型

新增迁移 `packages/backend/migrations/00030_channel_guide.sql`。

```sql
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
```

模型文件：`packages/backend/internal/dao/model/channel_guide.go`。

- 不使用软删除；自定义渠道硬删除，避免 `channel` 唯一键与软删除冲突。
- `channel` 是稳定业务标识，也是前台 URL 定位参数，创建后不可修改。
- 内置渠道的 `channel`、`builtin` 由服务端保护；运营只能改内容、排序和启用状态。

### 3.1 种子数据

新增 `channelguide.Service.EnsureDefaults(ctx)`：

- 读取现有 `internal/channelpkg/guides` 的内置渠道元数据和 markdown。
- 对每个内置渠道，若 `channel_guide` 中不存在则插入，`builtin=1`，`enabled=1`。
- 已存在记录不更新，保留运营编辑。
- CP 启动时执行一次；失败记录错误日志但不阻止服务启动，读取时仍有内置 markdown 兜底。

内置渠道：

| channel | title | 默认排序 |
|---|---|---|
| `wechat_minigame` | 微信小游戏 | 10 |
| `douyin_minigame` | 抖音小游戏 | 20 |
| `steam_desktop` | Steam 桌面 | 30 |

## 4. 后端模块

新增 `packages/backend/internal/channelguide/`：

```text
internal/channelguide/
├── service.go       # 校验、默认数据、公开列表、上传解析
├── repository.go    # Repository 接口与错误定义
└── service_test.go
```

新增 `packages/backend/internal/dao/channel_guide_repository.go`，使用 GORM 实现 `channelguide.Repository`。

### 4.1 Repository

```go
type Repository interface {
    List(ctx context.Context, includeDisabled bool, keyword string) ([]daomodel.ChannelGuide, error)
    Get(ctx context.Context, channel string) (*daomodel.ChannelGuide, error)
    Create(ctx context.Context, item *daomodel.ChannelGuide) error
    Update(ctx context.Context, item *daomodel.ChannelGuide) error
    Delete(ctx context.Context, channel string) error
    SeedDefaults(ctx context.Context, items []daomodel.ChannelGuide) error
}
```

`SeedDefaults` 只插入不存在的 `channel`，不更新已有记录。

### 4.2 校验规则

- `channel`：必填，`^[a-z][a-z0-9_]{1,31}$`，全局唯一。
- `title`：去空格后 1–64 字符。
- `summary`：≤255 字符。
- `steps` / `requirements`：各 ≤20 条，每条去空格后 1–120 字符，空行忽略。
- `markdown`：≤512KB，必须是合法 UTF-8；统一换行到 `\n`；去 BOM。
- `enabled`：布尔；新建默认 `true`。
- `sort_order`：0–9999；新建默认 100。
- 自定义渠道删除后允许重新创建同名 `channel`。
- 内置渠道删除返回冲突错误；修改 `channel` 或 `builtin` 返回参数错误。

### 4.3 错误

| 错误 | 含义 |
|---|---|
| `ErrInvalid` | 参数或 markdown 非法 |
| `ErrNotFound` | 渠道不存在 |
| `ErrConflict` | 渠道标识已存在 |
| `ErrBuiltinDelete` | 尝试删除内置渠道 |
| `ErrTooLarge` | markdown 超过 512KB |

## 5. CP API

### 5.1 公开接口

`GET /cp/api/v1/channel-guides`，要求登录，保持现有 JSON 信封：

```json
{
  "code": 0,
  "message": "ok",
  "result": {
    "guides": [
      {
        "channel": "wechat_minigame",
        "title": "微信小游戏",
        "summary": "企业主体、软著、类目审核后导入微信开发者工具",
        "markdown": "...",
        "steps": ["生成渠道包", "注册小游戏 AppID"],
        "requirements": ["企业主体", "软件著作权"],
        "updatedAt": "2026-11-08",
        "enabled": true,
        "builtin": true,
        "packageable": true,
        "sortOrder": 10
      }
    ]
  }
}
```

- 只返回 `enabled=1`。
- 按 `sort_order ASC, channel ASC` 排序。
- `packageable` 为 `channel` 属于 `wechat_minigame`、`douyin_minigame`、`steam_desktop` 时 `true`。
- `updatedAt` 保持 `YYYY-MM-DD` 日期格式，兼容 web 现有展示。

### 5.2 管理接口

所有接口注册在 CP 的 `adminProtected` 组下，自动经过 `admin.AuditMiddleware`。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/cp/api/v1/admin/channel-guides` | 列表，支持 `keyword`、`status=enabled\|disabled\|all` |
| GET | `/cp/api/v1/admin/channel-guides/:channel` | 详情，含 markdown |
| POST | `/cp/api/v1/admin/channel-guides` | 新建自定义渠道 |
| PUT | `/cp/api/v1/admin/channel-guides/:channel` | 更新渠道内容与状态 |
| DELETE | `/cp/api/v1/admin/channel-guides/:channel` | 删除自定义渠道 |
| POST | `/cp/api/v1/admin/channel-guides/:channel/markdown` | 上传 markdown 覆盖正文 |

列表项不含 markdown，返回 `channel`、`title`、`summary`、`enabled`、`builtin`、`sortOrder`、`markdownBytes`、`updatedAt`（RFC3339）。详情额外返回 `markdown`、`steps`、`requirements`。

创建请求：

```json
{
  "channel": "taptap_mobile",
  "title": "TapTap 移动端",
  "summary": "轻量接入说明",
  "markdown": "# TapTap\n...",
  "steps": ["准备资质", "提交审核"],
  "requirements": ["企业主体"],
  "enabled": true,
  "sortOrder": 40
}
```

- 创建时 `builtin` 固定为 `false`，请求中的 `builtin` 忽略。
- 更新请求不允许携带或修改 `channel`、`builtin`；携带不同值时返回 400。
- 上传接口：`multipart/form-data`，字段名 `file`。
  - 文件扩展名必须为 `.md` 或 `.markdown`。
  - `Content-Type` 允许 `text/markdown`、`text/plain`、`application/octet-stream`。
  - 请求体上限 512KB，超限返回 413。
  - 上传只修改 `markdown` 和 `updatedAt`，不清空标题、步骤和资质。
  - 上传成功返回详情。

### 5.3 错误码

| 场景 | HTTP | code |
|---|---|---|
| 参数或 markdown 非法 | 400 | 40001 |
| 渠道不存在 | 404 | 40411 |
| 渠道标识冲突 | 409 | 40905 |
| 删除内置渠道 | 409 | 40905 |
| markdown 超过 512KB | 413 | 41303 |
| 数据库异常 | 500 | 50016 |

## 6. 渠道包生成集成

修改 `internal/channelpkg`：

- `ServiceConfig` 增加 `Guides GuideProvider`。

```go
type GuideProvider interface {
    Markdown(ctx context.Context, channel string) (string, error)
}
```

- `Generate` 在适配器 `Validate`/`Build` 前解析指引：
  - 调用 `Guides.Markdown(ctx, channel)`；
  - 忽略 `enabled`，保证停用渠道仍能生成包；
  - 查询失败或内容为空时使用 `guides.Markdown(channel)` 内置文本；
  - 写入 `core.SourceInfo.Guide`。
- `core.SourceInfo` 增加 `Guide string`，json 忽略。
- 三个适配器的 `writeGuide` 改为：优先写入 `source.Guide`，为空时回退 `guides.Markdown(...)`。
- 渠道包列表/下载接口不变；`guideUrl` 不变。
- 已生成的 zip 不做回溯更新。

## 7. ops-web

### 7.1 路由与导航

- 新增 `packages/ops-web/src/views/ChannelGuidesView.vue`。
- 新增路由 `channel-guides`，路径 `/channel-guides`，标题「发布渠道」。
- 导航放置在「模板管理」之后、「定时任务」之前，图标 `pi pi-share-alt`。
- 同步更新 `navigation.test.ts` 的顺序断言。

### 7.2 API 与类型

`packages/ops-web/src/api/admin.ts` 增加：

- `getChannelGuides(query)`：列表。
- `getChannelGuide(channel)`：详情。
- `createChannelGuide(input)`。
- `updateChannelGuide(channel, input)`。
- `deleteChannelGuide(channel)`。
- `uploadChannelGuideMarkdown(channel, file)`。

`packages/ops-web/src/types/index.ts` 增加 `AdminChannelGuide`、`AdminChannelGuideDetail`、`ChannelGuideInput`、`ChannelGuideStatus`。

### 7.3 页面

- 顶部：标题、说明、渠道数量、关键词搜索、状态筛选、新建按钮。
- 表格列：渠道标识、名称、类型（内置/自定义）、状态、排序、更新时间、操作。
- 操作：
  - 编辑：打开抽屉或弹窗，加载详情。
  - 停用/启用：直接调用更新接口，保留其他字段。
  - 删除：仅自定义渠道显示，二次确认。
- 新建/编辑弹窗字段：
  - 渠道标识（仅新建可填，编辑只读）。
  - 名称、简介、排序、启用。
  - 发布步骤（多行文本，每行一条）。
  - 资质要求（多行文本，每行一条）。
  - 指引 markdown（文本域，显示字节数）。
  - 「上传 markdown」按钮：选择 `.md` 后立即上传并回填正文，不关闭弹窗。
- 保存后刷新列表并提示成功；错误使用 toast 显示后端 message。
- 不引入 markdown 预览依赖；避免 `v-html` 未净化内容。

### 7.4 web 前台

- `ChannelGuideView` 无需改结构，继续使用 `/channel-guides`；新增渠道自动出现在左侧列表。
- `ChannelPackagePanel`：
  - 挂载时调用 `listChannelGuides()`，只保留 `packageable=true` 的渠道作为可选渠道。
  - 默认选中第一个可打包渠道。
  - 接口失败或没有可打包渠道时，回退到内置三渠道，保证发布流程可用。
- `packages/web/src/types/index.ts`：
  - `ChannelGuide.channel` 从固定联合类型改为 `string`。
  - 增加 `enabled: boolean`、`builtin: boolean`、`packageable: boolean`、`sortOrder: number`。
  - `ChannelPackageChannel` 保持现有联合类型，生成请求仍只接受三个内置渠道。

## 8. 安全与边界

- 管理接口必须 `RequireOps`，公开接口必须 `RequireUser`。
- ops-web 不新增任何 DP 或 sandbox 请求。
- markdown 只在 web 设置页通过现有 `renderMarkdown`（`marked` + `DOMPurify`）渲染；ops-web 不渲染 HTML。
- 上传大小在 handler 层用 `http.MaxBytesReader` 限制，非法 UTF-8 和 BOM 在服务层处理。
- 日志只记录渠道标识、操作类型和错误，不记录 markdown 全文。
- 渠道包内 `CHANNEL_GUIDE.md` 不写入数据库 ID、内部路径或操作人信息。

## 9. 测试

### 9.1 Go

- `channelguide`：
  - 渠道标识、标题、步骤、资质校验；
  - markdown 大小、UTF-8、BOM、换行归一化；
  - 内置渠道不可删除、不可改标识；
  - `EnsureDefaults` 只补种缺失项，不覆盖已有内容；
  - 公开列表只返回启用项并正确计算 `packageable`。
- `dao`：CRUD、唯一键冲突、按排序读取、关键词筛选。
- handler：
  - 未登录 401；
  - CRUD 成功与错误码；
  - markdown 上传成功、错误扩展名、超限；
  - CP admin 路由存在；DP 路由不存在。
- `channelpkg`：
  - 生成时使用注入的数据库指引；
  - 指引为空或 provider 报错时回退内置 markdown；
  - 停用渠道仍可生成，包内指引为数据库内容。
- `cmd/cp` 种子调用不阻断启动。

### 9.2 ops-web

- API 信封解包与错误透传。
- 列表状态筛选、关键词参数。
- 步骤/资质文本行转换函数。
- 导航和路由顺序断言更新。
- 内置渠道不显示删除操作、自定义渠道显示。

### 9.3 web

- `packageable` 过滤：自定义渠道不进入发布弹窗。
- 停用内置渠道不出现在发布弹窗。
- 接口失败时回退内置三渠道。
- 设置页继续渲染动态渠道列表。

### 9.4 验收

1. `make db-migrate-up` 只创建表；CP 启动后三渠道自动补种。
2. ops-web 修改微信指引后，web 设置页立即显示新内容。
3. 重新生成微信渠道包，`CHANNEL_GUIDE.md` 与修改后内容一致。
4. 停用抖音渠道后，设置页和发布弹窗都不再展示抖音，但 API 生成仍可用。
5. 新增自定义渠道后，只在设置页展示，不出现在发布弹窗。
6. 上传 markdown 覆盖正文成功，内置渠道删除被拒绝。
7. `make validate` 通过。

## 10. 实施顺序

1. 迁移、DAO model、Repository。
2. `channelguide` 服务、校验、默认数据。
3. CP handler、router、启动种子。
4. `channelpkg` 注入指引与三个适配器回退。
5. ops-web 类型、API、页面、路由与导航。
6. web 渠道包面板过滤与类型调整。
7. 全量测试、`make validate`、手工验收。

## 11. 风险与后续

| 风险 | 处理 |
|---|---|
| 运营误删自定义渠道 | 二次确认；内置渠道禁止删除 |
| markdown 过大或非法 | 512KB 上限、UTF-8 校验、上传扩展名限制 |
| 数据库内容为空导致包内缺指引 | 生成时回退内置 markdown |
| 多实例内容不一致 | 统一读 MySQL，不写本地文件 |
| 后续需要让自定义渠道可打包 | 先实现适配器，再把渠道加入 packageable 白名单 |

后续演进：

1. 渠道字段增加「仅指引 / 可打包」显式配置，由适配器注册表驱动。
2. 增加指引版本历史和回滚。
3. 增加预览与 markdown lint。
4. 增加渠道停用后的生成接口拦截策略（当前只影响展示）。

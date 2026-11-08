# 游戏渠道发行包设计（微信小游戏 / 抖音小游戏 / Steam）

- 日期：2026-11-07
- 状态：已确认，待实现
- 范围：第一期渠道包生成与接入指引，不含平台账号授权和自动上传

## 1. 背景与目标

平台当前只有站内分享发布：CP 把工作区复制到 `PUBLISH_DIR/<projectId>/`，生成 `/p/<slug>/` 分享页。用户希望把生成的游戏发布到微信小游戏、抖音小游戏和 Steam，但各渠道的工程结构、运行时 API、资质和提审流程都不同。

本期目标：

1. 在项目「发布」弹窗内生成可下载的渠道 zip。
2. 每个 zip 包含渠道工程、`package-manifest.json`、兼容性报告和 `CHANNEL_GUIDE.md`。
3. 在设置页提供只读的「发布渠道指引」，展示资质清单、步骤、流程图和常见失败原因。
4. 同时支持平台生成的 HTML5 游戏和用户上传的 Unity/Cocos 已构建成品。

非目标：

- 微信/抖音/Steam 账号授权、自动上传、提审状态回传。
- Steamworks SDK、成就、云存档、内购、广告 SDK。
- Unity/Cocos 源码自动构建。
- 安装包签名、公证、Steam 商店素材生成。
- 修改现有 `/p/<slug>/` 分享发布链路。

## 2. 渠道与来源

| 渠道 | 标识 | 来源 | 首期能力 |
|---|---|---|---|
| 微信小游戏 | `wechat_minigame` | HTML5 工作区 / 引擎成品 | 生成小游戏工程 zip |
| 抖音小游戏 | `douyin_minigame` | HTML5 工作区 / 引擎成品 | 生成小游戏工程 zip |
| Steam 桌面 | `steam_desktop` | HTML5 工作区 / 引擎成品 | 生成 Electron 壳源码 zip 或接收原生桌面成品 |

来源类型：

- `html5`：读取 `WORKSPACES_DIR/<projectPublicId>`，入口为 `index.html` 或 `app.html`。
- `engine_artifact`：用户上传的已构建 zip，作为 `project_resource` 保存，`upload_purpose=channel_build`。

## 3. 用户流程

1. 编辑器点击「发布」，弹窗显示两个页签：`分享页`（现有）和 `渠道包`。
2. 渠道包页签选择渠道。
3. 来源为 HTML5 时直接生成；来源为引擎成品时：
   - 上传 zip（预签名直传或 CP 代理上传，沿用资源上传回退逻辑）；
   - 或从项目已有 `channel_build` 资源中选择。
4. CP 同步校验并生成 zip，返回：
   - 下载地址；
   - 兼容性等级与警告；
   - 该渠道指引链接。
5. 用户在项目发布弹窗查看历史生成记录，可重新下载或删除。
6. 用户进入设置 →「发布渠道指引」，查看资质、流程、包结构和常见问题。

## 4. 来源分级

打包前必须先分级，避免把无法运行的游戏硬打成渠道包。

| 级别 | 判定 | 处理 |
|---|---|---|
| A | 源内已有 `game.js` + `game.json` | 直接重打包，高置信 |
| B | Canvas-first HTML5，DOM 依赖少 | 注入适配层，生成小游戏包 |
| C | 强 DOM / CSS 布局 / 表单 / 多元素渲染 | 拒绝生成，返回改造建议 |

静态检查项：

- 是否包含 `<canvas>`；
- `document.`、`querySelector`、`innerHTML`、`addEventListener` 的使用；
- `localStorage`、`fetch`、`XMLHttpRequest`、`window.location`；
- 外部 URL、跨域资源；
- 入口文件、脚本数量、资源总大小。

分级和检查结果写入 `package-manifest.json` 的 `report`。C 级返回 `ErrUnsupported`，不产生 zip。

## 5. 架构

### 5.1 模块

新增 CP 模块 `packages/backend/internal/channelpkg/`：

```
internal/channelpkg/
├── service.go          # 编排：校验、取源、调用适配器、zip、记录
├── source.go           # WorkspaceSource / ResourceSource
├── adapter.go          # Adapter / Report / Manifest 类型
├── archive.go          # 安全 zip 写入
├── manifest.go         # package-manifest.json
├── report.go           # 兼容性检查与分级
├── guides/             # go:embed 渠道 markdown 和流程数据
└── adapters/
    ├── wechat/
    ├── douyin/
    └── steam/
```

### 5.2 接口

```go
type Source interface {
    Type() string
    Materialize(ctx context.Context) (SourceInfo, error)
    Cleanup() error
}

type SourceInfo struct {
    Dir         string
    Entry       string
    Version     string
    FileCount   int
    TotalBytes  int64
    Report      Report
}

type Adapter interface {
    Channel() string
    Validate(ctx context.Context, source SourceInfo) Report
    Build(ctx context.Context, source SourceInfo, out io.Writer) (Manifest, error)
}
```

```go
type Report struct {
    Level    string   // A / B / C
    Warnings []string
    Blockers []string
}

type Manifest struct {
    ProjectID    string    `json:"projectId"`
    Channel      string    `json:"channel"`
    SourceType   string    `json:"sourceType"`
    SourceVersion string   `json:"sourceVersion"`
    AdapterVersion string  `json:"adapterVersion"`
    Entry        string    `json:"entry"`
    FileCount    int       `json:"fileCount"`
    TotalBytes   int64     `json:"totalBytes"`
    GeneratedAt  time.Time `json:"generatedAt"`
    Level        string    `json:"level"`
    Warnings     []string  `json:"warnings"`
}
```

### 5.3 编排流程

1. Handler 完成登录、工作区成员、项目可见性校验。
2. Service 创建 `project_channel_package` 记录，状态 `building`。
3. 根据 `sourceType` 实例化 `WorkspaceSource` 或 `ResourceSource`。
4. `Materialize` 取源；HTML5 直接读工作区；引擎成品从 S3 取 zip 并安全解压到临时目录。
5. 计算来源版本：
   - HTML5：按排序后的 `(相对路径, 大小, 内容 sha256)` 计算总 sha256，取前 16 位；
   - 引擎成品：资源内容 sha256 前 16 位。
6. 调用适配器 `Validate`；C 级直接失败并记录 `error_message`。
7. 适配器 `Build` 写入临时 zip；同时写 `package-manifest.json` 和 `CHANNEL_GUIDE.md`。
8. zip 完成后原子 rename 到最终路径，更新记录为 `ready`。
9. 任何失败更新为 `failed`，保留可读错误，不保留半成品文件。

### 5.4 存储与限制

- 产物根目录：`CHANNEL_PACKAGES_DIR`，默认 `data/channel-packages`。
- 路径：`<root>/<projectPublicId>/<channel>/<packagePublicId>.zip`。
- 临时目录：`<root>/.tmp-<packagePublicId>-`，失败或完成后清理。
- HTML5 源上限：200MB、20,000 文件。
- 引擎成品 zip：单文件 500MB、解压后 1GB、20,000 文件；解压总大小/压缩包大小超过 100:1 视为 zip bomb 拒绝。
- 产物 zip 上限：1GB。
- 单次生成超时：90s。
- 进程内并发信号量：2。
- 超时或并发超限返回可读错误，不排队阻塞请求。

### 5.5 边界

- 不改 sandbox，不新增 sandbox 接口。
- 工作区以只读方式访问，不写项目文件。
- 不解析 Agent 消息，不依赖聊天历史。
- 现有 `internal/publish` 保持不变；渠道包与分享页互不影响。

## 6. 渠道适配器

### 6.1 公共产物

每个渠道 zip 都必须包含：

```
package-manifest.json
CHANNEL_GUIDE.md
```

`package-manifest.json` 字段与 `Manifest` 一致，并补充 `files` 列表摘要（路径、大小、sha256 前 12 位），用于用户校验和问题排查。文件摘要最多记录 500 条；超过时只保留总数、总大小和按目录聚合的统计，避免 manifest 过大。

### 6.2 微信小游戏 `wechat_minigame`

输出结构：

```
game.js
game.json
project.config.json
adapter/
game/
CHANNEL_GUIDE.md
package-manifest.json
```

- `project.config.json` 使用 `compileType: "game"`，`appid` 为占位 `touristappid`，提示用户替换。
- `game.js` 先加载 `adapter/index.js`，再加载 `game/game.bundle.js`。
- `adapter/index.js` 提供 `window`、`document` 子集、`canvas`、`Image`、`Audio`、触摸事件、`requestAnimationFrame`、`localStorage`（映射到 `wx` 存储）、`fetch`/`XMLHttpRequest`（映射到 `wx.request`）。
- HTML5 脚本抽取：使用 `golang.org/x/net/html` 解析入口 HTML，按文档顺序抽取内联和外部 `<script>`，生成 `game/game.bundle.js`；外部脚本必须位于项目目录内。
- CSS 不参与小游戏渲染；依赖 CSS 布局的游戏判为 C 级。
- 资源路径保持相对路径；禁止绝对路径和外部 URL。
- A 级源（已有 `game.js` + `game.json`）只做结构校验和资源复制，不注入适配层。

### 6.3 抖音小游戏 `douyin_minigame`

输出结构与微信一致，差异：

- `project.config.json` 使用抖音开发者工具字段。
- 适配层把 `wx` 全局替换为 `tt`，使用 `tt.createCanvas`、`tt.getStorageSync`、`tt.request`。
- 同样只支持 A/B 级；C 级拒绝并返回改造清单。

### 6.4 Steam 桌面 `steam_desktop`

HTML5 源输出 Electron 壳源码：

```
app/                     原始游戏文件
electron/main.js
electron/preload.js
package.json
electron-builder.yml
build-steam.sh
build-steam.ps1
steam/app_build.vdf.example
CHANNEL_GUIDE.md
package-manifest.json
```

- 不打包 Electron 二进制；用户在本地执行 `npm install && npm run dist`。
- `app_build.vdf.example` 中 AppID、Depot ID 使用占位符。
- 不接 Steamworks SDK；`main.js` 只负责窗口和加载 `app/index.html`。
- 引擎成品若已包含 Windows `.exe`：
  - 原样放入 `app/`；
  - 跳过 Electron 壳；
  - 仍提供 `steam/app_build.vdf.example` 和启动项说明。

## 7. 数据模型

新增迁移 `packages/backend/migrations/00028_project_channel_package.sql`。

```sql
CREATE TABLE `project_channel_package` (
  `id` bigint UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键id',
  `public_id` char(36) NOT NULL COMMENT '渠道包公开ID',
  `project_public_id` char(36) NOT NULL COMMENT '项目公开ID',
  `workspace_id` bigint UNSIGNED NOT NULL COMMENT '工作区ID',
  `source_type` varchar(16) NOT NULL COMMENT '来源：html5/engine_artifact',
  `source_resource_id` char(36) DEFAULT NULL COMMENT '引擎成品资源ID',
  `channel` varchar(32) NOT NULL COMMENT '渠道：wechat_minigame/douyin_minigame/steam_desktop',
  `status` varchar(16) NOT NULL COMMENT 'building/ready/failed',
  `version` varchar(64) NOT NULL COMMENT '来源版本',
  `file_path` varchar(512) NOT NULL DEFAULT '' COMMENT '相对 CHANNEL_PACKAGES_DIR 的产物路径',
  `file_size_bytes` bigint UNSIGNED NOT NULL DEFAULT 0 COMMENT '产物大小',
  `manifest` json DEFAULT NULL COMMENT 'package-manifest 摘要',
  `error_message` varchar(1024) NOT NULL DEFAULT '' COMMENT '失败原因',
  `created_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` datetime DEFAULT NULL COMMENT 'gorm 软删除时间',
  `extra_p` json DEFAULT NULL COMMENT '扩展字段',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_project_channel_package_public` (`public_id`),
  KEY `idx_project_channel_package_project` (`project_public_id`, `channel`, `created_at`),
  KEY `idx_project_channel_package_status` (`status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='项目渠道发行包';
```

同时扩展 `project_resource.upload_purpose`：

- 新增用途 `channel_build`。
- 只允许 zip：`application/zip`、`application/x-zip-compressed`、`application/octet-stream`。
- 单文件上限 500MB；项目总量沿用 500MB。
- 不允许作为图片、封面或克隆资源使用。

模型文件：`packages/backend/internal/dao/model/channel_package.go`，使用 `Base` 风格的 `CreatedAt/UpdatedAt/DeletedAt/ExtraP`。

## 8. CP API

所有接口要求 HttpOnly Cookie + Redis Session，并校验项目所属工作区在当前会话 `WorkspaceIDs` 内。

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/cp/api/v1/projects/:id/channel-packages` | 同步生成渠道包 |
| GET | `/cp/api/v1/projects/:id/channel-packages` | 渠道包列表，按渠道返回最新及历史 |
| GET | `/cp/api/v1/projects/:id/channel-packages/:packageId/download` | 下载 zip |
| DELETE | `/cp/api/v1/projects/:id/channel-packages/:packageId` | 删除记录与产物 |
| GET | `/cp/api/v1/channel-guides` | 返回三渠道指引、步骤和流程图数据 |

### 8.1 生成请求

```json
{
  "channel": "wechat_minigame",
  "sourceType": "html5",
  "resourceId": ""
}
```

- `sourceType=engine_artifact` 时 `resourceId` 必填，且资源必须属于当前项目、`purpose=channel_build`、状态 ready。
- `channel` 只接受三个枚举值。
- 同一项目同一渠道允许保留多次历史记录；列表默认展示最新一次。
- 生成请求不接收文件内容，避免 CP 承载大 body。

### 8.2 生成响应

```json
{
  "package": {
    "id": "uuid",
    "channel": "wechat_minigame",
    "sourceType": "html5",
    "status": "ready",
    "version": "a1b2c3d4e5f6a7b8",
    "fileSizeBytes": 1234567,
    "level": "B",
    "warnings": ["检测到 document.querySelector，已由适配层兜底，建议真机验证"],
    "createdAt": "2026-11-07T10:00:00Z",
    "downloadUrl": "/cp/api/v1/projects/p1/channel-packages/uuid/download",
    "guideUrl": "/settings/channel-guides?channel=wechat_minigame"
  }
}
```

### 8.3 下载

- 校验登录、工作区成员、记录属于该项目、状态为 `ready`。
- 返回 `Content-Type: application/zip`、`Content-Disposition: attachment; filename="<project>-<channel>-<version>.zip"`、`X-Content-Type-Options: nosniff`。
- 使用 `c.File` 或流式写入；文件不存在时返回 404 并记录错误。
- 下载路径必须 `filepath.Rel` 校验位于 `CHANNEL_PACKAGES_DIR` 内。

### 8.4 错误码

| 场景 | HTTP | code |
|---|---|---|
| 未登录 | 401 | 40101 |
| 项目不存在或非成员 | 404 | 40401 |
| 参数或渠道非法 | 400 | 40001 |
| 来源不匹配或 C 级不支持 | 409 | 40901 |
| 包体过大 | 413 | 41301 |
| 并发超限 | 429 | 42911 |
| 打包失败 | 500 | 50014 |
| 下载记录不存在 | 404 | 40409 |
| 取源不存在 | 404 | 40410 |

## 9. 渠道指引

### 9.1 内容

`internal/channelpkg/guides/` 下维护三份 markdown，通过 `go:embed` 加载：

- `wechat_minigame.md`
- `douyin_minigame.md`
- `steam_desktop.md`

每份包含：

- 资质与账号要求；
- 从生成包到发布的步骤；
- 包结构说明；
- 开发者工具导入方法；
- 常见失败原因；
- 政策更新时间。

`GET /cp/api/v1/channel-guides` 返回：

```json
{
  "guides": [
    {
      "channel": "wechat_minigame",
      "title": "微信小游戏",
      "summary": "企业主体、软著、类目审核后导入微信开发者工具",
      "markdown": "...",
      "steps": ["生成渠道包", "注册小游戏 AppID", "导入开发者工具", "真机调试", "上传提审"],
      "requirements": ["企业主体", "软件著作权", "类目资质"],
      "updatedAt": "2026-11-07"
    }
  ]
}
```

### 9.2 Web

- 新增设置页 `ChannelGuideView.vue`，路由 `/settings/channel-guides`，菜单分组「企业工程」。
- 页面左侧渠道列表，右侧渲染 markdown、资质清单和五步流程图。
- 流程图使用纯 CSS 步骤条，不引入 Mermaid 新依赖。
- 用现有 `renderMarkdown`（`marked` + `DOMPurify`）渲染，禁止直接 `v-html` 未净化内容。
- 发布弹窗的渠道包结果提供「查看指引」入口，带 `?channel=` 参数定位。
- 设置页不保存 AppID、密钥或账号；第一期只读。

## 10. 前端改动

| 文件 | 改动 |
|---|---|
| `packages/web/src/components/editor/PublishDialog.vue` | 增加 `分享页` / `渠道包` 页签 |
| `packages/web/src/components/editor/ChannelPackagePanel.vue` | 新增：渠道选择、来源选择、生成、结果、下载 |
| `packages/web/src/api/channelPackages.ts` | 新增：生成、列表、删除、下载、指引 |
| `packages/web/src/types/index.ts` | 新增渠道包与指引类型 |
| `packages/web/src/router/settingsRoutes.ts` | 新增 `/settings/channel-guides` 和菜单项 |
| `packages/web/src/views/settings/ChannelGuideView.vue` | 新增只读指引页 |
| `packages/web/src/styles/settings.css` | 指引页和渠道包样式，仅黑白灰蓝 |

下载实现：

- 使用 `fetch` 带 Cookie 请求下载 URL；
- 检查 `response.ok`；
- 从 `Content-Disposition` 解析文件名，失败时使用 `<project>-<channel>.zip`；
- 通过 `URL.createObjectURL` 触发浏览器下载并释放。

## 11. 安全

- 所有工作区路径先 `resolve()`，再校验 `is_relative_to`。
- 引擎成品解压必须防 zip-slip：拒绝绝对路径、`..`、符号链接、设备文件。
- 拒绝 NUL 字节、超长路径、超过文件数/大小限制的条目。
- 不执行用户上传内容；打包过程只读和复制。
- 排除点目录、`.env`、`.git`、`docs`、`node_modules`、密钥文件。
- zip 内不写入服务器绝对路径、S3 key、Cookie、内部 token。
- 下载必须鉴权，不允许通过分享页路径直接访问渠道包。
- 日志只记录项目、渠道、包 ID、大小、耗时和错误，不记录文件内容。
- 生成失败不保留半成品；清理临时目录。

## 12. 测试

### 12.1 Go

- `archive`：zip-slip、符号链接、大小/文件数超限、路径规范化。
- `report`：A/B/C 分级、DOM 依赖检测、外部 URL 检测。
- 三个适配器：
  - 微信/抖音：入口、`game.js`、`game.json`、`project.config.json`、manifest、guide 是否存在；
  - Steam：Electron 壳文件、VDF 示例、原生成品分支；
  - A 级透传与 B 级注入。
- `service`：来源版本计算、失败清理、超时、并发信号量。
- handler：未登录、非成员、非法渠道、来源不匹配、下载头、404/409/413/429。
- 迁移：表存在、唯一键、索引。

### 12.2 Web

- API 信封解包和下载文件名解析。
- 渠道选项映射和来源切换。
- 指引页步骤、资质、markdown 渲染。
- 菜单和路由测试。

### 12.3 手工验收

1. Canvas 示例项目生成三个渠道包。
2. DOM 强依赖样本明确失败并给出改造建议。
3. 引擎成品 zip 成功校验和重打包；非法 zip 有可读错误。
4. 非工作区成员无法生成或下载。
5. 现有分享页发布不受影响。
6. 指引页展示三渠道资质、流程和常见问题。
7. 真机/开发者工具验证：
   - 微信开发者工具可导入并预览 B 级 canvas 游戏；
   - 抖音开发者工具可导入并预览；
   - Steam 壳可本地构建并启动。

## 13. 风险与后续

| 风险 | 处理 |
|---|---|
| 小游戏运行时没有 DOM，适配层只能覆盖子集 | A/B/C 分级 + 警告 + C 级拒绝；指引写清限制 |
| Steam 壳依赖用户本地 Node/Electron | 提供脚本和指引，不承诺一键出安装包 |
| 同步打包大文件耗时 | 并发 2 + 90s 超时；超限返回可读错误 |
| 后续需要自动上传 | 数据模型保留 `status`、`extra_p`；可迁移到异步 worker + S3 产物 |
| 平台政策变化 | 指引带 `updatedAt`；每次更新单独提交 |

后续演进：

1. 引入 asynq 异步构建和 S3 产物存储，支持大包和后台任务。
2. 增加渠道账号授权与自动上传、提审状态回传。
3. 增加 Steamworks SDK 包装、成就和云存档。
4. 增加构建模板版本管理，支持多适配器版本并存。

## 14. 实施顺序

1. 迁移、DAO model、Repository。
2. `channelpkg` 核心：source、report、archive、manifest。
3. 微信/抖音/Steam 三个适配器与测试。
4. CP handler、router、配置项、日志和指标。
5. 渠道指引 markdown 与 `GET /channel-guides`。
6. Web API、发布弹窗渠道包页签、设置指引页。
7. 全量测试、`make validate`、真实渠道工具手工验收。

## 15. 配置

| 变量 | 服务 | 默认 | 说明 |
|---|---|---|---|
| `CHANNEL_PACKAGES_DIR` | CP | `data/channel-packages` | 渠道包产物根目录 |
| `CHANNEL_PACKAGE_MAX_BYTES` | CP | `1073741824` | 产物 zip 上限 |
| `CHANNEL_PACKAGE_TIMEOUT` | CP | `90s` | 单次生成超时 |
| `CHANNEL_PACKAGE_CONCURRENCY` | CP | `2` | 进程内并发上限 |

配置加入 `internal/config/config.go` 和对应 `.env.example`，容器挂载持久化目录。

## 16. 决策记录

- 选择方案 A：CP 轻量同步打包 + 适配器，先跑通渠道包和指引，后续再迁异步。
- 渠道入口放项目发布弹窗；设置页只放只读指引，不存账号密钥。
- 微信/抖音首期只承诺 A/B 级；C 级拒绝而不是静默生成不可用包。
- Steam 首期输出 Electron 壳源码，不打包二进制，不接 Steamworks。
- 引擎工程只接收已构建成品，不安装 Unity/Cocos 工具链。

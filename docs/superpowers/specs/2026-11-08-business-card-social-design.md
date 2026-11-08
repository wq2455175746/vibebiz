# 个人名片与编码社交发现页设计

| 项 | 内容 |
|---|---|
| 日期 | 2026-11-08 |
| 状态 | 设计稿 |
| 范围 | `packages/web`、`packages/ops-web`；本期只做前端效果，名片数据本地持久化，复用现有已发布项目 API |
| 关联 | `.ai/rules/architecture.md`、`docs/superpowers/specs/2026-11-07-published-leaderboard-design.md`、`docs/superpowers/specs/2026-10-28-cp-dp-ops-web-design.md` |
| 后续 | 评论、私信（IM）为下一阶段能力，本期只预留禁用入口 |

---

## 1. 背景与目标

平台已有项目创建、发布为 `/p/:slug` 分享页、排行榜与发现能力。本期新增“个人名片”，把开发者自己的已发布项目、个人简介、标签和主预览组合成一张可分享的开发者主页，形成“编码交友”的入口。

本期目标：

1. web 左侧菜单新增“我的名片”，位置在最下方、账户头像上方。
2. 名片编辑页右侧内容区分为内容模块与个人信息模块，宽度比例 `3 : 1`。
3. 内容模块最多添加 5 个自己已发布的项目，列表垂直滚动，点击项目进入对应项目分享页。
4. 项目分享页支持图片 / PDF、视频、HTML 游戏、文档 4 类内容预览。
5. 个人信息模块展示头像、昵称、简介、主预览和最多 5 个标签；主预览只能从已添加项目中选择，或上传单个 PDF / 图片。
6. 整张名片可通过右上角“分享”按钮生成公开名片分享页，内容模块与个人信息模块一起展示。
7. web 左侧菜单新增“发现”，以两列卡片展示所有已发布名片，滚动加载。
8. ops-web 新增“名片管理”页，列表展示名片，支持隐藏 / 恢复 / 删除（本期 mock 数据）。
9. 名片分享页预留“评论”“私信”按钮，点击提示“即将上线”，为后续编码社交能力留位。

非目标：

- 不新增 CP / DP / sandbox 接口，不新增数据库表与迁移。
- 不做真实名片后端持久化、跨设备同步、跨浏览器共享。
- 不做真实评论、点赞、关注、私信、消息通知。
- 不做真实 PDF / 图片上传到 S3；本期用 localStorage `dataUrl` 保存，单文件上限 2MB。
- 不改造现有 CP `/p/:slug` 分享页；本期非 HTML 类型的项目分享页使用前端演示资源。
- 不做名片搜索、排序、审核、举报、内容安全审核。
- ops-web 数据与 web localStorage 不互通，本期为独立 mock。

## 2. 关键决策

| 决策 | 结论 |
|---|---|
| 数据边界 | 前端 + 复用现有 `listPublishedProjects()` / `getProject()`；名片数据存 localStorage |
| 分享页语义 | 两个分享页：项目分享页（点击项目进入）与名片分享页（整张名片） |
| 主预览项目来源 | 只能从内容模块已添加的项目中选择 |
| 非 HTML 内容 | 项目分享页按类型渲染 4 类；真实已发布项目默认 HTML，其余类型使用演示资源 |
| ops-web | 独立 mock 仓储，支持隐藏 / 恢复 / 删除 |
| 社交入口 | 评论、私信按钮预留但禁用 |

## 3. 总体架构

```
web 我的名片页
  ├── listPublishedProjects() ──> CP /cp/api/v1/me/published-projects
  ├── getProject(id) ───────────> CP /cp/api/v1/projects/:id（取 previewPath 推断类型）
  └── CardRepository（localStorage）
        ├── mine
        └── discover mock

web 发现页 ──> CardRepository.listDiscover()（自有已发布名片 + mock）

名片分享页 /share/card/:id ──> CardRepository.getById()
项目分享页 /share/project/:id ──> CardRepository.getById() + 项目类型渲染

ops-web 名片管理 ──> src/api/cards.ts（localStorage mock 仓储）
```

依赖方向不变：

- `web → CP`：仅复用现有已发布项目与项目详情接口。
- `ops-web` 不访问 DP / sandbox，也不访问 web。
- 本期无 CP / DP / sandbox 改动。

## 4. 数据模型

```ts
export type CardContentType = 'html' | 'image' | 'pdf' | 'video' | 'document'

export interface CardProjectItem {
  projectId: string
  name: string
  description: string
  coverUrl: string | null
  type: CardContentType
  shareUrl: string
  previewUrl?: string
  addedAt: string
}

export type CardFeatured =
  | { kind: 'project'; projectId: string }
  | {
      kind: 'file'
      fileType: 'pdf' | 'image'
      name: string
      size: number
      dataUrl: string
    }
  | null

export interface BusinessCard {
  id: string
  ownerUserId: number | null
  ownerName: string
  avatarUrl: string | null
  headline: string
  tags: string[]
  items: CardProjectItem[]
  featured: CardFeatured
  published: boolean
  commentCount: number
  createdAt: string
  updatedAt: string
}
```

约束：

- `items.length <= 5`，`projectId` 不重复。
- `tags.length <= 5`，标签去重，单标签长度 `<= 12`。
- `featured.kind === 'project'` 时，`projectId` 必须存在于 `items`。
- 上传文件只允许 PDF / 图片，`size <= 2 * 1024 * 1024`。
- 自有名片固定 `id = 'me'`；mock 名片使用 `demo-1` ~ `demo-16`。
- `commentCount` 本期用于发现卡片展示，mock 为 0~999，自己名片为 0。

### 4.1 存储

| Key | 内容 |
|---|---|
| `vibebiz:cards:mine:v1` | 当前用户名片 JSON |
| `vibebiz:cards:discover:v1` | mock 名片列表 JSON（首次写入种子，之后可被本地修改） |
| `vibebiz:ops:cards:v1` | ops-web mock 名片列表 JSON |

说明：

- 读取时做 schema 校验与默认值补全；损坏数据回退到空名片或种子数据。
- localStorage 写入失败（配额满）时提示“本地存储空间不足”，不丢失当前内存状态。
- 昵称、头像从 `auth.user` 初始化，不在名片内单独编辑；个人资料统一在账户设置修改。

## 5. Web 设计

### 5.1 左侧菜单与路由

`HomeSidebar.vue`：

- 主导航新增：`{ label: '发现', icon: 'pi pi-compass', route: 'discover' }`，位于“首页”之后。
- `sidebar-footer` 中、头像工具区上方新增“我的名片”按钮：`pi pi-id-card`，点击进入 `my-card`。
- 折叠态下按钮只显示图标，保持现有 `title` / `aria-label` 行为。

`router/index.ts`：

| 路径 | name | 组件 | meta |
|---|---|---|---|
| `/my-card` | `my-card` | `HomeView` | 需登录 |
| `/discover` | `discover` | `HomeView` | 需登录 |
| `/share/card/:id` | `card-share` | `CardShareView` | `public: true` |
| `/share/project/:projectId` | `project-share` | `ProjectShareView` | `public: true` |

- `HomeView.vue` 新增 `isMyCardPage` / `isDiscoverPage` 分支，分别渲染 `MyCardPanel` 与 `DiscoverPanel`，与排行榜页模式一致。
- 文档标题增加“我的名片”“发现”。

### 5.2 我的名片编辑页

布局：

```
┌──────────────────────────────────────────────────────────────┐
│ 我的名片                                         [分享]       │
├───────────────────────────────────────┬──────────────────────┤
│ 内容模块                     [添加项目] │ 个人信息              │
│ ┌───────────────────────────────────┐ │ 头像  昵称            │
│ │ 项目 1                            │ │ 简介输入              │
│ │ 项目 2                            │ │ ┌──────────────────┐ │
│ │ 项目 3          （垂直滚动）        │ │ │ 主预览            │ │
│ │ 项目 4                            │ │ └──────────────────┘ │
│ │ 项目 5                            │ │ 标签  [x] [x] [+ ]   │
│ └───────────────────────────────────┘ │                      │
└───────────────────────────────────────┴──────────────────────┘
```

- 内容区与个人信息区 `flex: 3` / `flex: 1`，窄屏（`< 960px`）上下堆叠。
- 内容模块独立垂直滚动，高度 `calc(100vh - 页头)`；个人信息模块随页滚动或 sticky。
- 整页右上角“分享”按钮：至少 1 个项目时可用，否则禁用并提示“请先添加项目”。
- 修改自动保存（防抖 300ms），页头显示“已保存 / 保存中”状态。

内容模块：

- 顶部标题“我的项目”与“添加项目”按钮；已满 5 个时按钮禁用并提示“最多添加 5 个项目”。
- 每个 item：封面缩略图、项目名、类型徽标（HTML / 图片 / PDF / 视频 / 文档）、简介、移除按钮。
- 点击 item 新窗口打开 `/share/project/:projectId`，`noopener,noreferrer`。
- 空态：“还没有项目，添加自己已发布的作品吧”。

添加项目弹窗：

- 调用 `listPublishedProjects()`；加载 / 错误 / 空态。
- 已加入项目显示“已添加”并禁用；满 5 个后全部禁用。
- 加入时调用 `getProject(projectId)` 获取 `previewPath`，映射内容类型：
  - `.html` / `.htm` → `html`
  - `.pdf` → `pdf`
  - `.mp4` / `.webm` / `.mov` → `video`
  - `.md` / `.markdown` / `.txt` / `.doc` / `.docx` → `document`
  - 图片扩展名 → `image`
  - 其他 → `html`
- `shareUrl` 使用已发布项目的 `url`；`coverUrl` 优先发布封面，回退项目封面。
- 详情请求失败时仍允许添加，类型回退 `html`。

个人信息模块：

- 头像 + 昵称：来自 `auth.user`，只读。
- 简介：单行或两行输入，`maxlength=80`。
- 主预览：
  - 两个入口：“选择项目”和“上传文件”。
  - “选择项目”打开选择器，只列出已添加的 `items`；选中后 `featured = { kind: 'project', projectId }`。
  - “上传文件”接受 `application/pdf,image/*`，超过 2MB 或类型不符时提示；成功转 `dataUrl` 存 `featured = { kind: 'file', ... }`。
  - 已选主预览显示预览图 / PDF 图标 / 项目封面，可点击“移除”恢复为空。
  - 主预览区空态提示“从已添加项目中选择，或上传 PDF / 图片”。
- 标签：
  - 输入框回车、逗号、顿号或失焦添加；去重；最多 5 个；单标签 >12 字时提示。
  - 标签以 chip 展示，带删除按钮。
  - 达到 5 个后输入框禁用并提示“最多 5 个标签”。

### 5.3 项目分享页（4 类内容）

路由 `/share/project/:projectId`，公开访问。

数据解析顺序：

1. 在自有名片的 `items` 中查找 `projectId`。
2. 在 mock 名片的所有 `items` 中查找。
3. 找不到显示“项目不存在或已下线”空态与返回首页按钮。

渲染规则：

| 类型 | 渲染 |
|---|---|
| `html` | 若 `shareUrl` 是真实已发布地址，使用 `iframe` 嵌入；演示数据使用内置 `srcdoc` 小游戏 |
| `image` | `<img>` 展示 `previewUrl` 或演示 SVG |
| `pdf` | `<iframe>` 展示 `previewUrl` 或内置演示 PDF |
| `video` | `<video controls>` 展示 `previewUrl` 或演示视频地址；加载失败显示占位 |
| `document` | 使用 `marked` 渲染 `previewUrl` 文本或内置演示 Markdown |

页面结构：

- 顶部返回按钮、项目名、类型徽标、所属名片主人（头像 + 昵称）。
- 主区域按类型渲染。
- 底部“打开原项目”（真实 `shareUrl` 存在时）与“查看名片”入口。
- 移动端全屏宽度，桌面最大宽度 960px。

已知边界：

- 当前发布产物是 HTML 包装页，真实已发布项目均为 `html`。
- 图片 / PDF / 视频 / 文档类型的真实分享需要后端后续提供原文件访问能力；本期用演示资源保证 4 类渲染效果可验收。

### 5.4 名片分享页

路由 `/share/card/:id`，公开访问。

- `id === 'me'` 时读取自有名片；否则先查自有名片，再查 mock。
- 页面结构：
  - 头部：头像、昵称、简介、标签 chips。
  - 主预览：`featured` 项目封面或上传的 PDF / 图片。
  - 内容区：已添加项目列表（封面、名称、类型徽标），点击进入项目分享页。
  - 社交条：评论数量 + “评论”按钮 + “私信”按钮；两个按钮禁用，点击提示“即将上线”。
  - 右上角“复制分享链接”按钮。
- 自己的名片额外显示“编辑名片”按钮，跳转 `/my-card`。
- 未发布的名片仍可通过直接链接访问，但发现页不展示；名片不存在时显示空态。
- 分享页不接入登录校验，未登录也可访问。

### 5.5 分享动作

- 点击“分享”：校验至少 1 个项目；设置 `published = true` 并保存；弹出分享结果卡片，展示 `/share/card/me` 完整链接，支持复制和打开。
- 已发布后再次点击分享只更新内容并复用同一链接。
- 复制失败时提示手动复制。

### 5.6 发现页

- 顶部标题“发现 · 编码伙伴”与说明。
- 卡片两列（窄屏一列），卡片预览即个人信息模块：
  - 头像、昵称、简介。
  - 主预览缩略图（项目封面 / 上传图片 / PDF 图标）。
  - 最多展示 3 个标签，超出显示 `+N`。
  - 项目数与评论数。
  - 底部“评论即将上线”文字或禁用评论按钮。
- 数据 = 自有已发布名片（置顶）+ mock 名片；每页 6 条，`IntersectionObserver` 触发加载，模拟 300ms 延迟；底部显示“加载中 / 已到底”。
- 点击卡片进入 `/share/card/:id`。
- 空态：“还没有人发布名片”。

## 6. ops-web 设计

- `router/navigation.ts` 新增：`{ name: 'cards', label: '名片管理', path: '/cards', icon: 'pi pi-address-book' }`。
- `router/routes.ts` 新增 `/cards` → `CardsView.vue`。
- 新增 `src/api/cards.ts`：
  - `listCards({ keyword, page, pageSize })`
  - `updateCardHidden(id, hidden)`
  - `deleteCard(id)`
  - 使用 `vibebiz:ops:cards:v1` localStorage 种子数据，模拟 200ms 延迟；首次调用写入种子。
- 页面：
  - 复用 `DataTable`、`TableSearch`、`TablePagination`。
  - 列：名片、用户、标签、项目数、主预览类型、状态、更新时间、操作。
  - 状态：正常 / 已隐藏。
  - 操作：隐藏 / 恢复、删除；删除前 `window.confirm`。
  - 搜索按昵称或用户名过滤；分页每页 10 条。
- ops-web 不访问 DP、sandbox 或 web localStorage，只使用自身 mock 仓储。

## 7. 异常与边界

| 场景 | 行为 |
|---|---|
| localStorage 损坏 | 回退空名片 / 重新写入 mock 种子 |
| localStorage 写入失败 | 提示空间不足，内存状态保留 |
| 已发布项目接口失败 | 弹窗展示错误态与重试按钮 |
| 项目详情接口失败 | 允许添加，类型回退 `html` |
| 重复添加项目 | 禁止，按钮显示“已添加” |
| 超过 5 个项目 / 标签 | 禁止并提示 |
| 标签重复 / 超长 | 去重或提示，不写入 |
| 上传超 2MB / 类型不符 | 提示，不写入 |
| 主预览项目被移除 | `featured` 自动置空 |
| 分享时没有项目 | 分享按钮禁用并提示 |
| 名片分享链接无数据 | 空态 + 返回首页 |
| 项目分享页类型资源加载失败 | 显示占位与错误提示 |
| 评论 / 私信点击 | 提示“即将上线”，不发起请求 |
| ops 删除名片 | 仅影响 mock 列表 |

## 8. 测试

### web

- `src/utils/card.test.ts`：
  - 内容类型映射（html/pdf/video/document/image/未知）。
  - 标签添加、去重、超长、最多 5 个。
  - 项目添加去重与最多 5 个。
  - 主预览项目移除后置空。
  - mock 名片分页与自有名片置顶。
  - localStorage 编解码与损坏数据回退。
- `src/stores/card.test.ts`：
  - 初始化空名片、保存、发布、添加 / 移除项目、设置主预览、标签。
- 组件测试：
  - `MyCardPanel.test.ts`：渲染 3:1 区域、空态、分享按钮禁用 / 可用、添加项目弹窗。
  - `DiscoverPanel.test.ts`：两列卡片、置顶自有名片、滚动加载第二批。
  - `CardShareView.test.ts`：名片字段、内容项目、评论 / 私信禁用提示。
  - `ProjectShareView.test.ts`：4 类内容分别渲染对应节点。
- `api/projects` 与 `api/publish` 已存在，不新增 API 测试。

### ops-web

- `src/router/navigation.test.ts` / `routes.test.ts`：新增名片管理项。
- `src/api/cards.test.ts`：列表过滤、分页、隐藏、删除。
- `CardsView.test.ts`：表格渲染、隐藏 / 恢复调用、删除确认。

### 验证命令

```bash
cd packages/web && npm test -- --run && npm run build && npm run lint
cd packages/ops-web && npm test -- --run && npm run build && npm run lint
make validate
```

## 9. 验收标准

1. web 左侧菜单出现“发现”；侧栏底部、头像上方出现“我的名片”。
2. 我的名片页内容模块与个人信息模块宽度比例约 `3 : 1`，内容列表可垂直滚动。
3. 添加项目弹窗只列出自己已发布项目，最多添加 5 个，重复项目不可添加。
4. 点击内容模块项目进入项目分享页，能分别预览 HTML、图片、PDF、视频、文档。
5. 个人信息模块展示头像、昵称、简介、主预览与标签；主预览只能选已添加项目或上传 PDF / 图片；标签最多 5 个。
6. 右上角分享按钮生成公开名片分享页，包含内容模块与个人信息模块。
7. 名片分享页评论 / 私信按钮可见但禁用，点击提示“即将上线”。
8. 发现页两列展示名片卡片，滚动可加载更多，自己的已发布名片置顶。
9. ops-web 出现“名片管理”，支持搜索、隐藏 / 恢复、删除 mock 名片。
10. 现有功能与测试不回归，`make validate` 通过。

## 10. 文件清单

### web

新增：

- `src/components/card/MyCardPanel.vue`、`MyCardPanel.test.ts`
- `src/components/card/DiscoverPanel.vue`、`DiscoverPanel.test.ts`
- `src/components/card/AddProjectDialog.vue`
- `src/components/card/CardProjectItem.vue`
- `src/components/card/CardProfilePanel.vue`
- `src/components/card/CardPreview.vue`
- `src/components/card/CardTile.vue`
- `src/views/CardShareView.vue`、`CardShareView.test.ts`
- `src/views/ProjectShareView.vue`、`ProjectShareView.test.ts`
- `src/utils/card.ts`、`card.test.ts`
- `src/stores/card.ts`、`card.test.ts`
- `src/mocks/cards.ts`

修改：

- `src/types/index.ts`
- `src/components/home/HomeSidebar.vue`
- `src/views/HomeView.vue`
- `src/router/index.ts`
- `src/styles/home.css`（或新增 `src/styles/card.css` 并在入口引入）

复用：

- `src/api/projects.ts`：`getProject`
- `src/api/publish.ts`：`listPublishedProjects`
- `src/utils/preview.ts`：扩展内容类型判断
- `src/stores/auth.ts`：头像、昵称

### ops-web

新增：

- `src/views/CardsView.vue`、`CardsView.test.ts`
- `src/api/cards.ts`、`cards.test.ts`
- `src/mocks/cards.ts`

修改：

- `src/types/index.ts`
- `src/router/routes.ts`
- `src/router/navigation.ts`
- `src/router/navigation.test.ts`、对应路由测试

### 不修改

- `packages/backend`、`packages/sandbox`、数据库、迁移、现有 CP `/p/:slug` 分享页。

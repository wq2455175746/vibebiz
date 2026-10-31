# 克隆能力 · 设计规格

| 项 | 内容 |
|---|---|
| 日期 | 2026-10-31 |
| 状态 | 已确认 |
| 范围 | 克隆入口、创建表单、克隆编辑器、5 阶段流程、附件、项目模式与阶段契约、数据模型、安全与测试 |
| 关联文档 | `docs/AI-Coding-Platform-技术设计文档.md`、`docs/superpowers/specs/2026-10-30-enterprise-engineering-design.md`、`.ai/rules/architecture.md` |
| 数据库 | 目标 MySQL 8；新增 `project_clone_source`、`project_clone_asset` 两张表，`project.mode` 增加 `clone` |
| 实施顺序 | 先做前端本地 mock，再补 CP / DP / sandbox 与真实 Agent 链路 |

---

## 1. 背景与目标

现有平台已经具备企业工程开发流程：需求梳理、概要设计、详细设计、测试用例、全栈开发、制品空间、交付空间。克隆能力要复用这套“分阶段、逐步确认”的交互，但流程内容不同：

1. 用户先描述要复刻的软件或网站，并可选提供目标 URL、参考图片、单元测试压缩包、文档说明。
2. 系统先拆解目标软件本身的技术架构，再设计我们自己的克隆实现架构。
3. 后续依次完成测试、全栈开发，并直接进入制品空间，不提供交付空间。
4. 创建入口独立成左侧菜单「克隆」，交互形态与「定时任务」一致：空态居中一个按钮，点击弹出创建对话框。

目标：

- 新增「克隆」信息架构与完整的创建、列表、详情、编辑器体验。
- 定义克隆项目的 5 个阶段、固定产物与逐步确认规则。
- 定义克隆来源资料（URL、图片、单测 zip、文档）的保存与使用方式。
- 给出前端本地 mock 与后续真实后端的分层方案，避免前端返工。
- 复用现有资源上传、阶段接口、企业编辑器与制品空间，不重复造轮子。

非目标：

- 本轮不抓取目标 URL，不做页面爬虫、截图或浏览器采集。
- 本轮不实现真实 Agent 自动生成代码；前端先使用本地 mock 验证交互与样式。
- 不新增交付空间；不扩展交付流水线。
- 不改变现有企业工程开发流程与企业项目行为。
- 不引入新的前端依赖或新的 UI 组件库。

## 2. 关键决策

| # | 决策 | 理由 |
|---|---|---|
| D1 | 克隆是独立菜单与路由，不是企业工程开发的二级入口 | 入口语义清晰，便于后续独立演进 |
| D2 | 创建成功后直接进入克隆编辑器 | 用户目标明确，减少一次列表跳转 |
| D3 | 克隆使用新的 `project.mode = clone` | 阶段集合、默认角色与工作区不同，不能混用 enterprise |
| D4 | URL 只作为文本参考，服务端不出站 | 规避 SSRF、反爬与授权风险，第一版不做采集 |
| D5 | 克隆流程固定 5 段：基础设计、架构、测试、全栈开发、制品空间 | 与用户确认的流程一致，去掉交付空间 |
| D6 | 基础设计先分析目标软件技术架构，架构阶段再设计克隆实现 | 先理解被复刻对象，再决定怎么复刻 |
| D7 | 图片总数最多 9 张，其中 1 张主图 | 与用户确认的口径一致 |
| D8 | 附件复用现有 staging/adopt 与 `project_resource` | 不重复实现上传、S3 与工作区资源同步 |
| D9 | 前端第一阶段只做本地 mock，不调用 CP/DP | 先确认样式与交互，避免依赖未落地的后端 |
| D10 | 测试阶段先生成方案，再执行上传的单测并产出报告 | 单测 zip 是克隆验证输入，不是普通附件 |

## 3. 信息架构与交互

### 3.1 导航

左侧主导航在「企业工程开发」之后增加：

```text
首页
企业工程开发
克隆
资源
我的项目
定时任务
```

- 图标：`pi pi-copy`。
- 路由：`/clones`。
- 选中态与其他导航一致。

### 3.2 克隆页

`/clones` 继续使用首页壳层（`HomeSidebar` + `home-main`），内容区域渲染 `ClonePanel`。

空态：

- 居中标题「克隆一个软件或网站」。
- 一句说明：「上传参考资料，让 Vibebiz 先拆解目标架构，再按阶段复刻。」
- 主按钮「创建克隆」。

已有克隆项目时：

- 左侧任务式列表：名称、目标域名/URL、当前阶段、更新时间。
- 右侧详情：阶段进度、来源信息、附件摘要、创建时间、操作按钮。
- 操作：打开编辑器、删除。
- 列表排序：`updatedAt` 降序。

### 3.3 创建对话框

点击「创建克隆」打开 PrimeVue `Dialog`，宽度约 720px，移动端全宽。

字段：

| 字段 | 必填 | 规则 |
|---|---|---|
| 名称 | 是 | 去空格后 1–80 字 |
| 描述 | 是 | 去空格后 1–2000 字 |
| 目标 URL | 否 | 空或 `http/https`，≤1024 字符；只展示，不抓取 |
| 图片 | 否 | 最多 9 张；其中仅 1 张主图；jpg/png/webp/gif；单张 ≤10MB |
| 单元测试压缩包 | 否 | 1 个 `.zip`；≤100MB |
| 文档说明 | 否 | 1 个文件；≤100MB |

交互细节：

- 图片支持点击选择与拖拽；缩略图网格；每张可删除；可点击「设为主图」。
- 未指定主图时，第一张图片自动成为主图。
- 已达到 9 张或已有 zip/文档时，对应入口禁用并提示。
- 图片使用 `URL.createObjectURL` 即时预览；组件卸载时释放。
- 「Clone」按钮在必填缺失、字段非法或提交中时禁用。
- 提交错误显示在对话框内，不清空已填内容。
- 对话框关闭后重新打开应重置表单与临时预览。

### 3.4 克隆编辑器

路由：`/clone/editor/:id`。

布局复用企业编辑器视觉：

- 顶部：项目名、状态、返回克隆页。
- 阶段条：5 个阶段，显示锁定/进行中/已完成。
- 左侧：对话区（前端 mock 阶段展示静态引导消息与输入框）。
- 右侧：根据当前阶段切换工作区。

各阶段工作区：

| 阶段 | 工作区 | 内容 |
|---|---|---|
| 基础设计 | 文档 | 目标技术架构分析、功能/页面清单、克隆范围 |
| 架构 | 文档 | 克隆实现架构、模块、数据模型、接口、目录与里程碑 |
| 测试 | 测试 | 测试方案、单测执行状态与日志、测试报告 |
| 全栈开发 | 开发 | 预览 / 代码 / 控制台三页签（mock 内容） |
| 制品空间 | 制品 | 构建与制品记录（mock 内容），无交付入口 |

阶段操作：

- 当前阶段为 `active` 且未开始时，显示「开始阶段」。
- 开始后生成该阶段 mock 产物，显示「确认并继续」。
- 确认后当前阶段置为 `done`，下一阶段置为 `active`。
- 已完成阶段可回看；未解锁阶段不可点击。
- 全栈开发确认后，制品空间解锁为 `active`，不再出现交付空间。
- 克隆编辑器第一阶段不调用 CP/DP；阶段、内容与附件均来自本地 store。

### 3.5 资料区

创建后可在编辑器或详情中继续补充：

- 图片：仍受总数 9 张、主图唯一、类型与大小限制。
- 文档：可追加多个通用附件，单文件 ≤100MB。
- 单测 zip：第一版只允许 1 个；替换需先删除旧文件。
- 主图可更换；删除主图后，剩余第一张图片自动成为主图。

## 4. 克隆阶段定义

| # | key | 标题 | 角色 | 固定产物 | 工作区 |
|---|---|---|---|---|---|
| 1 | `base_design` | 基础设计 | Bob · 系统架构师（目标分析） | `docs/clone/base-design.md` | document |
| 2 | `architecture` | 架构 | Bob · 系统架构师（方案设计） | `docs/clone/architecture.md` | document |
| 3 | `test` | 测试 | QA · 测试开发工程师 | `docs/clone/test-plan.md`、`docs/clone/test-report.md` | test |
| 4 | `development` | 全栈开发 | Alex · 全栈工程师 | `docs/clone/development.md` | development |
| 5 | `artifacts` | 制品空间 | 制品流水线 | 无固定文档 | artifact |

### 4.1 基础设计（目标拆解）

目的：先理解被复刻对象，而不是直接设计我们的实现。

产物 `docs/clone/base-design.md` 必须包含：

1. 目标产品概览与使用场景。
2. 技术栈推测与证据来源：前端框架、后端语言/框架、数据库、部署形态、第三方服务。
3. 系统与模块结构：核心模块、依赖关系、边界。
4. 数据模型与关键实体。
5. API / 路由 / 集成点清单。
6. 页面与功能清单：页面、核心流程、交互状态。
7. 从参考图片和文档中提取的 UI 结构、布局与设计约束。
8. 可复刻范围、不复刻范围、信息缺口与风险。
9. 验收基线：哪些行为复刻后必须一致。

约束：

- URL 仅作为文本参考，不访问目标站点。
- 所有推测必须标注为「已确认」「推断」「缺失」之一。
- 信息不足时列出需要用户补充的问题，不编造事实。

### 4.2 架构（克隆实现）

产物 `docs/clone/architecture.md` 必须包含：

1. 技术选型与理由。
2. 系统分层、模块边界与目录结构。
3. 数据模型、存储与迁移策略。
4. API 契约与前后端交互。
5. 关键页面与组件拆分。
6. 测试策略与单测 zip 接入方式。
7. 构建、制品与发布方案。
8. 里程碑、风险与回滚点。

### 4.3 测试

产物：

- `docs/clone/test-plan.md`：测试范围、环境、用例映射、通过标准。
- `docs/clone/test-report.md`：执行时间、命令、通过/失败/跳过数量、失败原因、覆盖率（如有）、结论。

规则：

- 测试方案必须覆盖基础设计中的验收基线。
- 上传的单测 zip 解压到工作区隔离目录，路径穿越校验后执行。
- 测试命令由检测到的技术栈决定，只允许受控命令，禁止任意网络访问。
- 失败时报告必须给出回到全栈开发的修复项；不得直接标记阶段完成。
- 无单测 zip 时，仍生成方案与基于现有代码的可执行检查报告。

### 4.4 全栈开发

产物 `docs/clone/development.md` 记录：

- 已完成模块与对应需求/验收项。
- 关键实现决策与偏离架构的原因。
- 未完成项与已知问题。
- 运行与预览方式。

工作区沿用现有预览、代码、控制台能力。

### 4.5 制品空间

- 复用现有 `ArtifactWorkspace`。
- 产出代码归档、镜像/构建记录与可下载制品。
- 不创建交付阶段，不展示交付入口。

## 5. 前端设计

### 5.1 本阶段（本地 mock）

新增文件：

```text
packages/web/src/types/clone.ts
packages/web/src/utils/clone.ts
packages/web/src/utils/clone.test.ts
packages/web/src/stores/cloneProjects.ts
packages/web/src/components/clone/ClonePanel.vue
packages/web/src/components/clone/CloneCreateDialog.vue
packages/web/src/components/clone/CloneAssetField.vue
packages/web/src/views/CloneEditorView.vue
packages/web/src/styles/clone.css
```

修改文件：

```text
packages/web/src/router/index.ts
packages/web/src/components/home/HomeSidebar.vue
packages/web/src/views/HomeView.vue
```

数据流：

```text
ClonePanel
  ├─ CloneCreateDialog
  │    └─ validateCloneDraft / addCloneAsset
  └─ cloneProjects store
       └─ localStorage: vibebiz:clone:projects:v1

CloneEditorView
  └─ cloneProjects store
       ├─ stage definitions
       ├─ mock contents
       └─ advanceCloneStage
```

Mock 类型：

```ts
type CloneStageKey = 'base_design' | 'architecture' | 'test' | 'development' | 'artifacts'
type CloneStageStatus = 'locked' | 'active' | 'done'

interface CloneAsset {
  id: string
  name: string
  size: number
  mime: string
  role: 'image' | 'unit_test' | 'docs'
  previewUrl?: string
}

interface CloneProject {
  id: string
  name: string
  description: string
  sourceUrl: string
  assets: CloneAsset[]
  coverAssetId: string | null
  stages: Record<CloneStageKey, {
    status: CloneStageStatus
    startedAt: string | null
    completedAt: string | null
  }>
  contents: Partial<Record<CloneStageKey, string>>
  createdAt: string
  updatedAt: string
}
```

`coverAssetId` 是主图唯一真相源；图片统一使用 `role = image`，保存到后端时再映射为 `cover` 或 `gallery`。

初始阶段状态：

```text
base_design: active
architecture: locked
test: locked
development: locked
artifacts: locked
```

持久化规则：

- 读取时校验版本与字段，损坏数据回退为空数组。
- 只持久化元数据与阶段状态；`previewUrl` 不持久化。
- 图片刷新后显示占位缩略图，不伪造本地文件。
- 删除项目同时清理对应本地记录。

纯函数（必须有 vitest）：

```text
validateCloneDraft
canSubmitClone
addCloneAsset
removeCloneAsset
setCoverAsset
advanceCloneStage
createInitialCloneStages
cloneStageDefinition
parseCloneProjects
serializeCloneProjects
```

### 5.2 后续真实后端

克隆编辑器改为读取真实项目、阶段与资源；企业编辑器与克隆编辑器共享视觉组件，但阶段定义按 `project.mode` 注入，不复制两份编辑器。

## 6. 后端与契约（后续实现）

### 6.1 项目与阶段

- `internal/model` 增加 `ProjectModeClone = "clone"`。
- `projectstage.DefinitionsFor(mode)`：`enterprise` 返回现有 7 段，`clone` 返回 5 段。
- `projectstage` 增加 `WorkspaceTest = "test"`，仅克隆测试阶段使用；`artifacts` 使用 `WorkspaceArtifact`。
- 创建克隆项目时写入 clone 阶段，不写 enterprise 阶段；`artifacts` 初始为 `locked`，全栈开发完成后由阶段完成逻辑置为 `active`。
- `/projects/:id/stages` 返回项目实际 mode 与对应阶段。
- 阶段开始/完成接口保持不变，服务端按 mode 校验阶段 key。

### 6.2 数据模型

`project_clone_source`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint unsigned | 主键 |
| `project_public_id` | char(36) | 项目 UUID，唯一 |
| `source_url` | varchar(1024) | 目标 URL，可为空 |
| `status` | varchar(16) | `ready` / `disabled` |
| `created_at` | timestamp | 创建时间 |
| `updated_at` | timestamp | 更新时间 |
| `deleted_at` | datetime | 软删除 |

`project_clone_asset`：

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint unsigned | 主键 |
| `project_public_id` | char(36) | 项目 UUID |
| `resource_id` | char(36) | 对应 `project_resource.resource_id` |
| `role` | varchar(16) | `cover` / `gallery` / `unit_test` / `docs` |
| `sort_order` | int | 图片排序 |
| `created_at` | timestamp | 创建时间 |
| `deleted_at` | datetime | 软删除 |

约束：

- 同一项目仅一条 `cover`、一条 `unit_test`；`docs` 可多条，用于创建后继续补充文档。
- 图片总数（`cover + gallery`）≤9。
- 图片、zip、文档实体仍存放在 `project_resource`，不重复保存文件。

### 6.3 API

CP：

```text
POST   /cp/api/v1/clones
GET    /cp/api/v1/clones
GET    /cp/api/v1/clones/:id
DELETE /cp/api/v1/clones/:id
```

创建请求：

```json
{
  "name": "目标名称",
  "description": "目标描述",
  "sourceUrl": "https://example.com",
  "assets": {
    "coverResourceId": "uuid",
    "galleryResourceIds": ["uuid"],
    "unitTestResourceId": "uuid",
    "docsResourceId": "uuid"
  }
}
```

创建流程：

1. 校验字段与资源归属、大小、角色。
2. 创建 `project`，`mode = clone`。
3. 调用 sandbox 初始化工作区。
4. 写入 `project_clone_source` 与 `project_clone_asset`。
5. 创建 clone 阶段，`base_design` 为 active。
6. 导入克隆工程模板。
7. 失败时回滚项目、阶段、来源记录与资源绑定。

DP：

```text
POST   /dp/api/v1/projects/:id/clone/assets
DELETE /dp/api/v1/projects/:id/clone/assets/:assetId
GET    /dp/api/v1/projects/:id/clone/assets
```

- 创建后追加附件先走现有 staging/adopt，再绑定角色。
- 所有接口要求登录、工作区成员关系与项目归属校验。

### 6.4 上传限制

复用 `POST /dp/api/v1/resources/presign`，增加 `purpose`：

| purpose | 单文件上限 | 允许类型 |
|---|---|---|
| `clone_image` | 10MB | jpg/png/webp/gif |
| `clone_unit_test` | 100MB | zip |
| `clone_docs` | 100MB | 任意二进制，前端展示名称与大小 |
| 现有默认 | 20MB | 不变 |

克隆项目附件总量 ≤500MB。通用项目资源仍保持 20MB / 200MB 限制。

### 6.5 Sandbox

- 克隆项目初始化时生成与普通项目一致的工作区骨架。
- 来源元数据写入 `.clone/source.json`，只含名称、描述、URL 与资源展示信息，不含文件正文。
- 单测 zip 解压到 `.clone/tests/`；文档与图片进入 `.resources/`。
- `.clone/`、`.resources/` 不进入文件树、预览与 git。
- 测试执行使用受控命令与独立临时目录；解压前校验条目路径和总量。

## 7. 安全

- URL 不出站，杜绝 SSRF。
- 前端展示 URL 时使用纯文本或带 `rel="noopener noreferrer"` 的链接，不自动加载目标资源。
- 附件类型与大小在前后端双重校验。
- zip 解压拒绝绝对路径、`..`、符号链接与超限条目。
- 测试命令白名单化，禁止任意网络与宿主机路径访问。
- 不在日志中记录文档正文、聊天正文或附件内容。
- 克隆项目与其他项目一样要求登录和工作区隔离。

## 8. 测试与验收

### 8.1 前端本阶段

vitest：

- 名称、描述、URL 校验。
- 图片数量、主图唯一、类型与大小校验。
- zip/文档数量与大小校验。
- 阶段初始化与逐步推进。
- 本地数据序列化、反序列化与损坏兜底。
- 删除项目与删除附件后的主图回退。

手工验收：

- 侧栏「克隆」可进入。
- 空态按钮居中，创建对话框字段完整。
- 错误提示、禁用态、图片预览与主图切换正确。
- 创建后进入克隆编辑器，5 阶段可逐步推进与回看。
- 刷新后项目与阶段保留。
- 移动端布局不溢出；颜色仅黑白灰蓝。
- 全程不调用 CP/DP。

质量门禁：

```bash
cd packages/web && npm run test
cd packages/web && npm run build
cd packages/web && npm run lint
make validate
```

### 8.2 后续后端

- `go test ./...`：clone 创建、阶段定义、资源归属、回滚、权限隔离。
- `pytest`：zip 解压安全、测试目录隔离、报告生成。
- 端到端：上传资料 → 创建克隆 → 基础设计 → 架构 → 测试执行 → 全栈开发 → 制品空间。
- 确认 enterprise 流程与现有测试不回归。

## 9. 实施顺序

1. 写 `types/clone.ts`、`utils/clone.ts` 与 vitest。
2. 写 `stores/cloneProjects.ts` 本地持久化。
3. 写 `CloneAssetField.vue`、`CloneCreateDialog.vue`。
4. 写 `ClonePanel.vue` 与克隆页空态、列表、详情。
5. 写 `CloneEditorView.vue` 与 5 阶段 mock 工作区。
6. 接入侧栏、路由、`HomeView` 与 `clone.css`。
7. 跑 vitest、build、lint、`make validate`，用浏览器截图检查桌面与移动端。
8. 后端按第 6 节实现 CP / DP / sandbox，并把 mock store 替换为真实 API。

## 10. 已确认决策记录

- 第一版范围：只做前端 + 本地 mock。
- 创建成功后：进入克隆编辑器。
- 目标 URL：只作为参考，不抓取。
- 阶段：基础设计 → 架构 → 测试 → 全栈开发 → 制品空间，无交付空间。
- 测试阶段：生成测试方案 + 执行上传单测 + 产出报告。
- 图片口径：最多 9 张，其中 1 张主图。
- 基础设计：先分析目标软件的技术架构、功能/页面清单与克隆范围。

## 11. 实现状态（2026-10-31）

已实现：

- 前端已从 localStorage mock 切换到 CP/DP + MySQL；创建、列表、详情、阶段、附件、聊天、预览、单测执行均走真实接口。
- sandbox 已同步 `.clone/source.json`、解压单测到 `.clone/tests`、受控执行 pytest/npm 测试并写入 `docs/clone/test-report.md`。
- `00023_clone_capability.sql`：`project_clone_source`、`project_clone_asset`、`project_resource.upload_purpose`；已在本机目标库执行到 v23。
- `project.mode = clone` 与 5 阶段定义；阶段接口按 mode 返回企业或克隆阶段，克隆制品阶段无需固定文档即可完成。
- CP：`POST/GET /cp/api/v1/clones`、`GET/DELETE /cp/api/v1/clones/:id`；创建时完成沙箱初始化、阶段创建、Harness 导入、资源采纳与回滚。
- DP：`GET/POST /dp/api/v1/projects/:id/clone/assets`、`DELETE .../:resourceId`、`PUT .../:resourceId/cover`。
- 资源上传目的与限额：`clone_image` 10MB、`clone_unit_test` / `clone_docs` 100MB、克隆项目总量 500MB。
- Go 单测覆盖克隆服务、HTTP handler、阶段模式、资源限额、迁移与 MySQL store。

待实现：

- 真实制品构建流水线（源码归档、镜像、版本说明）。
- 通用项目删除时级联清理克隆来源、附件与阶段记录。
- 生产环境单测执行需要容器隔离与网络限制；当前仅本地 harness 开启。

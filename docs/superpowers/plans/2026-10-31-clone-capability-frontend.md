# 克隆能力前端（本地 Mock）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 在 `packages/web` 落地「克隆」菜单、创建对话框与 5 阶段克隆编辑器，全部使用本地 mock 数据。

**Architecture:** 纯函数与类型放在 `utils/clone.ts` / `types/clone.ts`，Pinia store 负责 localStorage 持久化；页面由 `ClonePanel`、`CloneCreateDialog`、`CloneAssetField` 和 `CloneEditorView` 组成。编辑器只读本地 store，不调用 CP/DP。

**Tech Stack:** Vue 3 + `<script setup lang="ts">` + Pinia + PrimeVue + Vitest。

**Spec:** `docs/superpowers/specs/2026-10-31-clone-capability-design.md`

## Global Constraints

- 不调用 CP/DP，不新增依赖。
- 颜色仅黑 / 白 / 灰 / 蓝。
- 克隆阶段固定：`base_design` → `architecture` → `test` → `development` → `artifacts`。
- 图片最多 9 张，其中 1 张主图；单张 ≤10MB；支持 jpg/png/webp/gif。
- zip / 文档单文件 ≤100MB；zip 仅 1 个。
- 名称必填 ≤80 字；描述必填 ≤2000 字；URL 仅空或 http/https。
- 每个阶段必须逐步开始、确认；全栈开发完成后制品空间解锁。
- 完成后必须通过 `npm run test`、`npm run build`、`npm run lint`、`make validate`。

---

### Task 1: 类型、阶段定义、校验与持久化纯函数

**Files:**
- Create: `packages/web/src/types/clone.ts`
- Create: `packages/web/src/utils/clone.ts`
- Test: `packages/web/src/utils/clone.test.ts`

**Interfaces:**
- Produces:
  - `CloneStageKey`、`CloneStageStatus`、`CloneStageState`、`CloneStageDefinition`、`CloneAsset`、`CloneProject`、`CloneDraft`。
  - `CLONE_STAGES: CloneStageDefinition[]`
  - `createInitialCloneStages(): CloneProject['stages']`
  - `validateCloneDraft(draft: CloneDraft): CloneDraftErrors`
  - `canSubmitClone(draft: CloneDraft): boolean`
  - `isCloneUrl(value: string): boolean`
  - `validateCloneAsset(file: FileLike, assets: CloneAsset[], role: CloneAsset['role']): string`
  - `addCloneAsset(assets: CloneAsset[], asset: CloneAsset): CloneAsset[]`
  - `removeCloneAsset(assets: CloneAsset[], id: string): CloneAsset[]`
  - `nextCoverAfterRemoval(assets: CloneAsset[], coverId: string | null, removedId: string): string | null`
  - `advanceCloneStage(stages: CloneProject['stages']): CloneProject['stages']`
  - `startCloneStage(project: CloneProject, key: CloneStageKey, now?: Date): CloneProject`
  - `createCloneProject(id: string, draft: CloneDraft, assets: CloneAsset[], now?: Date): CloneProject`
  - `cloneStageDefinition(key: CloneStageKey): CloneStageDefinition`
  - `mockStageContent(project: CloneProject, key: CloneStageKey): string`
  - `parseCloneProjects(raw: string | null): CloneProject[]`
  - `serializeCloneProjects(projects: CloneProject[]): string`

- [x] **Step 1: Write failing tests**

覆盖：
- 必填、长度、URL 协议。
- 图片第 10 张拒绝；第 10 张提示「最多 9 张」。
- 非图片、超大图片、超大 zip/文档拒绝。
- 删除主图后回退到第一张图片；删除最后一张图后为 `null`。
- 初始阶段只有 `base_design` 为 active。
- `advanceCloneStage` 只推进 active，并把下一段置 active；`development` 完成后 `artifacts` active。
- `parseCloneProjects` 对 `null`、非法 JSON、缺字段数据返回安全结果。
- `createCloneProject` 生成合法时间与初始阶段。

- [x] **Step 2: Run test to verify it fails**

```bash
cd packages/web && npm run test -- src/utils/clone.test.ts
```

- [x] **Step 3: Implement types and pure functions**

阶段定义：

```ts
export const CLONE_STAGES: CloneStageDefinition[] = [
  { key: 'base_design', seq: 1, title: '基础设计', roleLabel: 'Bob · 系统架构师（目标分析）', artifactPath: 'docs/clone/base-design.md', workspace: 'document' },
  { key: 'architecture', seq: 2, title: '架构', roleLabel: 'Bob · 系统架构师（方案设计）', artifactPath: 'docs/clone/architecture.md', workspace: 'document' },
  { key: 'test', seq: 3, title: '测试', roleLabel: 'QA · 测试开发工程师', artifactPath: 'docs/clone/test-plan.md', workspace: 'test' },
  { key: 'development', seq: 4, title: '全栈开发', roleLabel: 'Alex · 全栈工程师', artifactPath: 'docs/clone/development.md', workspace: 'development' },
  { key: 'artifacts', seq: 5, title: '制品空间', roleLabel: '制品流水线', artifactPath: '', workspace: 'artifact' },
]
```

持久化格式：

```json
{ "version": 1, "projects": [] }
```

`parseCloneProjects` 必须逐项校验字段类型，丢弃损坏项而不是整体失败。

- [x] **Step 4: Run test to verify it passes**

```bash
cd packages/web && npm run test -- src/utils/clone.test.ts
```

---

### Task 2: Pinia store

**Files:**
- Create: `packages/web/src/stores/cloneProjects.ts`

**Interfaces:**
- Consumes: Task 1 的所有导出。
- Produces:
  - `useCloneProjectsStore()` 返回 `projects`、`current`、`create`、`get`、`remove`、`startStage`、`confirmStage`、`addAsset`、`removeAsset`、`setCover`。

- [x] **Step 1: Implement store**

- 初始化从 `localStorage.getItem('vibebiz:clone:projects:v1')` 读取。
- 每次变更调用 `persist()`。
- `create` 生成 `crypto.randomUUID()`，未指定主图时取第一张图片。
- `startStage` 设置 `startedAt` 并写入 `mockStageContent`。
- `confirmStage` 调用 `advanceCloneStage`。
- `removeAsset` 用 `nextCoverAfterRemoval` 回退主图。

- [x] **Step 2: Run build to verify types**

```bash
cd packages/web && npm run build
```

---

### Task 3: 附件与创建对话框

**Files:**
- Create: `packages/web/src/components/clone/CloneAssetField.vue`
- Create: `packages/web/src/components/clone/CloneCreateDialog.vue`

**Interfaces:**
- `CloneAssetField` props：`images: CloneAsset[]`、`coverAssetId: string | null`、`unitTest: CloneAsset | null`、`docs: CloneAsset | null`。
- emits：`add-images(files: File[])`、`remove(id)`、`set-cover(id)`、`set-unit-test(file)`、`set-docs(file)`。
- `CloneCreateDialog` props：`visible: boolean`。emits：`update:visible`、`submit({ draft, assets })`。

- [x] **Step 1: Build asset field**

- 隐藏 `input[type=file]`，图片支持多选与拖拽。
- 九宫格缩略图；主图角标；删除按钮；点击缩略图设为主图。
- zip/文档行显示名称、大小；已有文件时按钮禁用。
- `onBeforeUnmount` 释放所有 object URL。

- [x] **Step 2: Build dialog**

- PrimeVue `Dialog`，字段 `InputText`、`Textarea`。
- 使用 `validateCloneDraft` 与 `validateCloneAsset`。
- 提交前生成 `CloneAsset`，图片 `role='image'`，zip `role='unit_test'`，文档 `role='docs'`。
- 提交后 emit，父组件创建成功后关闭。

- [x] **Step 3: Run build**

```bash
cd packages/web && npm run build
```

---

### Task 4: 克隆页与编辑器

**Files:**
- Create: `packages/web/src/components/clone/ClonePanel.vue`
- Create: `packages/web/src/views/CloneEditorView.vue`

**Interfaces:**
- `ClonePanel` 使用 store，创建成功 `router.push({ name: 'clone-editor', params: { id } })`。
- `CloneEditorView` 从 route 读取 id，找不到时跳回 `/clones`。

- [x] **Step 1: ClonePanel**

- 空态居中标题、说明、按钮。
- 有数据时左侧列表、右侧详情。
- 详情显示来源 URL、图片/zip/文档摘要、阶段进度。
- 支持删除与打开。
- 创建对话框接入 store。

- [x] **Step 2: CloneEditorView**

- 复用 `EditorHeader`，返回 `/clones`。
- 阶段条复制 `enterprise-stage-*` 视觉，显示 5 段状态。
- 左侧 mock 对话区：阶段说明 + 本地输入框 + 发送后追加消息。
- 右侧：
  - 文档阶段用 `DocumentWorkspace` 渲染 `project.contents[key]`。
  - 测试阶段显示方案/报告页签、单测文件、模拟运行按钮与日志。
  - 开发阶段显示预览/代码/控制台三个 mock 页签。
  - 制品阶段显示构建步骤与完成态。
- 底部/头部操作：开始阶段、确认并继续；已完成阶段可回看。

- [x] **Step 3: Run build**

```bash
cd packages/web && npm run build
```

---

### Task 5: 导航、路由、HomeView 与样式

**Files:**
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/views/HomeView.vue`
- Create: `packages/web/src/styles/clone.css`

**Interfaces:**
- 新增路由：`{ path: '/clones', name: 'clones', component: HomeView }`、`{ path: '/clone/editor/:id', name: 'clone-editor', component: CloneEditorView }`。
- 侧栏 navigation 增加 `{ label: '克隆', icon: 'pi pi-copy', route: 'clones' }`。
- `HomeView` 增加 `isClonesPage`，渲染 `ClonePanel`。
- `afterEach` 标题增加克隆页与克隆编辑器。

- [x] **Step 1: Wire navigation and routes**

- [x] **Step 2: Add clone styles**

覆盖空态、列表、详情、对话框、资产网格、阶段条、mock 工作区、移动端断点。
只使用 `--ink`、`--text`、`--muted`、`--line`、`--line-strong`、`--surface`、`--canvas`、`--soft`、`--blue`、`--blue-soft`。

- [x] **Step 3: Run tests/build/lint**

```bash
cd packages/web && npm run test
cd packages/web && npm run build
cd packages/web && npm run lint
```

---

### Task 6: 验证与视觉检查

**Files:**
- 无新增文件。

- [x] **Step 1: Full validation**

```bash
make validate
```

- [x] **Step 2: Browser smoke**

启动 `make dev-web`，检查：
- 侧栏「克隆」进入空态。
- 创建弹窗必填、9 图上限、主图切换、zip/文档。
- 创建后进入编辑器，逐步推进 5 阶段。
- 刷新后项目和阶段保留。
- 960px / 640px 断点无溢出。

- [x] **Step 3: Checkpoint**

```bash
make checkpoint
```

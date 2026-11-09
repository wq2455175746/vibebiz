# 超级团队真实项目复用 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把超级团队三步从独立演示页改为真实复用平台项目、企业级项目、定时任务的后端与页面逻辑。

**Architecture:** `SuperTeamPanel` 通过现有 CP API 创建/打开三类项目，并把项目 ID / 任务 ID 绑定到 Pinia + localStorage；在复用的 `EditorView`、`EnterpriseEditorView`、`ScheduledTasksPanel` 顶部注入 `SuperTeamContextBar`，负责返回与完成阶段。删除三个演示详情页。

**Tech Stack:** Vue 3、Vue Router、Pinia、PrimeVue、现有 CP API（`/projects`、`/projects/:id/stages`、`/scheduled-tasks`）、Vitest。

**Spec:** `docs/superpowers/specs/2026-11-09-super-team-design.md`

## Global Constraints

- 仅修改 `packages/web` 与必要文档；不新增后端接口、不新增项目类型。
- 不新增 npm 依赖。
- 事件源配置继续用 localStorage；步骤绑定也用 localStorage，后续替换为 `superteam_pipeline` 表。
- 颜色仅黑白灰蓝。
- 提交仅在用户明确要求时执行。

---

### Task 1: 类型与工具函数增加绑定与校验

**Files:**
- Modify: `packages/web/src/types/superTeam.ts`
- Modify: `packages/web/src/utils/superTeam.ts`
- Test: `packages/web/src/utils/superTeam.test.ts`

**Interfaces:**
- Produces: `SuperTeamBindings`、`SuperTeamState.bindings`、`defaultBindings()`、`reconcileSteps(steps, bindings, projectIds, taskIds)`。
- `reconcileSteps` 返回 `{ steps: SuperStepState[]; bindings: SuperTeamBindings }`。

- [ ] **Step 1: 写失败测试**

在 `packages/web/src/utils/superTeam.test.ts` 追加：

```ts
import {
  defaultBindings,
  reconcileSteps,
} from './superTeam'

it('starts with empty bindings', () => {
  expect(initialState().bindings).toEqual(defaultBindings())
})

it('merges bindings from storage', () => {
  const state = loadState(
    JSON.stringify({ bindings: { requirementsProjectId: 'p1', operationsTaskId: 9 } }),
  )
  expect(state.bindings.requirementsProjectId).toBe('p1')
  expect(state.bindings.operationsTaskId).toBe(9)
  expect(state.bindings.developmentProjectId).toBeNull()
})

it('clears missing bindings and rolls back steps', () => {
  const steps = advanceStep(defaultSteps(), 'requirements')
  const bindings = { ...defaultBindings(), requirementsProjectId: 'missing' }
  const result = reconcileSteps(steps, bindings, new Set<string>(), new Set<number>())
  expect(result.bindings.requirementsProjectId).toBeNull()
  expect(result.steps.find((step) => step.key === 'requirements')?.status).toBe('ready')
  expect(result.steps.find((step) => step.key === 'development')?.status).toBe('locked')
})

it('marks a ready step active when its binding exists', () => {
  const steps = [{ ...defaultSteps()[0], status: 'ready' as const }, ...defaultSteps().slice(1)]
  const bindings = { ...defaultBindings(), requirementsProjectId: 'p1' }
  const result = reconcileSteps(steps, bindings, new Set(['p1']), new Set<number>())
  expect(result.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npx vitest run src/utils/superTeam.test.ts`
Expected: FAIL — `defaultBindings` / `reconcileSteps` 未导出。

- [ ] **Step 3: 改类型**

在 `packages/web/src/types/superTeam.ts` 增加：

```ts
export interface SuperTeamBindings {
  requirementsProjectId: string | null
  developmentProjectId: string | null
  operationsTaskId: number | null
  operationsProjectId: string | null
}
```

并给 `SuperTeamState` 增加 `bindings: SuperTeamBindings`。

- [ ] **Step 4: 改 utils**

在 `packages/web/src/utils/superTeam.ts`：

```ts
export function defaultBindings(): SuperTeamBindings {
  return {
    requirementsProjectId: null,
    developmentProjectId: null,
    operationsTaskId: null,
    operationsProjectId: null,
  }
}
```

`initialState()` 增加 `bindings: defaultBindings()`。

`loadState` 增加：

```ts
function mergeBindings(value: unknown, fallback: SuperTeamBindings): SuperTeamBindings {
  if (!value || typeof value !== 'object') {
    return fallback
  }
  const stored = value as Partial<SuperTeamBindings>
  return {
    requirementsProjectId:
      typeof stored.requirementsProjectId === 'string' ? stored.requirementsProjectId : null,
    developmentProjectId:
      typeof stored.developmentProjectId === 'string' ? stored.developmentProjectId : null,
    operationsTaskId: typeof stored.operationsTaskId === 'number' ? stored.operationsTaskId : null,
    operationsProjectId:
      typeof stored.operationsProjectId === 'string' ? stored.operationsProjectId : null,
  }
}
```

返回值增加 `bindings: mergeBindings(parsed.bindings, base.bindings)`。

新增：

```ts
export function reconcileSteps(
  steps: SuperStepState[],
  bindings: SuperTeamBindings,
  projectIds: ReadonlySet<string>,
  taskIds: ReadonlySet<number>,
): { steps: SuperStepState[]; bindings: SuperTeamBindings } {
  const nextBindings = { ...bindings }
  if (nextBindings.requirementsProjectId && !projectIds.has(nextBindings.requirementsProjectId)) {
    nextBindings.requirementsProjectId = null
  }
  if (nextBindings.developmentProjectId && !projectIds.has(nextBindings.developmentProjectId)) {
    nextBindings.developmentProjectId = null
  }
  if (nextBindings.operationsTaskId !== null && !taskIds.has(nextBindings.operationsTaskId)) {
    nextBindings.operationsTaskId = null
    nextBindings.operationsProjectId = null
  }

  const requirementsDone =
    steps.find((step) => step.key === 'requirements')?.status === 'done' &&
    Boolean(nextBindings.requirementsProjectId)
  const developmentDone =
    requirementsDone &&
    steps.find((step) => step.key === 'development')?.status === 'done' &&
    Boolean(nextBindings.developmentProjectId)

  function setStatus(step: SuperStepState, status: SuperStepStatus): SuperStepState {
    if (step.status === status) {
      return step
    }
    return {
      ...step,
      status,
      progress: status === 'done' ? 100 : status === 'ready' ? 0 : step.progress,
      updatedAt: new Date().toISOString(),
    }
  }

  const nextSteps = steps.map((step) => {
    if (step.key === 'requirements') {
      if (!nextBindings.requirementsProjectId) {
        return step.status === 'done' || step.status === 'active' ? setStatus(step, 'ready') : step
      }
      return step.status === 'ready' ? setStatus(step, 'active') : step
    }
    if (step.key === 'development') {
      if (!requirementsDone) {
        return setStatus(step, 'locked')
      }
      if (!nextBindings.developmentProjectId) {
        return step.status === 'done' || step.status === 'active' ? setStatus(step, 'ready') : step
      }
      return step.status === 'ready' ? setStatus(step, 'active') : step
    }
    if (!developmentDone) {
      return setStatus(step, 'locked')
    }
    if (nextBindings.operationsTaskId === null) {
      return step.status === 'done' || step.status === 'active' ? setStatus(step, 'ready') : step
    }
    return step.status === 'ready' ? setStatus(step, 'active') : step
  })

  return { steps: nextSteps, bindings: nextBindings }
}
```

- [ ] **Step 5: 运行测试确认通过**

Run: `cd packages/web && npx vitest run src/utils/superTeam.test.ts`
Expected: PASS。

---

### Task 2: store 增加绑定与同步

**Files:**
- Modify: `packages/web/src/stores/superTeam.ts`
- Test: `packages/web/src/stores/superTeam.test.ts`

**Interfaces:**
- Produces: `store.bindings`、`store.setBinding(field, value)`、`store.reconcile(projects, tasks)`、`store.completeStep(key)`。

- [ ] **Step 1: 写失败测试**

追加：

```ts
import type { Project, ScheduledTask } from '@/types'

function project(id: string): Project {
  return {
    id,
    name: id,
    prompt: '',
    previewPath: '',
    state: 'ready',
    createdAt: '2026-11-09T00:00:00Z',
    updatedAt: '2026-11-09T00:00:00Z',
  }
}

function task(id: number, projectId: string): ScheduledTask {
  return {
    id,
    name: 'ops',
    description: '',
    scheduleType: 'cron',
    time: '',
    weekdays: [],
    cronExpression: '0 * * * *',
    timezone: 'Asia/Shanghai',
    source: 'manual',
    enabled: true,
    projectId,
    lastStatus: 'pending',
    lastError: '',
    runCount: 0,
  }
}

it('stores bindings and marks the step active', () => {
  const store = useSuperTeamStore()
  store.setBinding('requirementsProjectId', 'p1')
  expect(store.bindings.requirementsProjectId).toBe('p1')
  expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
})

it('reconciles missing projects and tasks', () => {
  const store = useSuperTeamStore()
  store.setBinding('requirementsProjectId', 'p1')
  store.reconcile([project('p1')], [])
  expect(store.bindings.requirementsProjectId).toBe('p1')
  store.reconcile([], [task(9, 'ops-project')])
  expect(store.bindings.requirementsProjectId).toBeNull()
  expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('ready')
})
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd packages/web && npx vitest run src/stores/superTeam.test.ts`
Expected: FAIL。

- [ ] **Step 3: 实现 store**

- 引入 `reconcileSteps`、`defaultBindings`、`SuperTeamBindings`、`Project`、`ScheduledTask`。
- `const bindings = ref<SuperTeamBindings>(initial.bindings)`。
- `persist` 写入 `bindings`。
- 新增：

```ts
function setBinding<K extends keyof SuperTeamBindings>(
  key: K,
  value: SuperTeamBindings[K],
): void {
  bindings.value = { ...bindings.value, [key]: value }
  const stepKey: SuperStepKey =
    key === 'requirementsProjectId'
      ? 'requirements'
      : key === 'developmentProjectId'
        ? 'development'
        : 'operations'
  steps.value = steps.value.map((step) =>
    step.key === stepKey && step.status === 'ready' ? { ...step, status: 'active' } : step,
  )
  persist()
}

function reconcile(projects: Project[], tasks: ScheduledTask[]): void {
  const projectIds = new Set(projects.map((item) => item.id))
  const taskIds = new Set(tasks.map((item) => item.id))
  const result = reconcileSteps(steps.value, bindings.value, projectIds, taskIds)
  steps.value = result.steps
  bindings.value = result.bindings
  persist()
}

function completeStep(key: SuperStepKey): void {
  advance(key)
}
```

- `reset()` 同时恢复 `bindings`。
- 返回对象增加 `bindings`、`setBinding`、`reconcile`、`completeStep`。

- [ ] **Step 4: 运行测试确认通过**

Run: `cd packages/web && npx vitest run src/stores/superTeam.test.ts`
Expected: PASS。

---

### Task 3: 超级团队首页接入真实项目

**Files:**
- Modify: `packages/web/src/components/superteam/SuperTeamPanel.vue`
- Modify: `packages/web/src/components/superteam/StepCard.vue`

**Interfaces:**
- Consumes: `createProject`、`listProjects`、`createScheduledTask`、`listScheduledTasks`、`useAgentRolesStore`、`markEnterpriseProject`、`store.setBinding`、`store.reconcile`。
- Produces: 点击步骤创建真实项目 / 定时任务并跳转。

- [ ] **Step 1: 改 StepCard 增加 busy**

`props` 增加 `busy?: boolean`；按钮 `:disabled="locked || busy"`；文案在 busy 时显示 `处理中…`。

- [ ] **Step 2: 改 SuperTeamPanel**

引入：

```ts
import { createProject, listProjects } from '@/api/projects'
import { createScheduledTask, listScheduledTasks } from '@/api/scheduledTasks'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { markEnterpriseProject } from '@/utils/enterprise'
import type { Project, ScheduledTask } from '@/types'
```

常量：

```ts
const REQUIREMENTS_PROMPT =
  '你是超级团队的需求挖掘 Agent。基于已接入的飞书、Webhook、Email 事件，生成需求思维导图、知识图谱与经济可行性分析文档，并写入当前项目工作区。'
const DEVELOPMENT_PROMPT =
  '你是超级团队的系统开发 Agent。请基于需求挖掘产物，按企业七阶段推进需求梳理、概要设计、详细设计、测试用例、全栈开发、制品与交付。'
const OPERATIONS_TASK = {
  name: '超级团队观测运维',
  description: '每小时巡检超级团队事件与指标，生成可预览的数据图表与指标报表。',
  scheduleType: 'cron' as const,
  time: '',
  weekdays: [],
  cronExpression: '0 * * * *',
  timezone: 'Asia/Shanghai',
  source: 'manual' as const,
}
```

状态与加载：

```ts
const agentRoles = useAgentRolesStore()
const projects = ref<Project[]>([])
const tasks = ref<ScheduledTask[]>([])
const pending = ref<SuperStepKey | null>(null)
const actionError = ref('')

async function loadBindings(): Promise<void> {
  const [projectResult, taskResult] = await Promise.allSettled([
    listProjects(),
    listScheduledTasks(),
  ])
  projects.value = projectResult.status === 'fulfilled' ? projectResult.value : []
  tasks.value = taskResult.status === 'fulfilled' ? taskResult.value : []
  store.reconcile(projects.value, tasks.value)
}

onMounted(loadBindings)
```

替换 `openStep` 为 `startStep`：

```ts
async function startStep(key: SuperStepKey): Promise<void> {
  const step = store.steps.find((item) => item.key === key)
  if (!step || step.status === 'locked' || pending.value) {
    return
  }
  pending.value = key
  actionError.value = ''
  try {
    if (key === 'requirements') {
      let id = store.bindings.requirementsProjectId
      if (!id) {
        const project = await createProject(REQUIREMENTS_PROMPT)
        id = project.id
        store.setBinding('requirementsProjectId', id)
      }
      await router.push({
        name: 'editor',
        params: { id },
        query: { from: 'super-team', step: 'requirements' },
      })
      return
    }
    if (key === 'development') {
      let id = store.bindings.developmentProjectId
      if (!id) {
        await agentRoles.load()
        const team = agentRoles.team
        if (!team) {
          actionError.value = '缺少团队角色，无法创建企业级项目'
          return
        }
        const project = await createProject(DEVELOPMENT_PROMPT, [team.id], 'enterprise')
        markEnterpriseProject(project.id)
        id = project.id
        store.setBinding('developmentProjectId', id)
      }
      await router.push({
        name: 'enterprise-editor',
        params: { id },
        query: { from: 'super-team', step: 'development' },
      })
      return
    }
    let task = tasks.value.find((item) => item.id === store.bindings.operationsTaskId)
    if (!task) {
      task = await createScheduledTask(OPERATIONS_TASK)
      store.setBinding('operationsTaskId', task.id)
      store.setBinding('operationsProjectId', task.projectId)
      tasks.value = [task, ...tasks.value]
    }
    await router.push({
      name: 'scheduled-tasks',
      query: { from: 'super-team', step: 'operations', taskId: String(task.id) },
    })
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '创建失败'
  } finally {
    pending.value = null
  }
}
```

模板：`@open="openStep(step.key)"` 改为 `@open="startStep(step.key)"`，并传 `:busy="pending === step.key"`；在事件区或步骤区上方显示 `actionError`。

- [ ] **Step 3: 类型检查**

Run: `cd packages/web && npx vue-tsc --noEmit`
Expected: PASS。

---

### Task 4: 上下文条与三个复用页面

**Files:**
- Create: `packages/web/src/components/superteam/SuperTeamContextBar.vue`
- Modify: `packages/web/src/views/EditorView.vue`
- Modify: `packages/web/src/views/EnterpriseEditorView.vue`
- Modify: `packages/web/src/components/scheduled/ScheduledTasksPanel.vue`
- Modify: `packages/web/src/styles/superteam.css`

**Interfaces:**
- Produces: `<SuperTeamContextBar step="requirements|development|operations" />`。
- 读取 `route.query.from === 'super-team'`；`taskId` 选中定时任务。

- [ ] **Step 1: 新建上下文条**

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useSuperTeamStore } from '@/stores/superTeam'
import type { SuperStepKey } from '@/types/superTeam'
import { stepTitle } from '@/utils/superTeam'

const props = defineProps<{ step: SuperStepKey }>()
const route = useRoute()
const router = useRouter()
const store = useSuperTeamStore()

const visible = computed(() => route.query.from === 'super-team')

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.completeStep(props.step)
  back()
}
</script>

<template>
  <div v-if="visible" class="superteam-context-bar">
    <span class="superteam-context-title">
      <i class="pi pi-users" />
      超级团队 · {{ stepTitle(step) }}
    </span>
    <span class="superteam-context-actions">
      <button type="button" class="superteam-context-btn" @click="back">返回超级团队</button>
      <button type="button" class="superteam-context-btn primary" @click="complete">
        完成本阶段
      </button>
    </span>
  </div>
</template>
```

- [ ] **Step 2: 挂到 EditorView / EnterpriseEditorView**

各自引入并在 `EditorHeader` 之后插入：

```vue
<SuperTeamContextBar step="requirements" />
```

（Enterprise 用 `step="development"`。）

- [ ] **Step 3: 挂到 ScheduledTasksPanel 并支持 taskId**

- 引入 `useRoute` 与 `SuperTeamContextBar`。
- 在 `<section class="scheduled-panel">` 内第一行插入 `<SuperTeamContextBar step="operations" />`。
- `load()` 末尾按 `route.query.taskId` 选中：

```ts
const queryTaskId = Number(route.query.taskId ?? 0)
if (queryTaskId && tasks.value.some((task) => task.id === queryTaskId)) {
  selectedId.value = queryTaskId
} else if (!tasks.value.some((task) => task.id === selectedId.value)) {
  selectedId.value = tasks.value[0]?.id ?? 0
}
```

- [ ] **Step 4: 补样式**

在 `packages/web/src/styles/superteam.css` 增加：

```css
.superteam-context-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 10px 16px;
  color: var(--blue);
  background: var(--blue-soft);
  border-bottom: 1px solid rgba(37, 99, 235, 0.3);
  font-size: 12px;
}

.superteam-context-title {
  display: flex;
  align-items: center;
  gap: 6px;
}

.superteam-context-actions {
  display: flex;
  gap: 8px;
}

.superteam-context-btn {
  padding: 6px 12px;
  color: var(--blue);
  background: var(--surface);
  border: 1px solid rgba(37, 99, 235, 0.35);
  border-radius: 999px;
  cursor: pointer;
}

.superteam-context-btn.primary {
  color: var(--surface);
  background: var(--blue);
  border-color: var(--blue);
}
```

- [ ] **Step 5: 类型检查**

Run: `cd packages/web && npx vue-tsc --noEmit`
Expected: PASS。

---

### Task 5: 删除演示详情页与旧路由

**Files:**
- Delete: `packages/web/src/views/SuperTeamRequirementsView.vue`
- Delete: `packages/web/src/views/SuperTeamDevelopmentView.vue`
- Delete: `packages/web/src/views/SuperTeamOperationsView.vue`
- Delete: `packages/web/src/components/superteam/MindMap.vue`
- Delete: `packages/web/src/components/superteam/KnowledgeGraph.vue`
- Modify: `packages/web/src/router/index.ts`
- Modify: `packages/web/src/styles/superteam.css`（删除仅演示页使用的样式）

- [ ] **Step 1: 删除文件**

```bash
rm packages/web/src/views/SuperTeamRequirementsView.vue \
   packages/web/src/views/SuperTeamDevelopmentView.vue \
   packages/web/src/views/SuperTeamOperationsView.vue \
   packages/web/src/components/superteam/MindMap.vue \
   packages/web/src/components/superteam/KnowledgeGraph.vue
```

- [ ] **Step 2: 删除路由与导入**

移除三个 view import 与 `/super-team/requirements|development|operations` 路由；保留 `/super-team`。

- [ ] **Step 3: 精简样式**

移除 `.superteam-editor`、`.superteam-editor-body`、`.superteam-chat*`、`.superteam-workbench`、`.superteam-tabs`、`.superteam-tab*`、`.superteam-footer-action`、`.superteam-primary`、`.super-mindmap*`、`.super-graph*`、`.super-doc*`、`.super-event*`、`.super-stage*`、`.super-inject*`、`.super-chart*`、`.super-task*` 以及对应媒体查询；保留面板、事件卡片、步骤卡、弹窗、上下文条样式。

- [ ] **Step 4: 类型检查与测试**

Run:

```bash
cd packages/web && npx vue-tsc --noEmit
cd packages/web && npm test
```

Expected: PASS。

---

### Task 6: 全量验证与合并

- [ ] **Step 1: 运行质量门禁**

```bash
cd packages/web && npm run lint
cd packages/web && npm run format:check
make validate
make test
```

Expected: PASS。预存在的 prettier 警告不影响 `make quality`；至少保证新增文件无格式问题。

- [ ] **Step 2: 构建**

```bash
make -C packages/web build
```

Expected: PASS。

- [ ] **Step 3: 提交并合并**

```bash
git add <相关文件>
git commit -m "feat(web): reuse platform, enterprise and scheduled projects in super team"
```

随后在 `dev` 工作树合并 `vibebiz-bakwt`，确认无冲突。

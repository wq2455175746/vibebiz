# 企业工程 · 制品空间与交付空间 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 web 增加账号级制品/交付配置，以及企业项目阶段 6/7 的完整前端模拟流水线。

**Architecture:** 全局连接配置和项目流程分别存入两个 Pinia store，持久化到 localStorage；阶段 6/7 由前端 overlay 覆盖后端仍返回的 `placeholder` 状态。页面通过类型化 store 和纯函数工作，后续接后端时替换 store 持久化实现。

**Tech Stack:** Vue 3、TypeScript、Pinia、Vue Router、Vitest、PrimeVue、现有 settings/enterprise CSS。

**Spec:** `docs/superpowers/specs/2026-10-31-enterprise-artifact-delivery-design.md`

## Global Constraints

- 只修改 `packages/web`；不改 CP、DP、sandbox、数据库、API。
- 不提交 git；每个任务结束跑测试并执行 `make checkpoint`。
- 配置为账号级全局配置，每类支持多条。
- 项目流程按项目 ID 隔离，配置与流程均写 localStorage。
- 密码、Token、kubeconfig 列表脱敏；编辑留空不覆盖。
- 阶段 6/7 不调用阶段开始/完成 API。
- 阶段 5 完成后阶段 6 活跃；制品确认后阶段 7 活跃。
- 制品三步严格顺序执行；交付四选一。
- 阶段 6/7 左侧聊天只读。
- 颜色只用黑 / 白 / 灰 / 蓝。
- 不写无意义注释。

---

### Task 1: 连接类型、配置工具和全局配置 Store

**Files:**
- Create: `packages/web/src/types/enterpriseSpace.ts`
- Create: `packages/web/src/utils/enterpriseConnections.ts`
- Create: `packages/web/src/utils/enterpriseConnections.test.ts`
- Create: `packages/web/src/stores/enterpriseConnections.ts`
- Create: `packages/web/src/stores/enterpriseConnections.test.ts`

**Interfaces:**
- Consumes: Pinia、localStorage、`crypto.randomUUID()`。
- Produces:
  - `EnterpriseConnection` 及各类连接类型
  - `ArtifactConnectionInput`、`DeliveryConnectionInput`
  - `CONNECTIONS_STORAGE_KEY`
  - `loadConnections()`、`saveConnections()`
  - `connectionDescription()`、`maskedCredential()`
  - `useEnterpriseConnectionsStore()`，暴露 `codeRepos`、`harbors`、`jenkins`、`k8sClusters`、`cloudNodes`、`argoes`、`fdes`、`create`、`update`、`remove`

- [ ] **Step 1: 写连接类型**

创建 `packages/web/src/types/enterpriseSpace.ts`：

```ts
export type CodeRepoProvider = 'github' | 'gitlab'
export type ArtifactConnectionKind = 'code-repo' | 'harbor' | 'jenkins'
export type DeliveryConnectionKind = 'k8s' | 'cloud' | 'argo' | 'fde'
export type ConnectionKind = ArtifactConnectionKind | DeliveryConnectionKind

export interface ConnectionBase {
  id: string
  name: string
  createdAt: string
  updatedAt: string
}

export interface CodeRepoConnection extends ConnectionBase {
  kind: 'code-repo'
  provider: CodeRepoProvider
  url: string
  username: string
  secret: string
  defaultBranch: string
}

export interface HarborConnection extends ConnectionBase {
  kind: 'harbor'
  registryUrl: string
  namespace: string
  username: string
  secret: string
}

export interface JenkinsConnection extends ConnectionBase {
  kind: 'jenkins'
  url: string
  username: string
  secret: string
  jobs: string[]
}

export interface K8sConnection extends ConnectionBase {
  kind: 'k8s'
  apiServer: string
  kubeconfig: string
  defaultNamespace: string
}

export interface CloudConnection extends ConnectionBase {
  kind: 'cloud'
  host: string
  port: number
  username: string
  secret: string
}

export interface ArgoConnection extends ConnectionBase {
  kind: 'argo'
  url: string
  username: string
  secret: string
}

export interface FdeConnection extends ConnectionBase {
  kind: 'fde'
  contact: string
  note: string
}

export type ArtifactConnection = CodeRepoConnection | HarborConnection | JenkinsConnection
export type DeliveryConnection = K8sConnection | CloudConnection | ArgoConnection | FdeConnection
export type EnterpriseConnection = ArtifactConnection | DeliveryConnection

export type ArtifactConnectionInput = Omit<
  CodeRepoConnection,
  'id' | 'createdAt' | 'updatedAt'
> | Omit<HarborConnection, 'id' | 'createdAt' | 'updatedAt'> | Omit<
  JenkinsConnection,
  'id' | 'createdAt' | 'updatedAt'
>

export type DeliveryConnectionInput = Omit<
  K8sConnection,
  'id' | 'createdAt' | 'updatedAt'
> | Omit<CloudConnection, 'id' | 'createdAt' | 'updatedAt'> | Omit<
  ArgoConnection,
  'id' | 'createdAt' | 'updatedAt'
> | Omit<FdeConnection, 'id' | 'createdAt' | 'updatedAt'>

export type MockStepStatus = 'idle' | 'running' | 'success' | 'error'

export interface MockStepState {
  status: MockStepStatus
  startedAt: string | null
  completedAt: string | null
  error: string
}

export type ArtifactStepKey = 'code' | 'jenkins' | 'harbor'

export interface ArtifactFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  codeRepoId: string
  branch: string
  commitMessage: string
  code: MockStepState
  jenkinsId: string
  jenkinsJob: string
  jenkins: MockStepState
  harborId: string
  image: string
  tag: string
  harbor: MockStepState
}

export interface DeliveryFlowState {
  status: 'locked' | 'active' | 'done'
  startedAt: string | null
  completedAt: string | null
  method: DeliveryConnectionKind | null
  targetId: string
  namespace: string
  note: string
  execution: MockStepState
}

export interface EnterpriseFlow {
  projectId: string
  artifact: ArtifactFlowState
  delivery: DeliveryFlowState
  updatedAt: string
}
```

- [ ] **Step 2: 写配置工具失败测试**

创建 `packages/web/src/utils/enterpriseConnections.test.ts`：

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import type { CodeRepoConnection, HarborConnection } from '@/types/enterpriseSpace'
import {
  CONNECTIONS_STORAGE_KEY,
  connectionDescription,
  loadConnections,
  maskedCredential,
  saveConnections,
} from './enterpriseConnections'

const repo: CodeRepoConnection = {
  id: 'r1',
  kind: 'code-repo',
  name: '主仓库',
  provider: 'github',
  url: 'https://github.com/acme/app',
  username: 'acme',
  secret: 'token',
  defaultBranch: 'main',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

const harbor: HarborConnection = {
  id: 'h1',
  kind: 'harbor',
  name: '主镜像库',
  registryUrl: 'https://harbor.example.com/',
  namespace: 'vibebiz',
  username: 'robot',
  secret: 'password',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

beforeEach(() => localStorage.clear())

describe('enterprise connections storage', () => {
  it('round-trips connections', () => {
    saveConnections([repo, harbor])
    expect(loadConnections()).toEqual([repo, harbor])
  })

  it('falls back to empty data for invalid JSON or version', () => {
    localStorage.setItem(CONNECTIONS_STORAGE_KEY, '{bad')
    expect(loadConnections()).toEqual([])
    localStorage.setItem(CONNECTIONS_STORAGE_KEY, JSON.stringify({ version: 2, connections: [repo] }))
    expect(loadConnections()).toEqual([])
  })

  it('describes connections without exposing secrets', () => {
    expect(connectionDescription(repo)).toContain('github')
    expect(connectionDescription(repo)).not.toContain('token')
    expect(connectionDescription(harbor)).toContain('harbor.example.com/vibebiz')
    expect(maskedCredential('secret')).toBe('••••••••')
    expect(maskedCredential('')).toBe('未配置')
  })
})
```

- [ ] **Step 3: 运行测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/utils/enterpriseConnections.test.ts
```

Expected: FAIL，模块不存在。

- [ ] **Step 4: 实现配置工具**

创建 `packages/web/src/utils/enterpriseConnections.ts`：

```ts
import type {
  CodeRepoConnection,
  EnterpriseConnection,
  HarborConnection,
  JenkinsConnection,
} from '@/types/enterpriseSpace'

export const CONNECTIONS_STORAGE_KEY = 'vibebiz:enterprise:connections:v1'

interface PersistedConnections {
  version: 1
  connections: EnterpriseConnection[]
}

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>

function isConnection(value: unknown): value is EnterpriseConnection {
  if (!value || typeof value !== 'object') return false
  const item = value as Partial<EnterpriseConnection>
  return typeof item.id === 'string' && typeof item.name === 'string' && typeof item.kind === 'string'
}

export function loadConnections(storage: ReadStorage = localStorage): EnterpriseConnection[] {
  try {
    const raw = storage.getItem(CONNECTIONS_STORAGE_KEY)
    if (!raw) return []
    const parsed = JSON.parse(raw) as Partial<PersistedConnections>
    if (parsed.version !== 1 || !Array.isArray(parsed.connections)) return []
    return parsed.connections.filter(isConnection)
  } catch {
    return []
  }
}

export function saveConnections(
  connections: EnterpriseConnection[],
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(CONNECTIONS_STORAGE_KEY, JSON.stringify({ version: 1, connections }))
}

export function maskedCredential(value: string): string {
  return value ? '••••••••' : '未配置'
}

export function connectionDescription(connection: EnterpriseConnection): string {
  switch (connection.kind) {
    case 'code-repo':
      return `${connection.provider.toUpperCase()} · ${connection.url}`
    case 'harbor':
      return `${connection.registryUrl.replace(/\/+$/, '')}/${connection.namespace}`
    case 'jenkins':
      return `${connection.url} · ${connection.jobs.length} 个任务`
    case 'k8s':
      return `${connection.apiServer} · ${connection.defaultNamespace || 'default'}`
    case 'cloud':
      return `${connection.username}@${connection.host}:${connection.port}`
    case 'argo':
      return connection.url
    case 'fde':
      return connection.contact
  }
}

export function codeRepoTarget(connection: CodeRepoConnection): string {
  return `${connection.url} · ${connection.defaultBranch}`
}

export function harborTarget(connection: HarborConnection): string {
  return `${connection.registryUrl.replace(/\/+$/, '')}/${connection.namespace}`
}

export function jenkinsTarget(connection: JenkinsConnection): string {
  return `${connection.url} · ${connection.jobs.join(', ')}`
}
```

- [ ] **Step 5: 写 Store 失败测试**

创建 `packages/web/src/stores/enterpriseConnections.test.ts`：

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { CodeRepoConnection } from '@/types/enterpriseSpace'
import { useEnterpriseConnectionsStore } from './enterpriseConnections'

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})

describe('useEnterpriseConnectionsStore', () => {
  it('creates, updates and removes a connection', () => {
    const store = useEnterpriseConnectionsStore()
    const created = store.create({
      kind: 'code-repo',
      name: '主仓库',
      provider: 'github',
      url: 'https://github.com/acme/app',
      username: 'acme',
      secret: 'token',
      defaultBranch: 'main',
    }) as CodeRepoConnection

    expect(store.codeRepos).toHaveLength(1)
    expect(created.id).toBeTruthy()

    store.update({ ...created, name: '改名仓库' })
    expect(store.codeRepos[0].name).toBe('改名仓库')

    store.remove(created.id)
    expect(store.codeRepos).toHaveLength(0)
  })

  it('keeps an existing secret when the edit value is empty', () => {
    const store = useEnterpriseConnectionsStore()
    const created = store.create({
      kind: 'code-repo',
      name: '主仓库',
      provider: 'gitlab',
      url: 'https://gitlab.com/acme/app',
      username: 'acme',
      secret: 'token',
      defaultBranch: 'main',
    }) as CodeRepoConnection

    store.update({ ...created, secret: '' })
    expect(store.codeRepos[0].secret).toBe('token')
  })

  it('loads persisted connections on a new store instance', () => {
    const first = useEnterpriseConnectionsStore()
    first.create({
      kind: 'harbor',
      name: '镜像库',
      registryUrl: 'https://harbor.example.com',
      namespace: 'vibebiz',
      username: 'robot',
      secret: 'password',
    })

    setActivePinia(createPinia())
    const second = useEnterpriseConnectionsStore()
    expect(second.harbors).toHaveLength(1)
  })
})
```

- [ ] **Step 6: 运行 Store 测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/stores/enterpriseConnections.test.ts
```

Expected: FAIL，store 不存在。

- [ ] **Step 7: 实现 Store**

创建 `packages/web/src/stores/enterpriseConnections.ts`：

```ts
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  ArgoConnection,
  ArtifactConnectionInput,
  CloudConnection,
  CodeRepoConnection,
  DeliveryConnectionInput,
  EnterpriseConnection,
  FdeConnection,
  HarborConnection,
  JenkinsConnection,
  K8sConnection,
} from '@/types/enterpriseSpace'
import { loadConnections, saveConnections } from '@/utils/enterpriseConnections'

export const useEnterpriseConnectionsStore = defineStore('enterpriseConnections', () => {
  const connections = ref<EnterpriseConnection[]>(loadConnections())

  const codeRepos = computed(() =>
    connections.value.filter((item): item is CodeRepoConnection => item.kind === 'code-repo'),
  )
  const harbors = computed(() =>
    connections.value.filter((item): item is HarborConnection => item.kind === 'harbor'),
  )
  const jenkins = computed(() =>
    connections.value.filter((item): item is JenkinsConnection => item.kind === 'jenkins'),
  )
  const k8sClusters = computed(() =>
    connections.value.filter((item): item is K8sConnection => item.kind === 'k8s'),
  )
  const cloudNodes = computed(() =>
    connections.value.filter((item): item is CloudConnection => item.kind === 'cloud'),
  )
  const argoes = computed(() =>
    connections.value.filter((item): item is ArgoConnection => item.kind === 'argo'),
  )
  const fdes = computed(() =>
    connections.value.filter((item): item is FdeConnection => item.kind === 'fde'),
  )

  function persist(): void {
    saveConnections(connections.value)
  }

  function create(input: ArtifactConnectionInput | DeliveryConnectionInput): EnterpriseConnection {
    const now = new Date().toISOString()
    const connection = {
      ...input,
      id: crypto.randomUUID(),
      createdAt: now,
      updatedAt: now,
    } as EnterpriseConnection
    connections.value = [...connections.value, connection]
    persist()
    return connection
  }

  function update(input: EnterpriseConnection): void {
    const index = connections.value.findIndex((item) => item.id === input.id)
    if (index < 0) return
    const previous = connections.value[index]
    const next = { ...input, updatedAt: new Date().toISOString() } as EnterpriseConnection
    if ('secret' in next && 'secret' in previous && !next.secret) {
      next.secret = previous.secret
    }
    if ('kubeconfig' in next && 'kubeconfig' in previous && !next.kubeconfig) {
      next.kubeconfig = previous.kubeconfig
    }
    const copy = [...connections.value]
    copy[index] = next
    connections.value = copy
    persist()
  }

  function remove(id: string): void {
    connections.value = connections.value.filter((item) => item.id !== id)
    persist()
  }

  return {
    connections,
    codeRepos,
    harbors,
    jenkins,
    k8sClusters,
    cloudNodes,
    argoes,
    fdes,
    create,
    update,
    remove,
  }
})
```

- [ ] **Step 8: 运行测试**

```bash
npm --prefix packages/web run test -- --run src/utils/enterpriseConnections.test.ts src/stores/enterpriseConnections.test.ts
npm --prefix packages/web run lint
```

Expected: 全部通过。

- [ ] **Step 9: Checkpoint**

```bash
make checkpoint
```

---

### Task 2: 流程类型、overlay 工具和项目流程 Store

**Files:**
- Modify: `packages/web/src/types/enterpriseSpace.ts`
- Create: `packages/web/src/utils/enterpriseFlow.ts`
- Create: `packages/web/src/utils/enterpriseFlow.test.ts`
- Create: `packages/web/src/stores/enterpriseFlow.ts`
- Create: `packages/web/src/stores/enterpriseFlow.test.ts`

**Interfaces:**
- Consumes: Task 1 的类型；`EnterpriseStageStateMap`。
- Produces:
  - `createEmptyStep()`
  - `createArtifactFlow()`、`createDeliveryFlow()`、`createEnterpriseFlow(projectId)`
  - `loadFlows()`、`saveFlows()`
  - `artifactStepUnlocked()`、`artifactReady()`、`deliveryReady()`
  - `dockerPullCommand()`
  - `mergeEnterpriseStageStates()`
  - `downloadTextFile()`
  - `useEnterpriseFlowStore()`，暴露 `get`、`ensureArtifact`、`patchArtifact`、`patchArtifactStep`、`confirmArtifact`、`patchDelivery`、`patchDeliveryExecution`、`confirmDelivery`

- [ ] **Step 1: 在类型文件补充流程工具依赖**

确认 Task 1 的 `enterpriseSpace.ts` 已包含 `MockStepState`、`ArtifactFlowState`、`DeliveryFlowState`、`EnterpriseFlow`。若 Task 1 未包含，按规格第 5.2 节补齐。

- [ ] **Step 2: 写 flow 工具失败测试**

创建 `packages/web/src/utils/enterpriseFlow.test.ts`：

```ts
import { beforeEach, describe, expect, it } from 'vitest'

import type { HarborConnection } from '@/types/enterpriseSpace'
import { createInitialStageState } from './enterprise'
import {
  FLOWS_STORAGE_KEY,
  artifactReady,
  artifactStepUnlocked,
  createArtifactFlow,
  createDeliveryFlow,
  createEnterpriseFlow,
  dockerPullCommand,
  loadFlows,
  mergeEnterpriseStageStates,
  saveFlows,
} from './enterpriseFlow'

const harbor: HarborConnection = {
  id: 'h1',
  kind: 'harbor',
  name: '镜像库',
  registryUrl: 'https://harbor.example.com/',
  namespace: 'vibebiz',
  username: 'robot',
  secret: 'password',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

beforeEach(() => localStorage.clear())

describe('enterprise flow utilities', () => {
  it('unlocks artifact steps in order', () => {
    const flow = createArtifactFlow(true)
    expect(artifactStepUnlocked(flow, 'code')).toBe(true)
    expect(artifactStepUnlocked(flow, 'jenkins')).toBe(false)

    flow.code = { ...flow.code, status: 'success' }
    expect(artifactStepUnlocked(flow, 'jenkins')).toBe(true)
    expect(artifactStepUnlocked(flow, 'harbor')).toBe(false)

    flow.jenkins = { ...flow.jenkins, status: 'success' }
    expect(artifactStepUnlocked(flow, 'harbor')).toBe(true)
  })

  it('requires all three steps before artifact confirmation', () => {
    const flow = createArtifactFlow(true)
    expect(artifactReady(flow)).toBe(false)
    flow.code = { ...flow.code, status: 'success' }
    flow.jenkins = { ...flow.jenkins, status: 'success' }
    expect(artifactReady(flow)).toBe(false)
    flow.harbor = { ...flow.harbor, status: 'success' }
    expect(artifactReady(flow)).toBe(true)
  })

  it('builds a docker pull command without duplicate slashes', () => {
    const flow = createArtifactFlow(true)
    flow.image = 'vibebiz-app'
    flow.tag = 'v1'
    expect(dockerPullCommand(flow, harbor)).toBe(
      'https://harbor.example.com/vibebiz/vibebiz-app:v1',
    )
  })

  it('overlays stages 6/7 only after development is done', () => {
    const states = createInitialStageState()
    const flow = createEnterpriseFlow('p1')
    flow.artifact = { ...flow.artifact, status: 'active', startedAt: '2026-10-31T00:00:00Z' }

    expect(mergeEnterpriseStageStates(states, flow).artifacts.status).toBe('placeholder')

    states.development.status = 'done'
    const merged = mergeEnterpriseStageStates(states, flow)
    expect(merged.artifacts.status).toBe('active')
    expect(merged.delivery.status).toBe('locked')

    flow.artifact.status = 'done'
    flow.delivery = { ...flow.delivery, status: 'active' }
    const next = mergeEnterpriseStageStates(states, flow)
    expect(next.artifacts.status).toBe('done')
    expect(next.delivery.status).toBe('active')
  })

  it('round-trips flows and falls back to empty data', () => {
    const flow = createEnterpriseFlow('p1')
    saveFlows({ p1: flow })
    expect(loadFlows().p1.projectId).toBe('p1')

    localStorage.setItem(FLOWS_STORAGE_KEY, '{bad')
    expect(loadFlows()).toEqual({})
  })

  it('creates locked delivery state by default', () => {
    expect(createDeliveryFlow().status).toBe('locked')
  })
})
```

- [ ] **Step 3: 运行测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/utils/enterpriseFlow.test.ts
```

Expected: FAIL，`enterpriseFlow.ts` 不存在。

- [ ] **Step 4: 实现 flow 工具**

创建 `packages/web/src/utils/enterpriseFlow.ts`：

```ts
import type {
  ArtifactFlowState,
  DeliveryFlowState,
  EnterpriseFlow,
  HarborConnection,
  MockStepState,
} from '@/types/enterpriseSpace'
import type { EnterpriseStageStateMap } from '@/utils/enterprise'

export const FLOWS_STORAGE_KEY = 'vibebiz:enterprise:flows:v1'

interface PersistedFlows {
  version: 1
  projects: Record<string, EnterpriseFlow>
}

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>

export function createEmptyStep(): MockStepState {
  return { status: 'idle', startedAt: null, completedAt: null, error: '' }
}

export function createArtifactFlow(active = false): ArtifactFlowState {
  return {
    status: active ? 'active' : 'locked',
    startedAt: active ? new Date().toISOString() : null,
    completedAt: null,
    codeRepoId: '',
    branch: '',
    commitMessage: 'chore: enterprise delivery snapshot',
    code: createEmptyStep(),
    jenkinsId: '',
    jenkinsJob: '',
    jenkins: createEmptyStep(),
    harborId: '',
    image: 'vibebiz-app',
    tag: 'latest',
    harbor: createEmptyStep(),
  }
}

export function createDeliveryFlow(): DeliveryFlowState {
  return {
    status: 'locked',
    startedAt: null,
    completedAt: null,
    method: null,
    targetId: '',
    namespace: '',
    note: '',
    execution: createEmptyStep(),
  }
}

export function createEnterpriseFlow(projectId: string): EnterpriseFlow {
  return {
    projectId,
    artifact: createArtifactFlow(),
    delivery: createDeliveryFlow(),
    updatedAt: new Date().toISOString(),
  }
}

export function loadFlows(storage: ReadStorage = localStorage): Record<string, EnterpriseFlow> {
  try {
    const raw = storage.getItem(FLOWS_STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Partial<PersistedFlows>
    if (parsed.version !== 1 || !parsed.projects || typeof parsed.projects !== 'object') return {}
    return parsed.projects
  } catch {
    return {}
  }
}

export function saveFlows(
  flows: Record<string, EnterpriseFlow>,
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(FLOWS_STORAGE_KEY, JSON.stringify({ version: 1, projects: flows }))
}

export function artifactStepUnlocked(
  flow: ArtifactFlowState,
  step: 'code' | 'jenkins' | 'harbor',
): boolean {
  if (flow.status === 'done') return true
  if (flow.status !== 'active') return false
  if (step === 'code') return true
  if (step === 'jenkins') return flow.code.status === 'success'
  return flow.jenkins.status === 'success'
}

export function artifactReady(flow: ArtifactFlowState): boolean {
  return (
    flow.code.status === 'success' &&
    flow.jenkins.status === 'success' &&
    flow.harbor.status === 'success'
  )
}

export function deliveryReady(flow: DeliveryFlowState): boolean {
  return flow.status === 'active' && flow.execution.status === 'success'
}

export function dockerPullCommand(flow: ArtifactFlowState, harbor: HarborConnection): string {
  const parts = [
    harbor.registryUrl.replace(/\/+$/, ''),
    harbor.namespace.replace(/^\/+|\/+$/g, ''),
    flow.image.trim(),
  ].filter(Boolean)
  return `${parts.join('/')}:${flow.tag.trim() || 'latest'}`
}

export function mergeEnterpriseStageStates(
  states: EnterpriseStageStateMap,
  flow: EnterpriseFlow,
): EnterpriseStageStateMap {
  const next: EnterpriseStageStateMap = {
    ...states,
    artifacts: { ...states.artifacts },
    delivery: { ...states.delivery },
  }
  if (states.development.status !== 'done') return next

  if (states.artifacts.status === 'placeholder') {
    next.artifacts = {
      status: flow.artifact.status === 'done' ? 'done' : 'active',
      startedAt: flow.artifact.startedAt,
      completedAt: flow.artifact.completedAt,
    }
  }

  if (states.delivery.status === 'placeholder') {
    next.delivery = {
      status:
        flow.delivery.status === 'done'
          ? 'done'
          : flow.delivery.status === 'active'
            ? 'active'
            : 'locked',
      startedAt: flow.delivery.startedAt,
      completedAt: flow.delivery.completedAt,
    }
  }

  return next
}

export function downloadTextFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}
```

- [ ] **Step 5: 写 Store 失败测试**

创建 `packages/web/src/stores/enterpriseFlow.test.ts`：

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { useEnterpriseFlowStore } from './enterpriseFlow'

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})

describe('useEnterpriseFlowStore', () => {
  it('creates artifact flow when development completes', () => {
    const store = useEnterpriseFlowStore()
    const flow = store.ensureArtifact('p1')
    expect(flow.artifact.status).toBe('active')
    expect(flow.delivery.status).toBe('locked')
  })

  it('requires all steps before confirming artifact', () => {
    const store = useEnterpriseFlowStore()
    store.ensureArtifact('p1')
    expect(store.confirmArtifact('p1')).toBe(false)

    store.patchArtifactStep('p1', 'code', { status: 'success' })
    store.patchArtifactStep('p1', 'jenkins', { status: 'success' })
    store.patchArtifactStep('p1', 'harbor', { status: 'success' })
    expect(store.confirmArtifact('p1')).toBe(true)
    expect(store.get('p1')?.artifact.status).toBe('done')
    expect(store.get('p1')?.delivery.status).toBe('active')
  })

  it('only confirms delivery after a successful execution', () => {
    const store = useEnterpriseFlowStore()
    store.ensureArtifact('p1')
    store.patchArtifactStep('p1', 'code', { status: 'success' })
    store.patchArtifactStep('p1', 'jenkins', { status: 'success' })
    store.patchArtifactStep('p1', 'harbor', { status: 'success' })
    store.confirmArtifact('p1')

    expect(store.confirmDelivery('p1')).toBe(false)
    store.patchDelivery('p1', { method: 'k8s', targetId: 'k1' })
    store.patchDeliveryExecution('p1', { status: 'success' })
    expect(store.confirmDelivery('p1')).toBe(true)
    expect(store.get('p1')?.delivery.status).toBe('done')
  })

  it('persists flows across store instances', () => {
    const first = useEnterpriseFlowStore()
    first.ensureArtifact('p1')

    setActivePinia(createPinia())
    const second = useEnterpriseFlowStore()
    expect(second.get('p1')?.artifact.status).toBe('active')
  })
})
```

- [ ] **Step 6: 运行 Store 测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/stores/enterpriseFlow.test.ts
```

Expected: FAIL，store 不存在。

- [ ] **Step 7: 实现 Store**

创建 `packages/web/src/stores/enterpriseFlow.ts`：

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'

import type {
  ArtifactFlowState,
  ArtifactStepKey,
  DeliveryFlowState,
  EnterpriseFlow,
  MockStepState,
} from '@/types/enterpriseSpace'
import {
  artifactReady,
  createArtifactFlow,
  createEnterpriseFlow,
  deliveryReady,
  loadFlows,
  saveFlows,
} from '@/utils/enterpriseFlow'

export const useEnterpriseFlowStore = defineStore('enterpriseFlow', () => {
  const flows = ref<Record<string, EnterpriseFlow>>(loadFlows())

  function persist(): void {
    saveFlows(flows.value)
  }

  function get(projectId: string): EnterpriseFlow | null {
    return flows.value[projectId] ?? null
  }

  function ensureArtifact(projectId: string): EnterpriseFlow {
    const existing = flows.value[projectId]
    if (existing) {
      if (existing.artifact.status === 'locked') {
        existing.artifact = {
          ...existing.artifact,
          status: 'active',
          startedAt: new Date().toISOString(),
        }
        existing.updatedAt = new Date().toISOString()
        flows.value = { ...flows.value, [projectId]: existing }
        persist()
      }
      return existing
    }
    const flow = createEnterpriseFlow(projectId)
    flow.artifact = createArtifactFlow(true)
    flows.value = { ...flows.value, [projectId]: flow }
    persist()
    return flow
  }

  function patchArtifact(projectId: string, patch: Partial<ArtifactFlowState>): ArtifactFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      artifact: { ...flow.artifact, ...patch },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.artifact
  }

  function patchArtifactStep(
    projectId: string,
    step: ArtifactStepKey,
    patch: Partial<MockStepState>,
  ): ArtifactFlowState {
    const flow = ensureArtifact(projectId)
    const artifact = {
      ...flow.artifact,
      [step]: { ...flow.artifact[step], ...patch },
    }
    const next = {
      ...flow,
      artifact,
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.artifact
  }

  function confirmArtifact(projectId: string): boolean {
    const flow = flows.value[projectId]
    if (!flow || !artifactReady(flow.artifact)) return false
    const now = new Date().toISOString()
    const next = {
      ...flow,
      artifact: { ...flow.artifact, status: 'done' as const, completedAt: now },
      delivery: {
        ...flow.delivery,
        status: 'active' as const,
        startedAt: flow.delivery.startedAt ?? now,
      },
      updatedAt: now,
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return true
  }

  function patchDelivery(projectId: string, patch: Partial<DeliveryFlowState>): DeliveryFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      delivery: { ...flow.delivery, ...patch },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.delivery
  }

  function patchDeliveryExecution(
    projectId: string,
    patch: Partial<MockStepState>,
  ): DeliveryFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      delivery: {
        ...flow.delivery,
        execution: { ...flow.delivery.execution, ...patch },
      },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.delivery
  }

  function confirmDelivery(projectId: string): boolean {
    const flow = flows.value[projectId]
    if (!flow || !deliveryReady(flow.delivery)) return false
    const now = new Date().toISOString()
    const next = {
      ...flow,
      delivery: { ...flow.delivery, status: 'done' as const, completedAt: now },
      updatedAt: now,
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return true
  }

  return {
    flows,
    get,
    ensureArtifact,
    patchArtifact,
    patchArtifactStep,
    confirmArtifact,
    patchDelivery,
    patchDeliveryExecution,
    confirmDelivery,
  }
})
```

- [ ] **Step 8: 运行测试**

```bash
npm --prefix packages/web run test -- --run src/utils/enterpriseFlow.test.ts src/stores/enterpriseFlow.test.ts
npm --prefix packages/web run lint
```

Expected: 全部通过。

- [ ] **Step 9: Checkpoint**

```bash
make checkpoint
```

---

### Task 3: 设置路由、共享连接列表和制品空间配置页

**Files:**
- Modify: `packages/web/src/router/settingsRoutes.ts`
- Modify: `packages/web/src/router/settingsRoutes.test.ts`
- Create: `packages/web/src/components/settings/ConnectionList.vue`
- Create: `packages/web/src/components/settings/ArtifactConnectionDialog.vue`
- Create: `packages/web/src/views/settings/ArtifactSpaceConfigView.vue`
- Modify: `packages/web/src/styles/settings.css`

**Interfaces:**
- Consumes: Task 1 的 store、类型、`connectionDescription`、`maskedCredential`。
- Produces:
  - 菜单项 `settings-artifact-space`
  - `ConnectionList`：`items`、`edit`、`remove`
  - `ArtifactConnectionDialog`：`kind`、`connection`、`save`
  - 路由 `/settings/artifact-space`

- [ ] **Step 1: 更新路由失败测试**

修改 `packages/web/src/router/settingsRoutes.test.ts` 的两个断言为：

```ts
expect(children.map((child) => child.path)).toEqual([
  '',
  'profile',
  'notification-channels',
  'artifact-space',
  'storage',
  'workspace',
  'members',
])
expect(children.map((child) => child.name)).toEqual([
  'settings',
  'settings-profile',
  'settings-notification-channels',
  'settings-artifact-space',
  'settings-storage',
  'settings-workspace',
  'settings-members',
])
```

菜单断言改为：

```ts
expect(settingsMenu.map((item) => item.label)).toEqual([
  '个人信息',
  '通知渠道',
  '制品空间配置',
  '磁盘空间',
  '工作区设置',
  '成员',
])
expect(settingsMenu.map((item) => item.to)).toEqual([
  '/settings/profile',
  '/settings/notification-channels',
  '/settings/artifact-space',
  '/settings/storage',
  '/settings/workspace',
  '/settings/members',
])
```

- [ ] **Step 2: 运行测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/router/settingsRoutes.test.ts
```

Expected: FAIL，菜单和路由缺失。

- [ ] **Step 3: 更新设置路由**

在 `settingsMenu` 的“通知渠道”之后加入：

```ts
{
  label: '制品空间配置',
  icon: 'pi pi-box',
  to: '/settings/artifact-space',
  name: 'settings-artifact-space',
  group: '企业工程',
},
```

在 `settingsRoute.children` 中、“notification-channels”之后加入：

```ts
{
  path: 'artifact-space',
  name: 'settings-artifact-space',
  component: () => import('@/views/settings/ArtifactSpaceConfigView.vue'),
},
```

- [ ] **Step 4: 运行路由测试**

```bash
npm --prefix packages/web run test -- --run src/router/settingsRoutes.test.ts
```

Expected: 通过；页面组件尚未创建时 build 会失败，下一步补上。

- [ ] **Step 5: 写共享连接列表**

创建 `packages/web/src/components/settings/ConnectionList.vue`：

```vue
<script setup lang="ts">
import type { EnterpriseConnection } from '@/types/enterpriseSpace'
import { connectionDescription, maskedCredential } from '@/utils/enterpriseConnections'

defineProps<{
  items: EnterpriseConnection[]
  emptyTitle: string
  emptyText: string
}>()

const emit = defineEmits<{
  (event: 'edit', item: EnterpriseConnection): void
  (event: 'remove', item: EnterpriseConnection): void
}>()

function credential(connection: EnterpriseConnection): string {
  if ('secret' in connection) return maskedCredential(connection.secret)
  if ('kubeconfig' in connection) return maskedCredential(connection.kubeconfig)
  return ''
}
</script>

<template>
  <div v-if="items.length === 0" class="connection-empty">
    <i class="pi pi-box" />
    <strong>{{ emptyTitle }}</strong>
    <span>{{ emptyText }}</span>
  </div>
  <div v-else class="connection-list">
    <article v-for="item in items" :key="item.id" class="connection-card">
      <div class="connection-main">
        <strong>{{ item.name }}</strong>
        <span>{{ connectionDescription(item) }}</span>
      </div>
      <span v-if="credential(item)" class="connection-credential">
        {{ credential(item) }}
      </span>
      <div class="connection-actions">
        <button type="button" class="connection-action" @click="emit('edit', item)">编辑</button>
        <button type="button" class="connection-action danger" @click="emit('remove', item)">
          删除
        </button>
      </div>
    </article>
  </div>
</template>
```

- [ ] **Step 6: 写制品配置弹窗**

创建 `packages/web/src/components/settings/ArtifactConnectionDialog.vue`，完整内容：

```vue
<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, reactive, watch } from 'vue'

import type {
  ArtifactConnection,
  ArtifactConnectionInput,
  ArtifactConnectionKind,
} from '@/types/enterpriseSpace'

const props = defineProps<{
  kind: ArtifactConnectionKind
  connection: ArtifactConnection | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'save', input: ArtifactConnectionInput): void
}>()

const draft = reactive({
  name: '',
  provider: 'github' as 'github' | 'gitlab',
  url: '',
  username: '',
  secret: '',
  defaultBranch: 'main',
  registryUrl: '',
  namespace: '',
  jobsText: '',
})
const error = reactive({ message: '' })

const title = computed(() => {
  if (props.kind === 'code-repo') return '代码仓库'
  if (props.kind === 'harbor') return 'Harbor'
  return 'Jenkins'
})

const hasSavedSecret = computed(
  () => props.connection !== null && props.connection.secret !== '',
)

watch(
  () => [props.kind, props.connection] as const,
  () => {
    const item = props.connection
    draft.name = item?.name ?? ''
    draft.provider = item?.kind === 'code-repo' ? item.provider : 'github'
    draft.url = item?.kind === 'code-repo' || item?.kind === 'jenkins' ? item.url : ''
    draft.username =
      item?.kind === 'code-repo' || item?.kind === 'harbor' || item?.kind === 'jenkins'
        ? item.username
        : ''
    draft.secret = ''
    draft.defaultBranch = item?.kind === 'code-repo' ? item.defaultBranch : 'main'
    draft.registryUrl = item?.kind === 'harbor' ? item.registryUrl : ''
    draft.namespace = item?.kind === 'harbor' ? item.namespace : ''
    draft.jobsText = item?.kind === 'jenkins' ? item.jobs.join('\n') : ''
    error.message = ''
  },
  { immediate: true },
)

function selectValue(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

function submit() {
  if (!draft.name.trim()) {
    error.message = '请填写名称'
    return
  }
  if (props.kind === 'code-repo') {
    if (!draft.url.trim()) {
      error.message = '请填写仓库地址'
      return
    }
    emit('save', {
      kind: 'code-repo',
      name: draft.name.trim(),
      provider: draft.provider,
      url: draft.url.trim(),
      username: draft.username.trim(),
      secret: draft.secret,
      defaultBranch: draft.defaultBranch.trim() || 'main',
    })
    return
  }
  if (props.kind === 'harbor') {
    if (!draft.registryUrl.trim() || !draft.namespace.trim()) {
      error.message = '请填写 Harbor 地址和命名空间'
      return
    }
    emit('save', {
      kind: 'harbor',
      name: draft.name.trim(),
      registryUrl: draft.registryUrl.trim(),
      namespace: draft.namespace.trim(),
      username: draft.username.trim(),
      secret: draft.secret,
    })
    return
  }
  if (!draft.url.trim()) {
    error.message = '请填写 Jenkins 地址'
    return
  }
  emit('save', {
    kind: 'jenkins',
    name: draft.name.trim(),
    url: draft.url.trim(),
    username: draft.username.trim(),
    secret: draft.secret,
    jobs: draft.jobsText
      .split('\n')
      .map((item) => item.trim())
      .filter(Boolean),
  })
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <form class="dialog-card connection-dialog" @submit.prevent="submit">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">制品空间配置</p>
          <h2>{{ connection ? `编辑${title}` : `新增${title}` }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <label class="connection-field">
        <span>名称</span>
        <InputText v-model="draft.name" maxlength="64" placeholder="例如：主仓库" />
      </label>

      <template v-if="kind === 'code-repo'">
        <label class="connection-field">
          <span>平台</span>
          <select
            :value="draft.provider"
            class="connection-select"
            @change="draft.provider = selectValue($event) as 'github' | 'gitlab'"
          >
            <option value="github">GitHub</option>
            <option value="gitlab">GitLab</option>
          </select>
        </label>
        <label class="connection-field">
          <span>仓库地址</span>
          <InputText v-model="draft.url" placeholder="https://github.com/acme/app" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码 / Token</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的 Token' : 'ghp_...'"
          />
        </label>
        <label class="connection-field">
          <span>默认分支</span>
          <InputText v-model="draft.defaultBranch" placeholder="main" />
        </label>
      </template>

      <template v-else-if="kind === 'harbor'">
        <label class="connection-field">
          <span>Harbor 地址</span>
          <InputText v-model="draft.registryUrl" placeholder="https://harbor.example.com" />
        </label>
        <label class="connection-field">
          <span>命名空间</span>
          <InputText v-model="draft.namespace" placeholder="vibebiz" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的密码' : '请输入密码'"
          />
        </label>
      </template>

      <template v-else>
        <label class="connection-field">
          <span>Jenkins 地址</span>
          <InputText v-model="draft.url" placeholder="https://jenkins.example.com" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>API Token / 密码</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的 Token' : '请输入 API Token'"
          />
        </label>
        <label class="connection-field">
          <span>任务列表</span>
          <textarea
            v-model="draft.jobsText"
            class="connection-textarea"
            rows="4"
            placeholder="build-and-test&#10;release-build"
          />
          <small>一行一个任务名。</small>
        </label>
      </template>

      <p v-if="error.message" class="settings-error" role="alert">{{ error.message }}</p>

      <div class="connection-dialog-actions">
        <button class="connection-secondary" type="button" @click="emit('close')">取消</button>
        <button class="connection-primary" type="submit">
          <i class="pi pi-check" />
          保存
        </button>
      </div>
    </form>
  </div>
</template>
```

- [ ] **Step 7: 写制品配置页**

创建 `packages/web/src/views/settings/ArtifactSpaceConfigView.vue`，完整内容：

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'

import ArtifactConnectionDialog from '@/components/settings/ArtifactConnectionDialog.vue'
import ConnectionList from '@/components/settings/ConnectionList.vue'
import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import type {
  ArtifactConnection,
  ArtifactConnectionInput,
  ArtifactConnectionKind,
  EnterpriseConnection,
} from '@/types/enterpriseSpace'

const store = useEnterpriseConnectionsStore()
const activeTab = ref<ArtifactConnectionKind>('code-repo')
const dialogOpen = ref(false)
const editing = ref<ArtifactConnection | null>(null)

const tabs: { key: ArtifactConnectionKind; label: string; empty: string }[] = [
  { key: 'code-repo', label: '代码仓库', empty: '添加 GitHub 或 GitLab 仓库，供项目上传代码。' },
  { key: 'harbor', label: 'Harbor', empty: '添加镜像仓库，供项目推送和拉取镜像。' },
  { key: 'jenkins', label: 'Jenkins', empty: '添加 Jenkins 实例和任务，供项目触发构建。' },
]

const items = computed<EnterpriseConnection[]>(() => {
  if (activeTab.value === 'code-repo') return store.codeRepos
  if (activeTab.value === 'harbor') return store.harbors
  return store.jenkins
})

const activeLabel = computed(() => tabs.find((item) => item.key === activeTab.value)?.label ?? '')

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(item: EnterpriseConnection) {
  editing.value = item as ArtifactConnection
  dialogOpen.value = true
}

function save(input: ArtifactConnectionInput) {
  if (editing.value) {
    store.update({ ...editing.value, ...input } as ArtifactConnection)
  } else {
    store.create(input)
  }
  dialogOpen.value = false
  editing.value = null
}

function remove(item: EnterpriseConnection) {
  if (!window.confirm(`确定删除「${item.name}」吗？`)) return
  store.remove(item.id)
}
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>制品空间配置</h1>
        <p>维护代码仓库、Harbor 和 Jenkins 连接，供企业项目阶段 6 使用。</p>
      </div>
      <button class="config-add" type="button" @click="openCreate">
        <i class="pi pi-plus" />
        新增{{ activeLabel }}
      </button>
    </header>

    <p class="settings-notice">
      配置仅保存在当前浏览器；接入后端前请勿填写真实生产密钥。
    </p>

    <div class="config-tabs" role="tablist" aria-label="制品空间配置分类">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="config-tab"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>

    <ConnectionList
      :items="items"
      :empty-title="`暂无${activeLabel}`"
      :empty-text="tabs.find((item) => item.key === activeTab)?.empty ?? ''"
      @edit="openEdit"
      @remove="remove"
    />

    <ArtifactConnectionDialog
      v-if="dialogOpen"
      :kind="activeTab"
      :connection="editing"
      @close="dialogOpen = false"
      @save="save"
    />
  </section>
</template>
```

- [ ] **Step 8: 补设置页样式**

在 `packages/web/src/styles/settings.css` 末尾加入：

```css
.config-add {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 36px;
  padding: 0 14px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 8px;
  color: var(--surface);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.config-tabs {
  display: flex;
  gap: 2px;
  padding: 4px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 10px;
}

.config-tab {
  flex: 1;
  height: 34px;
  background: transparent;
  border: 0;
  border-radius: 7px;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.config-tab.active {
  background: var(--surface);
  box-shadow: 0 1px 2px rgba(17, 17, 17, 0.06);
  color: var(--ink);
}

.connection-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.connection-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 16px 18px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.connection-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.connection-main strong {
  color: var(--ink);
  font-size: 13px;
}

.connection-main span {
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.connection-credential {
  padding: 3px 7px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 6px;
  color: var(--muted);
  font-size: 10px;
  white-space: nowrap;
}

.connection-actions {
  display: flex;
  gap: 4px;
}

.connection-action {
  padding: 5px 8px;
  background: transparent;
  border: 0;
  border-radius: 6px;
  color: var(--muted);
  font-size: 11px;
  cursor: pointer;
}

.connection-action:hover {
  background: var(--soft);
  color: var(--ink);
}

.connection-action.danger:hover {
  color: var(--ink);
}

.connection-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 7px;
  padding: 44px 20px;
  border: 1px dashed var(--line-strong);
  border-radius: 14px;
  color: var(--muted);
  text-align: center;
}

.connection-empty i {
  font-size: 22px;
}

.connection-empty strong {
  color: var(--ink);
  font-size: 13px;
}

.connection-empty span {
  font-size: 12px;
}

.connection-dialog {
  width: min(560px, 100%);
}

.connection-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--ink);
  font-size: 12px;
  font-weight: 600;
}

.connection-field small {
  color: var(--muted);
  font-size: 11px;
  font-weight: 400;
}

.connection-field .p-inputtext {
  width: 100%;
}

.connection-select,
.connection-textarea {
  width: 100%;
  padding: 0 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  outline: none;
}

.connection-select {
  height: 38px;
}

.connection-textarea {
  padding: 9px 10px;
  resize: vertical;
}

.connection-dialog-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.connection-primary,
.connection-secondary {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 34px;
  padding: 0 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.connection-primary {
  background: var(--ink);
  border: 1px solid var(--ink);
  color: var(--surface);
}

.connection-secondary {
  background: var(--surface);
  border: 1px solid var(--line-strong);
  color: var(--ink);
}
```

- [ ] **Step 9: 运行测试、lint、build**

```bash
npm --prefix packages/web run test -- --run src/router/settingsRoutes.test.ts
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 通过。

- [ ] **Step 10: Checkpoint**

```bash
make checkpoint
```

---

### Task 4: 交付空间配置页和弹窗

**Files:**
- Create: `packages/web/src/components/settings/DeliveryConnectionDialog.vue`
- Create: `packages/web/src/views/settings/DeliverySpaceConfigView.vue`
- Modify: `packages/web/src/router/settingsRoutes.ts`
- Modify: `packages/web/src/router/settingsRoutes.test.ts`

**Interfaces:**
- Consumes: Task 1 store、`DeliveryConnection`、`DeliveryConnectionInput`。
- Produces: 可用的 `/settings/delivery-space` 页面。

- [ ] **Step 1: 加入交付路由和测试**

在 `settingsMenu` 的“制品空间配置”之后加入：

```ts
{
  label: '交付空间配置',
  icon: 'pi pi-send',
  to: '/settings/delivery-space',
  name: 'settings-delivery-space',
  group: '企业工程',
},
```

在 `settingsRoute.children` 的 `artifact-space` 之后加入：

```ts
{
  path: 'delivery-space',
  name: 'settings-delivery-space',
  component: () => import('@/views/settings/DeliverySpaceConfigView.vue'),
},
```

把 `settingsRoutes.test.ts` 的路径断言改为：

```ts
expect(children.map((child) => child.path)).toEqual([
  '',
  'profile',
  'notification-channels',
  'artifact-space',
  'delivery-space',
  'storage',
  'workspace',
  'members',
])
```

名称断言改为：

```ts
expect(children.map((child) => child.name)).toEqual([
  'settings',
  'settings-profile',
  'settings-notification-channels',
  'settings-artifact-space',
  'settings-delivery-space',
  'settings-storage',
  'settings-workspace',
  'settings-members',
])
```

菜单断言改为：

```ts
expect(settingsMenu.map((item) => item.label)).toEqual([
  '个人信息',
  '通知渠道',
  '制品空间配置',
  '交付空间配置',
  '磁盘空间',
  '工作区设置',
  '成员',
])
expect(settingsMenu.map((item) => item.to)).toEqual([
  '/settings/profile',
  '/settings/notification-channels',
  '/settings/artifact-space',
  '/settings/delivery-space',
  '/settings/storage',
  '/settings/workspace',
  '/settings/members',
])
```

- [ ] **Step 2: 运行测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/router/settingsRoutes.test.ts
```

Expected: FAIL，`DeliverySpaceConfigView.vue` 不存在。

- [ ] **Step 3: 写交付配置弹窗**

创建 `packages/web/src/components/settings/DeliveryConnectionDialog.vue`，完整内容：

```vue
<script setup lang="ts">
import InputText from 'primevue/inputtext'
import { computed, reactive, watch } from 'vue'

import type {
  DeliveryConnection,
  DeliveryConnectionInput,
  DeliveryConnectionKind,
} from '@/types/enterpriseSpace'

const props = defineProps<{
  kind: DeliveryConnectionKind
  connection: DeliveryConnection | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
  (event: 'save', input: DeliveryConnectionInput): void
}>()

const draft = reactive({
  name: '',
  apiServer: '',
  kubeconfig: '',
  defaultNamespace: 'default',
  host: '',
  port: 22,
  username: '',
  secret: '',
  url: '',
  contact: '',
  note: '',
})
const error = reactive({ message: '' })

const title = computed(() => {
  if (props.kind === 'k8s') return 'K8s 集群'
  if (props.kind === 'cloud') return '云主机'
  if (props.kind === 'argo') return 'Argo'
  return 'FDE'
})

const hasSavedSecret = computed(
  () => props.connection !== null && 'secret' in props.connection && props.connection.secret !== '',
)
const hasSavedKubeconfig = computed(
  () =>
    props.connection !== null &&
    props.connection.kind === 'k8s' &&
    props.connection.kubeconfig !== '',
)

watch(
  () => [props.kind, props.connection] as const,
  () => {
    const item = props.connection
    draft.name = item?.name ?? ''
    draft.apiServer = item?.kind === 'k8s' ? item.apiServer : ''
    draft.kubeconfig = ''
    draft.defaultNamespace = item?.kind === 'k8s' ? item.defaultNamespace : 'default'
    draft.host = item?.kind === 'cloud' ? item.host : ''
    draft.port = item?.kind === 'cloud' ? item.port : 22
    draft.username =
      item?.kind === 'cloud' || item?.kind === 'argo' ? item.username : ''
    draft.secret = ''
    draft.url = item?.kind === 'argo' ? item.url : ''
    draft.contact = item?.kind === 'fde' ? item.contact : ''
    draft.note = item?.kind === 'fde' ? item.note : ''
    error.message = ''
  },
  { immediate: true },
)

function submit() {
  if (!draft.name.trim()) {
    error.message = '请填写名称'
    return
  }
  if (props.kind === 'k8s') {
    if (!draft.apiServer.trim()) {
      error.message = '请填写 apiserver 地址'
      return
    }
    emit('save', {
      kind: 'k8s',
      name: draft.name.trim(),
      apiServer: draft.apiServer.trim(),
      kubeconfig: draft.kubeconfig,
      defaultNamespace: draft.defaultNamespace.trim() || 'default',
    })
    return
  }
  if (props.kind === 'cloud') {
    if (!draft.host.trim() || !draft.username.trim()) {
      error.message = '请填写主机地址和用户名'
      return
    }
    emit('save', {
      kind: 'cloud',
      name: draft.name.trim(),
      host: draft.host.trim(),
      port: Number(draft.port) || 22,
      username: draft.username.trim(),
      secret: draft.secret,
    })
    return
  }
  if (props.kind === 'argo') {
    if (!draft.url.trim()) {
      error.message = '请填写 Argo 地址'
      return
    }
    emit('save', {
      kind: 'argo',
      name: draft.name.trim(),
      url: draft.url.trim(),
      username: draft.username.trim(),
      secret: draft.secret,
    })
    return
  }
  if (!draft.contact.trim()) {
    error.message = '请填写 FDE 联系方式'
    return
  }
  emit('save', {
    kind: 'fde',
    name: draft.name.trim(),
    contact: draft.contact.trim(),
    note: draft.note.trim(),
  })
}
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <form class="dialog-card connection-dialog" @submit.prevent="submit">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">交付空间配置</p>
          <h2>{{ connection ? `编辑${title}` : `新增${title}` }}</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <label class="connection-field">
        <span>名称</span>
        <InputText v-model="draft.name" maxlength="64" placeholder="例如：生产集群" />
      </label>

      <template v-if="kind === 'k8s'">
        <label class="connection-field">
          <span>apiserver 地址</span>
          <InputText v-model="draft.apiServer" placeholder="https://kubernetes.example.com:6443" />
        </label>
        <label class="connection-field">
          <span>kubeconfig</span>
          <textarea
            v-model="draft.kubeconfig"
            class="connection-textarea"
            rows="6"
            :placeholder="hasSavedKubeconfig ? '留空保持已保存的 kubeconfig' : '粘贴 kubeconfig 内容'"
          />
        </label>
        <label class="connection-field">
          <span>默认命名空间</span>
          <InputText v-model="draft.defaultNamespace" placeholder="default" />
        </label>
      </template>

      <template v-else-if="kind === 'cloud'">
        <label class="connection-field">
          <span>主机地址</span>
          <InputText v-model="draft.host" placeholder="10.0.0.10" />
        </label>
        <label class="connection-field">
          <span>SSH 端口</span>
          <input v-model.number="draft.port" class="connection-select" type="number" min="1" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的密码' : '请输入密码'"
          />
        </label>
      </template>

      <template v-else-if="kind === 'argo'">
        <label class="connection-field">
          <span>Argo 地址</span>
          <InputText v-model="draft.url" placeholder="https://argo.example.com" />
        </label>
        <label class="connection-field">
          <span>用户名</span>
          <InputText v-model="draft.username" autocomplete="off" />
        </label>
        <label class="connection-field">
          <span>密码 / Token</span>
          <InputText
            v-model="draft.secret"
            type="password"
            autocomplete="new-password"
            :placeholder="hasSavedSecret ? '留空保持已保存的凭据' : '请输入密码或 Token'"
          />
        </label>
      </template>

      <template v-else>
        <label class="connection-field">
          <span>联系方式</span>
          <InputText v-model="draft.contact" placeholder="邮箱、电话或 IM 账号" />
        </label>
        <label class="connection-field">
          <span>备注</span>
          <textarea v-model="draft.note" class="connection-textarea" rows="4" />
        </label>
      </template>

      <p v-if="error.message" class="settings-error" role="alert">{{ error.message }}</p>

      <div class="connection-dialog-actions">
        <button class="connection-secondary" type="button" @click="emit('close')">取消</button>
        <button class="connection-primary" type="submit">
          <i class="pi pi-check" />
          保存
        </button>
      </div>
    </form>
  </div>
</template>
```

- [ ] **Step 4: 写交付配置页**

创建 `packages/web/src/views/settings/DeliverySpaceConfigView.vue`，完整内容：

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'

import ConnectionList from '@/components/settings/ConnectionList.vue'
import DeliveryConnectionDialog from '@/components/settings/DeliveryConnectionDialog.vue'
import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import type {
  DeliveryConnection,
  DeliveryConnectionInput,
  DeliveryConnectionKind,
  EnterpriseConnection,
} from '@/types/enterpriseSpace'

const store = useEnterpriseConnectionsStore()
const activeTab = ref<DeliveryConnectionKind>('k8s')
const dialogOpen = ref(false)
const editing = ref<DeliveryConnection | null>(null)

const tabs: { key: DeliveryConnectionKind; label: string; empty: string }[] = [
  { key: 'k8s', label: 'K8s 集群', empty: '添加集群后，项目可部署到该集群。' },
  { key: 'cloud', label: '云主机', empty: '添加云主机节点，供项目直接部署。' },
  { key: 'argo', label: 'Argo', empty: '添加 Argo 地址，供项目触发 GitOps 交付。' },
  { key: 'fde', label: 'FDE 人工', empty: '添加 FDE 人员，供人工交付选择。' },
]

const items = computed<EnterpriseConnection[]>(() => {
  if (activeTab.value === 'k8s') return store.k8sClusters
  if (activeTab.value === 'cloud') return store.cloudNodes
  if (activeTab.value === 'argo') return store.argoes
  return store.fdes
})

const activeLabel = computed(() => tabs.find((item) => item.key === activeTab.value)?.label ?? '')

function openCreate() {
  editing.value = null
  dialogOpen.value = true
}

function openEdit(item: EnterpriseConnection) {
  editing.value = item as DeliveryConnection
  dialogOpen.value = true
}

function save(input: DeliveryConnectionInput) {
  if (editing.value) {
    store.update({ ...editing.value, ...input } as DeliveryConnection)
  } else {
    store.create(input)
  }
  dialogOpen.value = false
  editing.value = null
}

function remove(item: EnterpriseConnection) {
  if (!window.confirm(`确定删除「${item.name}」吗？`)) return
  store.remove(item.id)
}
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>交付空间配置</h1>
        <p>维护 K8s、云主机、Argo 和 FDE 人员，供企业项目阶段 7 使用。</p>
      </div>
      <button class="config-add" type="button" @click="openCreate">
        <i class="pi pi-plus" />
        新增{{ activeLabel }}
      </button>
    </header>

    <p class="settings-notice">
      配置仅保存在当前浏览器；接入后端前请勿填写真实生产密钥。
    </p>

    <div class="config-tabs" role="tablist" aria-label="交付空间配置分类">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        class="config-tab"
        :class="{ active: activeTab === tab.key }"
        @click="activeTab = tab.key"
      >
        {{ tab.label }}
      </button>
    </div>

    <ConnectionList
      :items="items"
      :empty-title="`暂无${activeLabel}`"
      :empty-text="tabs.find((item) => item.key === activeTab)?.empty ?? ''"
      @edit="openEdit"
      @remove="remove"
    />

    <DeliveryConnectionDialog
      v-if="dialogOpen"
      :kind="activeTab"
      :connection="editing"
      @close="dialogOpen = false"
      @save="save"
    />
  </section>
</template>
```

- [ ] **Step 5: 运行设置页验证**

```bash
npm --prefix packages/web run test -- --run src/router/settingsRoutes.test.ts
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 全部通过。

- [ ] **Step 6: Checkpoint**

```bash
make checkpoint
```

---

### Task 5: 阶段 6/7 工作区定义

**Files:**
- Modify: `packages/web/src/utils/enterprise.ts`
- Modify: `packages/web/src/utils/enterprise.test.ts`

**Interfaces:**
- Consumes: 现有 `EnterpriseStageDefinition` / `EnterpriseStageWorkspace`。
- Produces:
  - 阶段 6 workspace 为 `artifact`，阶段 7 workspace 为 `delivery`
  - `buildStagePrompt()` 对阶段 6/7 返回空字符串
  - 初始状态仍把阶段 6/7 标为 `placeholder`，等待 flow overlay

- [ ] **Step 1: 更新阶段类型测试**

在 `packages/web/src/utils/enterprise.test.ts` 增加：

```ts
it('marks artifact and delivery as dedicated workspaces', () => {
  expect(ENTERPRISE_STAGES.find((item) => item.key === 'artifacts')?.workspace).toBe('artifact')
  expect(ENTERPRISE_STAGES.find((item) => item.key === 'delivery')?.workspace).toBe('delivery')
  expect(buildStagePrompt(ENTERPRISE_STAGES[5])).toBe('')
  expect(buildStagePrompt(ENTERPRISE_STAGES[6])).toBe('')
})
```

原测试 `stages.artifacts.status === 'placeholder'` 保持成立。

- [ ] **Step 2: 运行测试确认失败**

```bash
npm --prefix packages/web run test -- --run src/utils/enterprise.test.ts
```

Expected: FAIL，workspace 仍是 `placeholder`。

- [ ] **Step 3: 修改阶段定义**

在 `packages/web/src/utils/enterprise.ts`：

```ts
export type EnterpriseStageWorkspace =
  | 'document'
  | 'development'
  | 'artifact'
  | 'delivery'
  | 'placeholder'
```

阶段 6/7 改为：

```ts
{
  key: 'artifacts',
  seq: 6,
  title: '制品空间',
  roleCode: '',
  roleLabel: '制品流水线',
  artifactPath: '',
  workspace: 'artifact',
},
{
  key: 'delivery',
  seq: 7,
  title: '交付空间',
  roleCode: '',
  roleLabel: '交付流水线',
  artifactPath: '',
  workspace: 'delivery',
},
```

`createInitialStageState` 的状态判断改为：

```ts
status:
  stage.key === 'requirements'
    ? 'active'
    : stage.key === 'artifacts' || stage.key === 'delivery'
      ? 'placeholder'
      : 'locked',
```

`buildStagePrompt` 开头改为：

```ts
if (stage.workspace !== 'document' && stage.workspace !== 'development') {
  return ''
}
```

- [ ] **Step 4: 运行测试**

```bash
npm --prefix packages/web run test -- --run src/utils/enterprise.test.ts
```

Expected: 通过。

- [ ] **Step 5: Checkpoint**

```bash
make checkpoint
```

---

### Task 6: 制品空间工作区

**Files:**
- Create: `packages/web/src/components/enterprise/ArtifactWorkspace.vue`
- Modify: `packages/web/src/styles/enterprise.css`

**Interfaces:**
- Consumes: `useEnterpriseConnectionsStore`、`useEnterpriseFlowStore`、`enterpriseFlow` 工具。
- Produces: 阶段 6 完整 UI，emit `confirmed`。

- [ ] **Step 1: 创建制品工作区**

创建 `packages/web/src/components/enterprise/ArtifactWorkspace.vue`，完整内容：

```vue
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import type { ArtifactFlowState, ArtifactStepKey, MockStepStatus } from '@/types/enterpriseSpace'
import {
  artifactReady,
  artifactStepUnlocked,
  createEmptyStep,
  dockerPullCommand,
  downloadTextFile,
} from '@/utils/enterpriseFlow'

const props = defineProps<{
  projectId: string
  flow: ArtifactFlowState
}>()

const emit = defineEmits<{
  (event: 'confirmed'): void
}>()

const connections = useEnterpriseConnectionsStore()
const flows = useEnterpriseFlowStore()
const error = ref('')
const copied = ref(false)
let timer: ReturnType<typeof setTimeout> | null = null
let runningStep: ArtifactStepKey | null = null
let copyTimer: ReturnType<typeof setTimeout> | null = null

const selectedRepo = computed(() =>
  connections.codeRepos.find((item) => item.id === props.flow.codeRepoId),
)
const selectedJenkins = computed(() =>
  connections.jenkins.find((item) => item.id === props.flow.jenkinsId),
)
const selectedHarbor = computed(() =>
  connections.harbors.find((item) => item.id === props.flow.harborId),
)
const ready = computed(() => artifactReady(props.flow))
const running = computed(
  () =>
    props.flow.code.status === 'running' ||
    props.flow.jenkins.status === 'running' ||
    props.flow.harbor.status === 'running',
)
const dockerCommand = computed(() =>
  selectedHarbor.value ? dockerPullCommand(props.flow, selectedHarbor.value) : '',
)

function statusText(status: MockStepStatus): string {
  if (status === 'running') return '执行中'
  if (status === 'success') return '已完成'
  if (status === 'error') return '失败'
  return '待执行'
}

function selectRepo(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  const repo = connections.codeRepos.find((item) => item.id === id)
  flows.patchArtifact(props.projectId, {
    codeRepoId: id,
    branch: repo?.defaultBranch ?? '',
  })
}

function selectJenkins(event: Event) {
  const id = (event.target as HTMLSelectElement).value
  flows.patchArtifact(props.projectId, { jenkinsId: id, jenkinsJob: '' })
}

function selectHarbor(event: Event) {
  flows.patchArtifact(props.projectId, {
    harborId: (event.target as HTMLSelectElement).value,
  })
}

function inputValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function selectValue(event: Event): string {
  return (event.target as HTMLSelectElement).value
}

function updateField(
  field: 'branch' | 'commitMessage' | 'jenkinsJob' | 'image' | 'tag',
  value: string,
): void {
  const patch: Partial<ArtifactFlowState> = {}
  if (field === 'branch') patch.branch = value
  if (field === 'commitMessage') patch.commitMessage = value
  if (field === 'jenkinsJob') patch.jenkinsJob = value
  if (field === 'image') patch.image = value
  if (field === 'tag') patch.tag = value
  flows.patchArtifact(props.projectId, patch)
}

function startStep(step: ArtifactStepKey): void {
  if (running.value || props.flow.status === 'done') return
  if (step === 'code' && (!props.flow.codeRepoId || !props.flow.branch.trim())) {
    error.value = '请选择代码仓库并填写分支'
    return
  }
  if (step === 'jenkins' && (!props.flow.jenkinsId || !props.flow.jenkinsJob.trim())) {
    error.value = '请选择 Jenkins 配置和任务'
    return
  }
  if (step === 'harbor' && (!props.flow.harborId || !props.flow.image.trim())) {
    error.value = '请选择 Harbor 并填写镜像名'
    return
  }
  error.value = ''
  const startedAt = new Date().toISOString()
  flows.patchArtifactStep(props.projectId, step, {
    status: 'running',
    startedAt,
    completedAt: null,
    error: '',
  })
  runningStep = step
  timer = setTimeout(() => {
    flows.patchArtifactStep(props.projectId, step, {
      status: 'success',
      completedAt: new Date().toISOString(),
    })
    runningStep = null
    timer = null
  }, 1100)
}

function confirm() {
  if (!ready.value) {
    error.value = '三步全部成功后才能确认制品'
    return
  }
  if (flows.confirmArtifact(props.projectId)) {
    error.value = ''
    emit('confirmed')
  }
}

async function copyCommand() {
  if (!dockerCommand.value) return
  await navigator.clipboard?.writeText(dockerCommand.value)
  copied.value = true
  if (copyTimer) clearTimeout(copyTimer)
  copyTimer = setTimeout(() => {
    copied.value = false
  }, 1500)
}

function downloadCode() {
  downloadTextFile(
    `vibebiz-${props.projectId}-source-demo.txt`,
    `Vibebiz 代码下载演示文件\n项目：${props.projectId}\n仓库：${selectedRepo.value?.url ?? '-'}\n分支：${props.flow.branch}\n\n前端演示文件，未连接真实代码仓库。`,
  )
}

function downloadHelm() {
  const image = props.flow.image.replace(/[^\w.-]+/g, '-') || 'image'
  const tag = props.flow.tag.replace(/[^\w.-]+/g, '-') || 'latest'
  downloadTextFile(
    `vibebiz-${image}-${tag}-helm-demo.yaml`,
    `# Vibebiz Helm 下载演示文件\n# 前端演示文件，未连接真实构建系统\nimage: ${dockerCommand.value || '-'}\nnamespace: ${selectedHarbor.value?.namespace ?? '-'}\n`,
  )
}

onMounted(() => {
  for (const step of ['code', 'jenkins', 'harbor'] as ArtifactStepKey[]) {
    if (props.flow[step].status === 'running') {
      flows.patchArtifactStep(props.projectId, step, {
        ...createEmptyStep(),
      })
    }
  }
})

onBeforeUnmount(() => {
  if (timer) clearTimeout(timer)
  if (copyTimer) clearTimeout(copyTimer)
  if (runningStep) {
    flows.patchArtifactStep(props.projectId, runningStep, {
      ...createEmptyStep(),
    })
  }
})
</script>

<template>
  <section class="artifact-workspace">
    <header class="pipeline-head">
      <div>
        <p class="enterprise-stage-eyebrow">阶段 6</p>
        <h1>制品空间</h1>
        <p>按顺序上传代码、触发 Jenkins 构建，并把镜像推送到 Harbor。</p>
      </div>
      <button
        class="enterprise-primary"
        type="button"
        :disabled="!ready || flow.status === 'done'"
        @click="confirm"
      >
        <i :class="flow.status === 'done' ? 'pi pi-check' : 'pi pi-flag'" />
        {{ flow.status === 'done' ? '制品已完成' : '确认制品完成' }}
      </button>
    </header>

    <p v-if="error" class="enterprise-stage-error">{{ error }}</p>

    <div class="pipeline-list">
      <article class="pipeline-card" :class="{ done: flow.code.status === 'success' }">
        <span class="pipeline-index">1</span>
        <div class="pipeline-body">
          <header>
            <strong>上传代码</strong>
            <span class="pipeline-status">{{ statusText(flow.code.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.codeRepoId"
              :disabled="!artifactStepUnlocked(flow, 'code') || flow.code.status === 'success'"
              @change="selectRepo"
            >
              <option value="">选择代码仓库</option>
              <option v-for="item in connections.codeRepos" :key="item.id" :value="item.id">
                {{ item.name }} · {{ item.provider }}
              </option>
            </select>
            <input
              :value="flow.branch"
              class="pipeline-input"
              placeholder="分支"
              :disabled="flow.code.status === 'success'"
              @input="updateField('branch', inputValue($event))"
            />
            <input
              :value="flow.commitMessage"
              class="pipeline-input"
              placeholder="提交信息"
              :disabled="flow.code.status === 'success'"
              @input="updateField('commitMessage', inputValue($event))"
            />
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'code') ||
              running ||
              flow.code.status === 'success' ||
              !connections.codeRepos.length
            "
            @click="startStep('code')"
          >
            上传代码
          </button>
          <p v-if="!connections.codeRepos.length" class="pipeline-hint">
            请先在设置中配置代码仓库。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>

      <article class="pipeline-card" :class="{ done: flow.jenkins.status === 'success' }">
        <span class="pipeline-index">2</span>
        <div class="pipeline-body">
          <header>
            <strong>Jenkins 构建</strong>
            <span class="pipeline-status">{{ statusText(flow.jenkins.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.jenkinsId"
              :disabled="
                !artifactStepUnlocked(flow, 'jenkins') || flow.jenkins.status === 'success'
              "
              @change="selectJenkins"
            >
              <option value="">选择 Jenkins</option>
              <option v-for="item in connections.jenkins" :key="item.id" :value="item.id">
                {{ item.name }}
              </option>
            </select>
            <select
              :value="flow.jenkinsJob"
              class="pipeline-input"
              :disabled="
                !artifactStepUnlocked(flow, 'jenkins') ||
                !selectedJenkins ||
                flow.jenkins.status === 'success'
              "
              @change="updateField('jenkinsJob', selectValue($event))"
            >
              <option value="">选择任务</option>
              <option v-for="job in selectedJenkins?.jobs ?? []" :key="job" :value="job">
                {{ job }}
              </option>
            </select>
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'jenkins') ||
              running ||
              flow.jenkins.status === 'success' ||
              !connections.jenkins.length
            "
            @click="startStep('jenkins')"
          >
            开始构建
          </button>
          <p v-if="!connections.jenkins.length" class="pipeline-hint">
            请先在设置中配置 Jenkins。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>

      <article class="pipeline-card" :class="{ done: flow.harbor.status === 'success' }">
        <span class="pipeline-index">3</span>
        <div class="pipeline-body">
          <header>
            <strong>Harbor 推送</strong>
            <span class="pipeline-status">{{ statusText(flow.harbor.status) }}</span>
          </header>
          <div class="pipeline-fields">
            <select
              class="pipeline-input"
              :value="flow.harborId"
              :disabled="!artifactStepUnlocked(flow, 'harbor') || flow.harbor.status === 'success'"
              @change="selectHarbor"
            >
              <option value="">选择 Harbor</option>
              <option v-for="item in connections.harbors" :key="item.id" :value="item.id">
                {{ item.name }} · {{ item.namespace }}
              </option>
            </select>
            <input
              :value="flow.image"
              class="pipeline-input"
              placeholder="镜像名"
              :disabled="flow.harbor.status === 'success'"
              @input="updateField('image', inputValue($event))"
            />
            <input
              :value="flow.tag"
              class="pipeline-input"
              placeholder="Tag"
              :disabled="flow.harbor.status === 'success'"
              @input="updateField('tag', inputValue($event))"
            />
          </div>
          <button
            class="pipeline-run"
            type="button"
            :disabled="
              !artifactStepUnlocked(flow, 'harbor') ||
              running ||
              flow.harbor.status === 'success' ||
              !connections.harbors.length
            "
            @click="startStep('harbor')"
          >
            推送镜像
          </button>
          <p v-if="!connections.harbors.length" class="pipeline-hint">
            请先在设置中配置 Harbor。<RouterLink to="/settings/artifact-space">去配置</RouterLink>
          </p>
        </div>
      </article>
    </div>

    <section v-if="ready" class="pipeline-result">
      <h2>制品已就绪</h2>
      <code>{{ dockerCommand }}</code>
      <div class="pipeline-result-actions">
        <button class="enterprise-secondary" type="button" @click="copyCommand">
          <i class="pi pi-copy" />
          {{ copied ? '已复制' : '复制 Docker 命令' }}
        </button>
        <button class="enterprise-secondary" type="button" @click="downloadCode">
          <i class="pi pi-download" />
          手动下载代码
        </button>
        <button class="enterprise-secondary" type="button" @click="downloadHelm">
          <i class="pi pi-download" />
          下载 Helm 文件
        </button>
      </div>
    </section>
  </section>
</template>
```

- [ ] **Step 2: 补制品流水线样式**

在 `packages/web/src/styles/enterprise.css` 末尾加入：

```css
.artifact-workspace,
.delivery-workspace {
  display: flex;
  flex: 1;
  flex-direction: column;
  min-height: 0;
  gap: 18px;
  padding: 24px;
  overflow-y: auto;
}

.pipeline-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 24px;
}

.pipeline-head h1 {
  margin: 2px 0 6px;
  color: var(--ink);
  font-size: 24px;
}

.pipeline-head p:not(.enterprise-stage-eyebrow) {
  margin: 0;
  color: var(--muted);
  font-size: 13px;
}

.pipeline-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.pipeline-card {
  display: flex;
  gap: 14px;
  padding: 18px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.pipeline-card.done {
  border-color: var(--line-strong);
  background: var(--soft);
}

.pipeline-index {
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border-radius: 999px;
  background: var(--ink);
  color: var(--surface);
  font-size: 12px;
  font-weight: 700;
}

.pipeline-body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.pipeline-body header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.pipeline-body header strong {
  color: var(--ink);
  font-size: 14px;
}

.pipeline-status {
  color: var(--muted);
  font-size: 11px;
}

.pipeline-fields {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
}

.pipeline-input {
  width: 100%;
  height: 36px;
  padding: 0 10px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 8px;
  color: var(--text);
  font: inherit;
  outline: none;
}

.pipeline-input:focus {
  border-color: var(--ink);
}

.pipeline-run {
  align-self: flex-start;
  height: 32px;
  padding: 0 12px;
  background: var(--ink);
  border: 1px solid var(--ink);
  border-radius: 8px;
  color: var(--surface);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.pipeline-run:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.pipeline-hint {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
}

.pipeline-result {
  padding: 18px;
  background: var(--blue-soft);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.pipeline-result h2 {
  margin: 0 0 10px;
  color: var(--ink);
  font-size: 14px;
}

.pipeline-result code {
  display: block;
  overflow-x: auto;
  padding: 10px 12px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 8px;
  color: var(--ink);
  font-size: 12px;
}

.pipeline-result-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 12px;
}
```

- [ ] **Step 3: 运行测试、lint、build**

```bash
npm --prefix packages/web run test -- --run
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 通过。

- [ ] **Step 4: Checkpoint**

```bash
make checkpoint
```

---

### Task 7: 交付空间工作区

**Files:**
- Create: `packages/web/src/components/enterprise/DeliveryWorkspace.vue`
- Modify: `packages/web/src/styles/enterprise.css`

**Interfaces:**
- Consumes: 两个 store、`deliveryReady`、`createEmptyStep`。
- Produces: 阶段 7 完整 UI，emit `confirmed`。

- [ ] **Step 1: 创建交付工作区**

创建 `packages/web/src/components/enterprise/DeliveryWorkspace.vue`，完整内容：

```vue
<script setup lang="ts">
import { computed, onBeforeUnmount, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { useEnterpriseConnectionsStore } from '@/stores/enterpriseConnections'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import type {
  DeliveryConnectionKind,
  DeliveryFlowState,
  MockStepStatus,
} from '@/types/enterpriseSpace'
import { createEmptyStep, deliveryReady } from '@/utils/enterpriseFlow'

const props = defineProps<{
  projectId: string
  flow: DeliveryFlowState
}>()

const emit = defineEmits<{
  (event: 'confirmed'): void
}>()

const connections = useEnterpriseConnectionsStore()
const flows = useEnterpriseFlowStore()
const error = ref('')
let timer: ReturnType<typeof setTimeout> | null = null

const methods: { key: DeliveryConnectionKind; title: string; description: string }[] = [
  { key: 'k8s', title: 'K8s 集群', description: '部署到已注册的 Kubernetes 集群。' },
  { key: 'cloud', title: '云主机', description: '部署到已注册的云主机节点。' },
  { key: 'argo', title: 'Argo', description: '触发 Argo GitOps 交付。' },
  { key: 'fde', title: 'FDE 人工', description: '转交 FDE 完成人工交付。' },
]

const ready = computed(() => deliveryReady(props.flow))
const running = computed(() => props.flow.execution.status === 'running')
const selectedLabel = computed(() => {
  const method = methods.find((item) => item.key === props.flow.method)
  return method?.title ?? ''
})

function statusText(status: MockStepStatus): string {
  if (status === 'running') return '执行中'
  if (status === 'success') return '已完成'
  if (status === 'error') return '失败'
  return '待执行'
}

function targets(kind: DeliveryConnectionKind) {
  if (kind === 'k8s') return connections.k8sClusters
  if (kind === 'cloud') return connections.cloudNodes
  if (kind === 'argo') return connections.argoes
  return connections.fdes
}

function selectMethod(kind: DeliveryConnectionKind) {
  if (props.flow.execution.status === 'success' || props.flow.status === 'done') return
  flows.patchDelivery(props.projectId, {
    method: kind,
    targetId: '',
    namespace: '',
    note: '',
    execution: createEmptyStep(),
  })
  error.value = ''
}

const activeTargets = computed(() => (props.flow.method ? targets(props.flow.method) : []))

function selectTarget(event: Event) {
  const targetId = (event.target as HTMLSelectElement).value
  const k8s = connections.k8sClusters.find((item) => item.id === targetId)
  flows.patchDelivery(props.projectId, {
    targetId,
    namespace: k8s?.defaultNamespace ?? props.flow.namespace,
  })
}

function inputValue(event: Event): string {
  return (event.target as HTMLInputElement).value
}

function updateDeliveryField(field: 'namespace' | 'note', value: string): void {
  const patch: Partial<DeliveryFlowState> = {}
  if (field === 'namespace') patch.namespace = value
  if (field === 'note') patch.note = value
  flows.patchDelivery(props.projectId, patch)
}

function execute() {
  if (!props.flow.method || !props.flow.targetId) {
    error.value = '请选择交付方式和目标'
    return
  }
  if (running.value) return
  error.value = ''
  const startedAt = new Date().toISOString()
  flows.patchDeliveryExecution(props.projectId, {
    status: 'running',
    startedAt,
    completedAt: null,
    error: '',
  })
  if (props.flow.method === 'fde') return
  if (timer) clearTimeout(timer)
  timer = setTimeout(() => {
    flows.patchDeliveryExecution(props.projectId, {
      status: 'success',
      completedAt: new Date().toISOString(),
    })
    timer = null
  }, 1100)
}

function markManualDone() {
  if (props.flow.method !== 'fde' || props.flow.execution.status !== 'running') return
  flows.patchDeliveryExecution(props.projectId, {
    status: 'success',
    completedAt: new Date().toISOString(),
  })
}

function confirm() {
  if (!ready.value) {
    error.value = '交付执行成功后才能确认'
    return
  }
  if (flows.confirmDelivery(props.projectId)) {
    error.value = ''
    emit('confirmed')
  }
}

onBeforeUnmount(() => {
  if (timer) {
    clearTimeout(timer)
    flows.patchDeliveryExecution(props.projectId, { ...createEmptyStep() })
  }
})
</script>

<template>
  <section class="delivery-workspace">
    <header class="pipeline-head">
      <div>
        <p class="enterprise-stage-eyebrow">阶段 7</p>
        <h1>交付空间</h1>
        <p>选择一种交付方式，执行成功并确认后完成企业项目交付。</p>
      </div>
      <button
        class="enterprise-primary"
        type="button"
        :disabled="!ready || flow.status === 'done'"
        @click="confirm"
      >
        <i :class="flow.status === 'done' ? 'pi pi-check' : 'pi pi-flag'" />
        {{ flow.status === 'done' ? '交付已完成' : '确认交付完成' }}
      </button>
    </header>

    <p v-if="error" class="enterprise-stage-error">{{ error }}</p>

    <div class="delivery-methods">
      <button
        v-for="method in methods"
        :key="method.key"
        type="button"
        class="delivery-method"
        :class="{ active: flow.method === method.key }"
        :disabled="
          running || flow.execution.status === 'success' || flow.status === 'done'
        "
        @click="selectMethod(method.key)"
      >
        <strong>{{ method.title }}</strong>
        <span>{{ method.description }}</span>
        <small>{{ targets(method.key).length }} 个可用配置</small>
      </button>
    </div>

    <section v-if="flow.method" class="pipeline-card delivery-target">
      <span class="pipeline-index">→</span>
      <div class="pipeline-body">
        <header>
          <strong>交付到{{ selectedLabel }}</strong>
          <span class="pipeline-status">{{ statusText(flow.execution.status) }}</span>
        </header>

        <div class="pipeline-fields">
          <select
            class="pipeline-input"
            :value="flow.targetId"
            :disabled="running || flow.execution.status === 'success'"
            @change="selectTarget"
          >
            <option value="">选择目标</option>
            <option v-for="item in activeTargets" :key="item.id" :value="item.id">
              {{ item.name }}
            </option>
          </select>
          <input
            v-if="flow.method === 'k8s'"
            :value="flow.namespace"
            class="pipeline-input"
            placeholder="命名空间"
            :disabled="running || flow.execution.status === 'success'"
            @input="updateDeliveryField('namespace', inputValue($event))"
          />
          <input
            v-if="flow.method === 'fde'"
            :value="flow.note"
            class="pipeline-input"
            placeholder="交付备注"
            :disabled="running || flow.execution.status === 'success'"
            @input="updateDeliveryField('note', inputValue($event))"
          />
        </div>

        <button
          v-if="flow.execution.status !== 'success'"
          class="pipeline-run"
          type="button"
          :disabled="!flow.targetId || (flow.method !== 'fde' && running)"
          @click="flow.method === 'fde' && flow.execution.status === 'running' ? markManualDone() : execute()"
        >
          {{ flow.method === 'fde' && flow.execution.status === 'running' ? '标记人工完成' : '执行交付' }}
        </button>

        <p v-if="!activeTargets.length" class="pipeline-hint">
          请先在设置中配置{{ selectedLabel }}。<RouterLink to="/settings/delivery-space">去配置</RouterLink>
        </p>
      </div>
    </section>

    <section v-if="ready" class="pipeline-result">
      <h2>交付完成</h2>
      <p>方式：{{ selectedLabel }} · 目标：{{ flow.targetId }}</p>
      <p v-if="flow.namespace">命名空间：{{ flow.namespace }}</p>
      <p v-if="flow.note">备注：{{ flow.note }}</p>
    </section>
  </section>
</template>
```

- [ ] **Step 2: 补交付样式**

在 `packages/web/src/styles/enterprise.css` 末尾加入：

```css
.delivery-methods {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
}

.delivery-method {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 6px;
  min-height: 118px;
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
  text-align: left;
  cursor: pointer;
}

.delivery-method strong {
  color: var(--ink);
  font-size: 13px;
}

.delivery-method span {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.delivery-method small {
  margin-top: auto;
  color: var(--muted);
  font-size: 10px;
}

.delivery-method.active {
  border-color: var(--ink);
  background: var(--soft);
}

.delivery-method:disabled {
  cursor: default;
  opacity: 0.7;
}

.delivery-target {
  margin-top: 4px;
}
```

- [ ] **Step 3: 运行测试、lint、build**

```bash
npm --prefix packages/web run test -- --run
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 通过。

- [ ] **Step 4: Checkpoint**

```bash
make checkpoint
```

---

### Task 8: 编辑页接入阶段 6/7

**Files:**
- Modify: `packages/web/src/views/EnterpriseEditorView.vue`

**Interfaces:**
- Consumes: Task 2 flow store、Task 5 阶段定义、Task 6/7 工作区组件。
- Produces:
  - 阶段 5 完成后激活阶段 6
  - 阶段 6/7 使用 flow overlay
  - 阶段 6/7 聊天只读
  - 两个工作区的确认回调

- [ ] **Step 1: 引入依赖和当前 flow**

在 `EnterpriseEditorView.vue` 的 import 中加入：

```ts
import ArtifactWorkspace from '@/components/enterprise/ArtifactWorkspace.vue'
import DeliveryWorkspace from '@/components/enterprise/DeliveryWorkspace.vue'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import { mergeEnterpriseStageStates } from '@/utils/enterpriseFlow'
```

在 store 初始化后加入：

```ts
const flows = useEnterpriseFlowStore()
```

在 `activeStage` 附近加入：

```ts
const currentFlow = computed(() => flows.get(id.value))
const chatReadOnly = computed(
  () =>
    isHistory.value ||
    displayedStage.value.workspace === 'artifact' ||
    displayedStage.value.workspace === 'delivery',
)

function syncStageOverlay(): void {
  const flow = currentFlow.value
  if (flow) {
    stageStates.value = mergeEnterpriseStageStates(stageStates.value, flow)
  }
}

function ensureArtifactFlow(): void {
  if (stageStates.value.development.status === 'done') {
    flows.ensureArtifact(id.value)
    syncStageOverlay()
  }
}
```

- [ ] **Step 2: 接入阶段加载和全栈开发确认**

`loadStages` 成功分支改为：

```ts
const result = await listProjectStages(projectId)
stageStates.value = stageStatesFromProject(result.stages)
ensureArtifactFlow()
stageLoadError.value = ''
```

`completeCurrentStage` 成功分支在 `applyStageMutation(result)` 后加入：

```ts
if (key === 'development') {
  ensureArtifactFlow()
}
```

- [ ] **Step 3: 增加阶段 6/7 确认回调**

```ts
function confirmArtifactStage(): void {
  if (!flows.confirmArtifact(id.value)) {
    confirmError.value = '请先完成代码上传、Jenkins 构建和 Harbor 推送'
    return
  }
  viewingKey.value = null
  confirmError.value = ''
  syncStageOverlay()
}

function confirmDeliveryStage(): void {
  if (!flows.confirmDelivery(id.value)) {
    confirmError.value = '请先完成所选交付方式'
    return
  }
  viewingKey.value = null
  confirmError.value = ''
  syncStageOverlay()
}
```

`startCurrentStage` 增加守卫：

```ts
if (
  !activeStage.value ||
  activeStage.value.workspace === 'artifact' ||
  activeStage.value.workspace === 'delivery'
) {
  return
}
```

- [ ] **Step 4: 聊天面板只读**

改为：

```vue
<ChatPanel :project-id="id" :readonly="chatReadOnly" required-role-code="team" />
```

- [ ] **Step 5: 修改工作区模板分支**

将 `main` 内分支改为：

```vue
<ArtifactWorkspace
  v-if="displayedStage.workspace === 'artifact' && currentFlow"
  :project-id="id"
  :flow="currentFlow.artifact"
  @confirmed="confirmArtifactStage"
/>
<DeliveryWorkspace
  v-else-if="displayedStage.workspace === 'delivery' && currentFlow"
  :project-id="id"
  :flow="currentFlow.delivery"
  @confirmed="confirmDeliveryStage"
/>
<template v-else-if="displayedStage.workspace !== 'placeholder'">
  <!-- 保留现有 StageWorkspaceHeader / DocumentWorkspace / development 分支 -->
</template>
<PlaceholderWorkspace v-else :stage="displayedStage" />
```

- [ ] **Step 6: 运行测试、lint、build**

```bash
npm --prefix packages/web run test -- --run
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 通过。

- [ ] **Step 7: 手动验收阶段 6/7**

1. 在设置页新增代码仓库、Jenkins、Harbor、K8s、云主机、Argo、FDE。
2. 打开全栈开发已确认的企业项目；阶段 6 应为活跃，聊天只读。
3. 依次上传、构建、推送；确认 Docker 命令、复制、两个下载按钮。
4. 点击“确认制品完成”，阶段 7 激活。
5. 切换四种交付方式，确认只能选一个。
6. 执行 K8s/云主机/Argo；FDE 发起后标记人工完成。
7. 点击“确认交付完成”，页面显示最终摘要。
8. 刷新后确认阶段 6/7 状态仍在。

- [ ] **Step 8: Checkpoint**

```bash
make checkpoint
```

---

### Task 9: 全量验证与回归

**Files:**
- Modify: `docs/superpowers/specs/2026-10-31-enterprise-artifact-delivery-design.md`（仅当实现与规格不一致时同步）

**Interfaces:**
- Consumes: 全部前置任务。
- Produces: 可交付的前端功能。

- [ ] **Step 1: 跑全量前端测试**

```bash
npm --prefix packages/web run test -- --run
```

Expected: 全部通过。

- [ ] **Step 2: 跑 lint 和 build**

```bash
npm --prefix packages/web run lint
npm --prefix packages/web run build
```

Expected: 无错误。

- [ ] **Step 3: 跑仓库质量门禁**

```bash
make quality
make validate
```

Expected: 通过。

- [ ] **Step 4: 手动回归阶段 1–5**

1. 新建企业项目。
2. 需求阶段启动、生成产物、确认。
3. 依次完成概要设计、详细设计、测试用例。
4. 全栈开发确认后进入制品空间。
5. 阶段 1–5 的历史阶段仍可只读查看。
6. 标准项目 `/editor/:id` 不受影响。

- [ ] **Step 5: 手动回归刷新**

1. 添加几条配置，刷新设置页，配置仍在。
2. 完成制品一步，刷新编辑页，步骤和状态仍在。
3. 确认阶段 6 后刷新，阶段 7 仍为活跃。
4. 点击配置编辑，secret 留空保存，原凭据仍在。

- [ ] **Step 6: 最终 Checkpoint**

```bash
make checkpoint
```

---


## 计划自检

- 规格覆盖：设置菜单、七类连接、localStorage、阶段 6/7 overlay、制品三步、独立操作、交付四选一、聊天只读、敏感字段、测试均有对应任务。
- 类型一致性：连接类型、flow 类型、store 方法名在 Task 1/2 定义，后续任务只使用这些名称。
- 后端边界：所有任务只改 `packages/web`，阶段 6/7 不调用后端阶段接口。
- 提交约束：全部任务以 `make checkpoint` 结束，不执行 git commit。

# 超级团队前端 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在 web 与 ops-web 交付「超级团队」前端效果：事件入口卡片、三步门禁流程、三个演示详情页、ops-web 只读审计。

**Architecture:** web 新增 `SuperTeamPanel`（HomeView 条件渲染，保留侧栏）与三个独立全屏演示视图；状态集中在 Pinia `superTeam` store，持久化到 localStorage。ops-web 新增单导航 `SuperTeamView`，三 tab 只读，数据来自本地 mock。全部纯前端，不新增 CP/DP/sandbox 接口。

**Tech Stack:** Vue 3 `<script setup lang="ts">`、Vue Router、Pinia、PrimeVue、PrimeIcons、Vitest（jsdom）。不新增依赖，不新增颜色。

**Spec:** `docs/superpowers/specs/2026-11-09-super-team-design.md`

## Global Constraints

- 仅修改 `packages/web` 与 `packages/ops-web`；不触碰 backend / sandbox / 数据库。
- 不新增 npm 依赖；不使用 `@vue/test-utils`（未安装）。
- 颜色仅 `--ink`、`--text`、`--muted`、`--line`、`--line-strong`、`--surface`、`--canvas`、`--soft`、`--blue`、`--blue-soft`。
- 事件源属于用户：web 可编辑，ops-web 只读。
- 消息队列事件源本轮禁用，状态固定 `coming_soon`。
- 三步严格顺序：`requirements → development → operations`，锁定步骤不可推进。
- 测试命令：`npm test` / `npm run lint` / `npm run format:check`，在 `packages/web` 或 `packages/ops-web` 下执行。
- 提交步骤仅在用户明确要求时执行；本计划先完成文件与验证。

---

### Task 1: web 超级团队类型与纯函数

**Files:**
- Create: `packages/web/src/types/superTeam.ts`
- Create: `packages/web/src/utils/superTeam.ts`
- Test: `packages/web/src/utils/superTeam.test.ts`

**Interfaces:**
- Produces: 所有类型 `EventSourceKind`、`EventSourceStatus`、`EventSourceConfig`、`SuperStepKey`、`SuperStepStatus`、`SuperStepState`、`SuperEventStage`、`SuperEvent`、`AgentActionKind`、`AgentActionStatus`、`AgentActionAudit`、`SuperTeamState`、`EventSourceField`。
- Produces: `SUPER_TEAM_STORAGE_KEY`、`STEP_ORDER`、`sourceMeta(kind)`、`sourceFields(kind)`、`stepTitle(key)`、`stepDescription(key)`、`defaultSources()`、`defaultSteps()`、`defaultEvents()`、`defaultAudit()`、`initialState()`、`loadState(raw)`、`serializeState(state)`、`isStepUnlocked(steps, key)`、`advanceStep(steps, key)`、`sourceStatusLabel(status)`、`stepStatusLabel(status)`、`eventStageLabel(stage)`、`noiseBand(score)`、`semanticBand(score)`。

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/utils/superTeam.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import {
  SUPER_TEAM_STORAGE_KEY,
  advanceStep,
  defaultSources,
  defaultSteps,
  initialState,
  isStepUnlocked,
  loadState,
  noiseBand,
  semanticBand,
  serializeState,
  sourceMeta,
  sourceStatusLabel,
  stepStatusLabel,
  stepTitle,
} from './superTeam'

describe('super team utils', () => {
  it('starts with requirements active and later steps locked', () => {
    const steps = defaultSteps()
    expect(steps.map((step) => step.key)).toEqual(['requirements', 'development', 'operations'])
    expect(steps[0].status).toBe('active')
    expect(steps[1].status).toBe('locked')
    expect(steps[2].status).toBe('locked')
    expect(isStepUnlocked(steps, 'requirements')).toBe(true)
    expect(isStepUnlocked(steps, 'development')).toBe(false)
  })

  it('advances requirements and unlocks development only', () => {
    const steps = advanceStep(defaultSteps(), 'requirements')
    expect(steps[0].status).toBe('done')
    expect(steps[0].progress).toBe(100)
    expect(steps[1].status).toBe('ready')
    expect(steps[2].status).toBe('locked')
  })

  it('does not advance a locked step', () => {
    const steps = defaultSteps()
    expect(advanceStep(steps, 'operations')).toEqual(steps)
  })

  it('advances development and unlocks operations', () => {
    const steps = advanceStep(defaultSteps(), 'requirements')
    const next = advanceStep(steps, 'development')
    expect(next[2].status).toBe('ready')
  })

  it('provides four event sources with queue disabled', () => {
    const sources = defaultSources()
    expect(sources.map((source) => source.kind)).toEqual(['feishu', 'webhook', 'email', 'queue'])
    const queue = sources.find((source) => source.kind === 'queue')
    expect(queue?.enabled).toBe(false)
    expect(queue?.status).toBe('coming_soon')
  })

  it('labels sources and steps', () => {
    expect(sourceMeta('feishu').label).toBe('飞书')
    expect(sourceStatusLabel('connected')).toBe('已连接')
    expect(stepTitle('development')).toBe('系统开发')
    expect(stepStatusLabel('locked')).toBe('锁定')
  })

  it('round-trips state and merges missing fields', () => {
    const state = initialState()
    const raw = serializeState(state)
    expect(loadState(raw).sources).toHaveLength(4)
    const merged = loadState(
      JSON.stringify({
        sources: [{ kind: 'email', enabled: true, fields: { host: 'imap.test' } }],
        steps: [{ key: 'requirements', status: 'done', progress: 100 }],
      }),
    )
    const email = merged.sources.find((source) => source.kind === 'email')
    expect(email?.enabled).toBe(true)
    expect(email?.fields.host).toBe('imap.test')
    expect(merged.steps.find((step) => step.key === 'requirements')?.status).toBe('done')
    expect(merged.steps.find((step) => step.key === 'development')?.status).toBe('locked')
  })

  it('falls back to defaults for invalid storage', () => {
    expect(loadState('not-json').sources).toHaveLength(4)
    expect(loadState(null).steps).toHaveLength(3)
  })

  it('bands scores consistently', () => {
    expect(noiseBand(20)).toBe('low')
    expect(noiseBand(55)).toBe('medium')
    expect(noiseBand(85)).toBe('high')
    expect(semanticBand(75)).toBe('high')
  })

  it('exposes a stable storage key', () => {
    expect(SUPER_TEAM_STORAGE_KEY).toBe('vibebiz:super-team')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npx vitest run src/utils/superTeam.test.ts`
Expected: FAIL — `Cannot find module './superTeam'`.

- [ ] **Step 3: Write types**

Create `packages/web/src/types/superTeam.ts`:

```ts
export type EventSourceKind = 'feishu' | 'webhook' | 'email' | 'queue'
export type EventSourceStatus = 'connected' | 'configured' | 'unconfigured' | 'coming_soon'
export type SuperStepKey = 'requirements' | 'development' | 'operations'
export type SuperStepStatus = 'locked' | 'ready' | 'active' | 'done'
export type SuperEventStage = 'received' | 'filtered' | 'evaluated' | 'routed' | 'ignored'
export type AgentActionKind = 'noise_filter' | 'semantic_eval' | 'artifact' | 'step_advance'
export type AgentActionStatus = 'success' | 'failed' | 'running'

export interface EventSourceField {
  key: string
  label: string
  placeholder: string
  secret?: boolean
}

export interface EventSourceConfig {
  kind: EventSourceKind
  enabled: boolean
  status: EventSourceStatus
  lastEventAt: string | null
  eventCount: number
  fields: Record<string, string>
}

export interface SuperStepState {
  key: SuperStepKey
  status: SuperStepStatus
  progress: number
  updatedAt: string | null
  summary: string
}

export interface SuperEvent {
  id: string
  source: EventSourceKind
  title: string
  receivedAt: string
  noiseScore: number
  semanticScore: number
  stage: SuperEventStage
  routedStep: SuperStepKey | null
}

export interface AgentActionAudit {
  id: string
  eventId: string
  step: SuperStepKey
  agent: string
  action: AgentActionKind
  status: AgentActionStatus
  durationMs: number
  createdAt: string
}

export interface SuperTeamState {
  sources: EventSourceConfig[]
  steps: SuperStepState[]
  events: SuperEvent[]
  audit: AgentActionAudit[]
}
```

- [ ] **Step 4: Write utils**

Create `packages/web/src/utils/superTeam.ts`:

```ts
import type {
  AgentActionAudit,
  EventSourceConfig,
  EventSourceField,
  EventSourceKind,
  EventSourceStatus,
  SuperEvent,
  SuperEventStage,
  SuperStepKey,
  SuperStepState,
  SuperStepStatus,
  SuperTeamState,
} from '@/types/superTeam'

export const SUPER_TEAM_STORAGE_KEY = 'vibebiz:super-team'

export const STEP_ORDER: SuperStepKey[] = ['requirements', 'development', 'operations']

const STEP_TITLES: Record<SuperStepKey, string> = {
  requirements: '需求挖掘',
  development: '系统开发',
  operations: '观测运维',
}

const STEP_DESCRIPTIONS: Record<SuperStepKey, string> = {
  requirements: '监听事件源，生成思维导图、知识图谱与经济可行性文档。',
  development: '按企业七阶段推进，接入事件驱动的需求与变更。',
  operations: '每小时巡检事件与指标，生成可预览报表。',
}

interface SourceMeta {
  label: string
  icon: string
  description: string
}

const SOURCE_META: Record<EventSourceKind, SourceMeta> = {
  feishu: { label: '飞书', icon: 'pi pi-comments', description: '群聊与私聊消息事件' },
  webhook: { label: 'Webhook', icon: 'pi pi-link', description: '接收任意 JSON 事件' },
  email: { label: 'Email', icon: 'pi pi-envelope', description: 'IMAP 轮询邮件事件' },
  queue: { label: '消息队列', icon: 'pi pi-server', description: 'Redis / Kafka 队列，即将支持' },
}

const SOURCE_FIELDS: Record<EventSourceKind, EventSourceField[]> = {
  feishu: [
    { key: 'appId', label: 'App ID', placeholder: 'cli_xxx' },
    { key: 'appSecret', label: 'App Secret', placeholder: '应用密钥', secret: true },
    { key: 'encryptKey', label: 'Encrypt Key', placeholder: '事件加密密钥', secret: true },
    {
      key: 'verificationToken',
      label: 'Verification Token',
      placeholder: '校验 Token',
      secret: true,
    },
  ],
  webhook: [
    { key: 'url', label: '入站地址', placeholder: '/cp/api/v1/superteam/events/webhook/…' },
    { key: 'secret', label: '签名密钥', placeholder: 'HMAC 密钥', secret: true },
  ],
  email: [
    { key: 'host', label: 'IMAP 服务器', placeholder: 'imap.example.com' },
    { key: 'port', label: '端口', placeholder: '993' },
    { key: 'user', label: '邮箱账号', placeholder: 'team@example.com' },
    { key: 'password', label: '授权码', placeholder: '邮箱授权码', secret: true },
    { key: 'intervalMinutes', label: '轮询间隔（分钟）', placeholder: '5' },
  ],
  queue: [],
}

export function sourceMeta(kind: EventSourceKind): SourceMeta {
  return SOURCE_META[kind]
}

export function sourceFields(kind: EventSourceKind): EventSourceField[] {
  return SOURCE_FIELDS[kind]
}

export function stepTitle(key: SuperStepKey): string {
  return STEP_TITLES[key]
}

export function stepDescription(key: SuperStepKey): string {
  return STEP_DESCRIPTIONS[key]
}

export function defaultSources(): EventSourceConfig[] {
  return [
    {
      kind: 'feishu',
      enabled: true,
      status: 'connected',
      lastEventAt: '2026-11-09T09:42:00.000Z',
      eventCount: 128,
      fields: {
        appId: 'cli_demo',
        appSecret: 'demo-secret',
        encryptKey: 'demo-encrypt',
        verificationToken: 'demo-token',
      },
    },
    {
      kind: 'webhook',
      enabled: true,
      status: 'connected',
      lastEventAt: '2026-11-09T09:30:00.000Z',
      eventCount: 64,
      fields: { url: '/cp/api/v1/superteam/events/webhook/demo-token', secret: 'demo-hmac' },
    },
    {
      kind: 'email',
      enabled: false,
      status: 'unconfigured',
      lastEventAt: null,
      eventCount: 0,
      fields: {},
    },
    {
      kind: 'queue',
      enabled: false,
      status: 'coming_soon',
      lastEventAt: null,
      eventCount: 0,
      fields: {},
    },
  ]
}

export function defaultSteps(): SuperStepState[] {
  return [
    {
      key: 'requirements',
      status: 'active',
      progress: 45,
      updatedAt: '2026-11-09T09:00:00.000Z',
      summary: '已生成需求思维导图，可行性文档撰写中。',
    },
    {
      key: 'development',
      status: 'locked',
      progress: 0,
      updatedAt: null,
      summary: '完成需求挖掘后解锁。',
    },
    {
      key: 'operations',
      status: 'locked',
      progress: 0,
      updatedAt: null,
      summary: '完成系统开发后解锁。',
    },
  ]
}

export function defaultEvents(): SuperEvent[] {
  return [
    {
      id: 'evt-1001',
      source: 'feishu',
      title: '客户反馈支付失败，影响下单',
      receivedAt: '2026-11-09T09:42:00.000Z',
      noiseScore: 8,
      semanticScore: 92,
      stage: 'routed',
      routedStep: 'requirements',
    },
    {
      id: 'evt-1002',
      source: 'webhook',
      title: '监控告警：导出任务超时',
      receivedAt: '2026-11-09T09:30:00.000Z',
      noiseScore: 16,
      semanticScore: 84,
      stage: 'routed',
      routedStep: 'requirements',
    },
    {
      id: 'evt-1003',
      source: 'email',
      title: '周报邮件：本周功能使用统计',
      receivedAt: '2026-11-09T09:12:00.000Z',
      noiseScore: 68,
      semanticScore: 41,
      stage: 'filtered',
      routedStep: null,
    },
    {
      id: 'evt-1004',
      source: 'webhook',
      title: '快递物流状态更新',
      receivedAt: '2026-11-09T08:55:00.000Z',
      noiseScore: 91,
      semanticScore: 12,
      stage: 'ignored',
      routedStep: null,
    },
    {
      id: 'evt-1005',
      source: 'feishu',
      title: '运营群讨论续费策略',
      receivedAt: '2026-11-09T08:40:00.000Z',
      noiseScore: 22,
      semanticScore: 77,
      stage: 'evaluated',
      routedStep: null,
    },
    {
      id: 'evt-1006',
      source: 'email',
      title: '客户来信：希望增加批量导出',
      receivedAt: '2026-11-09T08:20:00.000Z',
      noiseScore: 11,
      semanticScore: 88,
      stage: 'received',
      routedStep: null,
    },
  ]
}

export function defaultAudit(): AgentActionAudit[] {
  return [
    {
      id: 'act-1',
      eventId: 'evt-1001',
      step: 'requirements',
      agent: '需求分析师',
      action: 'noise_filter',
      status: 'success',
      durationMs: 320,
      createdAt: '2026-11-09T09:42:02.000Z',
    },
    {
      id: 'act-2',
      eventId: 'evt-1001',
      step: 'requirements',
      agent: '需求分析师',
      action: 'semantic_eval',
      status: 'success',
      durationMs: 1840,
      createdAt: '2026-11-09T09:42:04.000Z',
    },
    {
      id: 'act-3',
      eventId: 'evt-1001',
      step: 'requirements',
      agent: '产品经理',
      action: 'artifact',
      status: 'success',
      durationMs: 5220,
      createdAt: '2026-11-09T09:42:10.000Z',
    },
    {
      id: 'act-4',
      eventId: 'evt-1004',
      step: 'requirements',
      agent: '需求分析师',
      action: 'noise_filter',
      status: 'success',
      durationMs: 210,
      createdAt: '2026-11-09T08:55:01.000Z',
    },
    {
      id: 'act-5',
      eventId: 'evt-1006',
      step: 'requirements',
      agent: '需求分析师',
      action: 'semantic_eval',
      status: 'running',
      durationMs: 0,
      createdAt: '2026-11-09T08:20:03.000Z',
    },
  ]
}

export function initialState(): SuperTeamState {
  return {
    sources: defaultSources(),
    steps: defaultSteps(),
    events: defaultEvents(),
    audit: defaultAudit(),
  }
}

function mergeSources(value: unknown, defaults: EventSourceConfig[]): EventSourceConfig[] {
  const list = Array.isArray(value) ? (value as Partial<EventSourceConfig>[]) : []
  return defaults.map((fallback) => {
    const stored = list.find((item) => item.kind === fallback.kind)
    if (!stored) {
      return fallback
    }
    return {
      ...fallback,
      ...stored,
      kind: fallback.kind,
      fields: { ...fallback.fields, ...(stored.fields ?? {}) },
    }
  })
}

function mergeSteps(value: unknown, defaults: SuperStepState[]): SuperStepState[] {
  const list = Array.isArray(value) ? (value as Partial<SuperStepState>[]) : []
  return defaults.map((fallback) => {
    const stored = list.find((item) => item.key === fallback.key)
    return stored ? { ...fallback, ...stored, key: fallback.key } : fallback
  })
}

export function loadState(raw: string | null): SuperTeamState {
  const base = initialState()
  if (!raw) {
    return base
  }
  let parsed: Partial<SuperTeamState>
  try {
    parsed = JSON.parse(raw) as Partial<SuperTeamState>
  } catch {
    return base
  }
  return {
    sources: mergeSources(parsed.sources, base.sources),
    steps: mergeSteps(parsed.steps, base.steps),
    events: Array.isArray(parsed.events) ? parsed.events : base.events,
    audit: Array.isArray(parsed.audit) ? parsed.audit : base.audit,
  }
}

export function serializeState(state: SuperTeamState): string {
  return JSON.stringify(state)
}

export function isStepUnlocked(steps: SuperStepState[], key: SuperStepKey): boolean {
  return steps.some((step) => step.key === key && step.status !== 'locked')
}

export function advanceStep(steps: SuperStepState[], key: SuperStepKey): SuperStepState[] {
  const index = STEP_ORDER.indexOf(key)
  if (index < 0) {
    return steps
  }
  const current = steps.find((step) => step.key === key)
  if (!current || (current.status !== 'ready' && current.status !== 'active')) {
    return steps
  }
  const now = new Date().toISOString()
  const nextKey = STEP_ORDER[index + 1]
  return steps.map((step) => {
    if (step.key === key) {
      return { ...step, status: 'done', progress: 100, updatedAt: now }
    }
    if (nextKey && step.key === nextKey && step.status === 'locked') {
      return { ...step, status: 'ready', updatedAt: now }
    }
    return step
  })
}

const SOURCE_STATUS_LABEL: Record<EventSourceStatus, string> = {
  connected: '已连接',
  configured: '已配置',
  unconfigured: '未配置',
  coming_soon: '即将支持',
}

const STEP_STATUS_LABEL: Record<SuperStepStatus, string> = {
  locked: '锁定',
  ready: '可开始',
  active: '进行中',
  done: '已完成',
}

const EVENT_STAGE_LABEL: Record<SuperEventStage, string> = {
  received: '已接收',
  filtered: '已过滤',
  evaluated: '已评估',
  routed: '已路由',
  ignored: '已忽略',
}

export function sourceStatusLabel(status: EventSourceStatus): string {
  return SOURCE_STATUS_LABEL[status]
}

export function stepStatusLabel(status: SuperStepStatus): string {
  return STEP_STATUS_LABEL[status]
}

export function eventStageLabel(stage: SuperEventStage): string {
  return EVENT_STAGE_LABEL[stage]
}

export type ScoreBand = 'low' | 'medium' | 'high'

export function noiseBand(score: number): ScoreBand {
  if (score >= 70) {
    return 'high'
  }
  return score >= 40 ? 'medium' : 'low'
}

export function semanticBand(score: number): ScoreBand {
  if (score >= 70) {
    return 'high'
  }
  return score >= 40 ? 'medium' : 'low'
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd packages/web && npx vitest run src/utils/superTeam.test.ts`
Expected: PASS (all tests).

- [ ] **Step 6: Commit (only if user approved commits)**

```bash
git add packages/web/src/types/superTeam.ts packages/web/src/utils/superTeam.ts packages/web/src/utils/superTeam.test.ts
git commit -m "feat(web): add super team types and utils"
```

---

### Task 2: web 超级团队 Pinia store

**Files:**
- Create: `packages/web/src/stores/superTeam.ts`
- Test: `packages/web/src/stores/superTeam.test.ts`

**Interfaces:**
- Consumes: Task 1 的 `initialState`、`loadState`、`serializeState`、`SUPER_TEAM_STORAGE_KEY`、`advanceStep`。
- Produces: `useSuperTeamStore()`，字段 `sources`、`steps`、`events`、`audit`；方法 `updateSource(kind, patch)`、`toggleSource(kind)`、`advance(key)`、`reset()`、`persist()`。

- [ ] **Step 1: Write the failing test**

Create `packages/web/src/stores/superTeam.test.ts`:

```ts
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { SUPER_TEAM_STORAGE_KEY } from '@/utils/superTeam'
import { useSuperTeamStore } from './superTeam'

beforeEach(() => {
  localStorage.clear()
  setActivePinia(createPinia())
})

describe('super team store', () => {
  it('loads defaults', () => {
    const store = useSuperTeamStore()
    expect(store.sources).toHaveLength(4)
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
  })

  it('advances steps and persists', () => {
    const store = useSuperTeamStore()
    store.advance('requirements')
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('done')
    expect(store.steps.find((step) => step.key === 'development')?.status).toBe('ready')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toContain('"done"')
  })

  it('does not advance locked steps', () => {
    const store = useSuperTeamStore()
    store.advance('operations')
    expect(store.steps.find((step) => step.key === 'operations')?.status).toBe('locked')
  })

  it('updates source fields and recomputes status', () => {
    const store = useSuperTeamStore()
    store.updateSource('email', {
      enabled: true,
      fields: { host: 'imap.test', port: '993', user: 'a@b.c', password: 'x', intervalMinutes: '5' },
    })
    const email = store.sources.find((source) => source.kind === 'email')
    expect(email?.status).toBe('connected')
    expect(email?.fields.host).toBe('imap.test')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toContain('imap.test')
  })

  it('turns off a configured source', () => {
    const store = useSuperTeamStore()
    store.updateSource('webhook', { enabled: false })
    const webhook = store.sources.find((source) => source.kind === 'webhook')
    expect(webhook?.status).toBe('configured')
  })

  it('never enables the queue source', () => {
    const store = useSuperTeamStore()
    store.toggleSource('queue')
    store.updateSource('queue', { enabled: true })
    const queue = store.sources.find((source) => source.kind === 'queue')
    expect(queue?.enabled).toBe(false)
    expect(queue?.status).toBe('coming_soon')
  })

  it('resets to defaults', () => {
    const store = useSuperTeamStore()
    store.advance('requirements')
    store.reset()
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toBeNull()
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/web && npx vitest run src/stores/superTeam.test.ts`
Expected: FAIL — `Cannot find module './superTeam'`.

- [ ] **Step 3: Write store**

Create `packages/web/src/stores/superTeam.ts`:

```ts
import { defineStore } from 'pinia'
import { ref } from 'vue'

import type { EventSourceConfig, EventSourceKind, SuperStepKey, SuperTeamState } from '@/types/superTeam'
import {
  SUPER_TEAM_STORAGE_KEY,
  advanceStep,
  initialState,
  loadState,
  serializeState,
} from '@/utils/superTeam'

function readStored(): SuperTeamState {
  try {
    const raw = typeof localStorage === 'undefined' ? null : localStorage.getItem(SUPER_TEAM_STORAGE_KEY)
    return loadState(raw)
  } catch {
    return initialState()
  }
}

function sourceStatusFor(config: EventSourceConfig): EventSourceConfig['status'] {
  if (config.kind === 'queue') {
    return 'coming_soon'
  }
  if (config.enabled) {
    return 'connected'
  }
  const hasFields = Object.values(config.fields).some((value) => value.trim() !== '')
  return hasFields ? 'configured' : 'unconfigured'
}

export const useSuperTeamStore = defineStore('superTeam', () => {
  const initial = readStored()
  const sources = ref<EventSourceConfig[]>(initial.sources)
  const steps = ref(initial.steps)
  const events = ref(initial.events)
  const audit = ref(initial.audit)

  function persist(): void {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(
          SUPER_TEAM_STORAGE_KEY,
          serializeState({
            sources: sources.value,
            steps: steps.value,
            events: events.value,
            audit: audit.value,
          }),
        )
      }
    } catch {
      return
    }
  }

  function updateSource(kind: EventSourceKind, patch: Partial<EventSourceConfig>): void {
    sources.value = sources.value.map((source) => {
      if (source.kind !== kind) {
        return source
      }
      const merged: EventSourceConfig = {
        ...source,
        ...patch,
        kind,
        fields: { ...source.fields, ...(patch.fields ?? {}) },
      }
      if (kind === 'queue') {
        return { ...merged, enabled: false, status: 'coming_soon' }
      }
      return { ...merged, status: sourceStatusFor(merged) }
    })
    persist()
  }

  function toggleSource(kind: EventSourceKind): void {
    if (kind === 'queue') {
      return
    }
    const source = sources.value.find((item) => item.kind === kind)
    if (!source) {
      return
    }
    updateSource(kind, { enabled: !source.enabled })
  }

  function advance(key: SuperStepKey): void {
    steps.value = advanceStep(steps.value, key)
    persist()
  }

  function reset(): void {
    const base = initialState()
    sources.value = base.sources
    steps.value = base.steps
    events.value = base.events
    audit.value = base.audit
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.removeItem(SUPER_TEAM_STORAGE_KEY)
      }
    } catch {
      return
    }
  }

  return {
    sources,
    steps,
    events,
    audit,
    persist,
    updateSource,
    toggleSource,
    advance,
    reset,
  }
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd packages/web && npx vitest run src/stores/superTeam.test.ts`
Expected: PASS.

- [ ] **Step 5: Commit (only if user approved commits)**

```bash
git add packages/web/src/stores/superTeam.ts packages/web/src/stores/superTeam.test.ts
git commit -m "feat(web): add super team store"
```

---

### Task 3: web 事件卡片与配置抽屉

**Files:**
- Create: `packages/web/src/components/superteam/EventSourceCard.vue`
- Create: `packages/web/src/components/superteam/EventSourceDialog.vue`

**Interfaces:**
- Consumes: `sourceMeta`、`sourceStatusLabel`、`sourceFields`、`EventSourceConfig`。
- Produces: `<EventSourceCard :source @configure />`、`<EventSourceDialog v-model:visible :source @save />`。

- [ ] **Step 1: Create EventSourceCard**

`packages/web/src/components/superteam/EventSourceCard.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'

import type { EventSourceConfig } from '@/types/superTeam'
import { sourceMeta, sourceStatusLabel } from '@/utils/superTeam'

const props = defineProps<{ source: EventSourceConfig }>()
const emit = defineEmits<{ configure: [] }>()

const meta = computed(() => sourceMeta(props.source.kind))
const disabled = computed(() => props.source.status === 'coming_soon')
const statusLabel = computed(() => sourceStatusLabel(props.source.status))
</script>

<template>
  <button
    type="button"
    class="super-source-card"
    :class="{ 'is-disabled': disabled, 'is-connected': source.status === 'connected' }"
    :disabled="disabled"
    :aria-label="`${meta.label}事件入口`"
    @click="emit('configure')"
  >
    <span class="super-source-icon"><i :class="meta.icon" /></span>
    <span class="super-source-body">
      <strong>{{ meta.label }}</strong>
      <small>{{ meta.description }}</small>
    </span>
    <span class="super-source-foot">
      <span class="tag" :class="{ 'tag-accent': source.status === 'connected' }">
        {{ statusLabel }}
      </span>
      <span class="super-source-count">{{ source.eventCount }} 条事件</span>
    </span>
  </button>
</template>
```

- [ ] **Step 2: Create EventSourceDialog**

`packages/web/src/components/superteam/EventSourceDialog.vue`:

```vue
<script setup lang="ts">
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import { computed, ref, watch } from 'vue'

import type { EventSourceConfig, EventSourceKind } from '@/types/superTeam'
import { sourceFields, sourceMeta } from '@/utils/superTeam'

const props = defineProps<{ visible: boolean; source: EventSourceConfig | null }>()

const emit = defineEmits<{
  'update:visible': [value: boolean]
  save: [kind: EventSourceKind, fields: Record<string, string>]
}>()

const form = ref<Record<string, string>>({})
const error = ref('')

watch(
  () => props.source,
  (source) => {
    form.value = source ? { ...source.fields } : {}
    error.value = ''
  },
  { immediate: true },
)

const fields = computed(() => (props.source ? sourceFields(props.source.kind) : []))
const meta = computed(() => (props.source ? sourceMeta(props.source.kind) : null))

function close(): void {
  emit('update:visible', false)
}

function save(): void {
  if (!props.source || !meta.value) {
    return
  }
  for (const field of fields.value) {
    if (!form.value[field.key] || !form.value[field.key].trim()) {
      error.value = `请填写${field.label}`
      return
    }
  }
  emit('save', props.source.kind, { ...form.value })
  emit('update:visible', false)
}
</script>

<template>
  <Dialog
    :visible="visible"
    :header="meta ? `配置${meta.label}事件入口` : '配置事件入口'"
    modal
    :style="{ width: '560px' }"
    @update:visible="emit('update:visible', $event)"
  >
    <p class="superteam-dialog-note">
      配置归属当前用户，保存在本地演示存储；ops-web 仅可查看，不可编辑。
    </p>
    <div class="superteam-dialog-form">
      <label v-for="field in fields" :key="field.key" class="superteam-field">
        <span>{{ field.label }}</span>
        <InputText
          v-model="form[field.key]"
          :type="field.secret ? 'password' : 'text'"
          :placeholder="field.placeholder"
        />
      </label>
      <p v-if="error" class="superteam-error" role="alert">{{ error }}</p>
    </div>
    <template #footer>
      <Button label="取消" text @click="close" />
      <Button label="保存配置" @click="save" />
    </template>
  </Dialog>
</template>
```

- [ ] **Step 3: Typecheck**

Run: `cd packages/web && npx vue-tsc --noEmit`
Expected: PASS (no type errors).

- [ ] **Step 4: Commit (only if user approved commits)**

```bash
git add packages/web/src/components/superteam/EventSourceCard.vue packages/web/src/components/superteam/EventSourceDialog.vue
git commit -m "feat(web): add super team event source cards"
```

---

### Task 4: web 首页面板、步骤卡片、路由与菜单

**Files:**
- Create: `packages/web/src/components/superteam/StepCard.vue`
- Create: `packages/web/src/components/superteam/SuperTeamPanel.vue`
- Create: `packages/web/src/styles/superteam.css`
- Modify: `packages/web/src/views/HomeView.vue`
- Modify: `packages/web/src/components/home/HomeSidebar.vue`
- Modify: `packages/web/src/router/index.ts`

**Interfaces:**
- Consumes: Task 2 store、Task 3 组件、Task 1 `stepTitle`、`stepDescription`、`stepStatusLabel`。
- Produces: 路由 `super-team`、`super-team-requirements`、`super-team-development`、`super-team-operations`（后三者视图在 Task 5-7 创建；本任务先建占位导入时需等视图文件存在，因此本任务只加 `super-team` 路由，Task 5-7 再补详情路由）。

- [ ] **Step 1: Create StepCard**

`packages/web/src/components/superteam/StepCard.vue`:

```vue
<script setup lang="ts">
import { computed } from 'vue'

import type { SuperStepState } from '@/types/superTeam'
import { stepDescription, stepStatusLabel, stepTitle } from '@/utils/superTeam'

const props = defineProps<{ step: SuperStepState; index: number }>()
const emit = defineEmits<{ open: [] }>()

const locked = computed(() => props.step.status === 'locked')
const actionLabel = computed(() => {
  if (props.step.status === 'ready') {
    return '开始'
  }
  if (props.step.status === 'active') {
    return '继续'
  }
  if (props.step.status === 'done') {
    return '查看'
  }
  return '未解锁'
})

function open(): void {
  if (!locked.value) {
    emit('open')
  }
}
</script>

<template>
  <article class="super-step-card" :class="`is-${step.status}`">
    <header class="super-step-head">
      <span class="super-step-index">{{ index + 1 }}</span>
      <span class="tag">{{ stepStatusLabel(step.status) }}</span>
    </header>
    <h3>{{ stepTitle(step.key) }}</h3>
    <p>{{ stepDescription(step.key) }}</p>
    <div class="super-step-progress" :aria-label="`进度 ${step.progress}%`">
      <span :style="{ width: `${step.progress}%` }" />
    </div>
    <small class="super-step-summary">{{ step.summary }}</small>
    <button class="super-step-action" type="button" :disabled="locked" @click="open">
      {{ actionLabel }}
    </button>
  </article>
</template>
```

- [ ] **Step 2: Create SuperTeamPanel**

`packages/web/src/components/superteam/SuperTeamPanel.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EventSourceCard from '@/components/superteam/EventSourceCard.vue'
import EventSourceDialog from '@/components/superteam/EventSourceDialog.vue'
import StepCard from '@/components/superteam/StepCard.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import type { EventSourceConfig, EventSourceKind, SuperStepKey } from '@/types/superTeam'
import '@/styles/superteam.css'

const router = useRouter()
const store = useSuperTeamStore()

const dialogVisible = ref(false)
const activeSource = ref<EventSourceConfig | null>(null)

const connected = computed(
  () => store.sources.filter((source) => source.status === 'connected').length,
)
const routed = computed(() => store.events.filter((event) => event.stage === 'routed').length)

const stepRoutes: Record<SuperStepKey, string> = {
  requirements: 'super-team-requirements',
  development: 'super-team-development',
  operations: 'super-team-operations',
}

function openSource(source: EventSourceConfig): void {
  if (source.status === 'coming_soon') {
    return
  }
  activeSource.value = source
  dialogVisible.value = true
}

function saveSource(kind: EventSourceKind, fields: Record<string, string>): void {
  store.updateSource(kind, { fields, enabled: true })
}

function openStep(key: SuperStepKey): void {
  void router.push({ name: stepRoutes[key] })
}
</script>

<template>
  <section class="superteam-page">
    <header class="superteam-head">
      <p class="superteam-eyebrow">SUPER TEAM</p>
      <h1>超级团队</h1>
      <p class="superteam-subtitle">由事件驱动的多 Agent 团队，覆盖需求挖掘、系统开发与观测运维。</p>
      <div class="superteam-stats">
        <span><strong>{{ store.events.length }}</strong> 今日事件</span>
        <span><strong>{{ connected }}</strong> 已连接入口</span>
        <span><strong>{{ routed }}</strong> 已路由</span>
      </div>
    </header>

    <section class="superteam-section">
      <div class="superteam-section-head">
        <h2>事件入口</h2>
        <span>配置归属当前用户，ops-web 只读</span>
      </div>
      <div class="superteam-source-grid">
        <EventSourceCard
          v-for="source in store.sources"
          :key="source.kind"
          :source="source"
          @configure="openSource(source)"
        />
      </div>
    </section>

    <section class="superteam-section">
      <div class="superteam-section-head">
        <h2>执行流程</h2>
        <span>必须按顺序完成</span>
      </div>
      <div class="superteam-step-row">
        <StepCard
          v-for="(step, index) in store.steps"
          :key="step.key"
          :step="step"
          :index="index"
          @open="openStep(step.key)"
        />
      </div>
    </section>

    <EventSourceDialog
      v-model:visible="dialogVisible"
      :source="activeSource"
      @save="saveSource"
    />
  </section>
</template>
```

- [ ] **Step 3: Create shared styles**

Create `packages/web/src/styles/superteam.css` with the full block:

```css
.superteam-page {
  width: 100%;
  min-height: 100%;
  padding: 32px 32px 48px;
}

.superteam-head,
.superteam-section {
  width: 100%;
  max-width: 1080px;
  margin: 0 auto;
}

.superteam-head {
  padding: 12px 0 28px;
  text-align: center;
}

.superteam-eyebrow {
  margin: 0;
  color: var(--muted);
  font-size: 11px;
  letter-spacing: 0.24em;
}

.superteam-head h1 {
  margin: 10px 0 8px;
  font-size: 30px;
}

.superteam-subtitle {
  margin: 0 auto;
  max-width: 620px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.7;
}

.superteam-stats {
  display: flex;
  justify-content: center;
  gap: 28px;
  margin-top: 20px;
  color: var(--muted);
  font-size: 12px;
}

.superteam-stats strong {
  color: var(--ink);
  font-size: 18px;
  margin-right: 4px;
}

.superteam-section {
  margin-top: 28px;
}

.superteam-section-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 14px;
}

.superteam-section-head h2 {
  margin: 0;
  font-size: 16px;
}

.superteam-section-head span {
  color: var(--muted);
  font-size: 12px;
}

.superteam-source-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 14px;
}

.super-source-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 18px;
  color: var(--text);
  text-align: left;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
  cursor: pointer;
  transition:
    border-color 150ms ease,
    transform 150ms ease;
}

.super-source-card:hover:not(:disabled) {
  border-color: var(--line-strong);
  transform: translateY(-1px);
}

.super-source-card.is-connected {
  border-color: rgba(37, 99, 235, 0.35);
}

.super-source-card.is-disabled {
  color: var(--muted);
  cursor: not-allowed;
  opacity: 0.6;
}

.super-source-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  color: var(--blue);
  background: var(--blue-soft);
  border-radius: 10px;
}

.super-source-body {
  display: grid;
  gap: 6px;
}

.super-source-body strong {
  font-size: 14px;
}

.super-source-body small {
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
}

.super-source-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.super-source-count {
  color: var(--muted);
  font-size: 11px;
}

.superteam-step-row {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16px;
}

.super-step-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 22px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.super-step-card.is-locked {
  color: var(--muted);
  background: var(--soft);
}

.super-step-card.is-active {
  border-color: rgba(37, 99, 235, 0.45);
}

.super-step-card.is-done {
  background: var(--soft);
}

.super-step-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.super-step-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  color: var(--ink);
  font-size: 13px;
  border: 1px solid var(--line-strong);
  border-radius: 999px;
}

.super-step-card.is-active .super-step-index {
  color: var(--blue);
  border-color: var(--blue);
}

.super-step-card h3 {
  margin: 0;
  font-size: 18px;
}

.super-step-card p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.7;
}

.super-step-progress {
  height: 6px;
  overflow: hidden;
  background: var(--soft);
  border-radius: 999px;
}

.super-step-progress span {
  display: block;
  height: 100%;
  background: var(--blue);
  border-radius: 999px;
}

.super-step-card.is-done .super-step-progress span {
  background: var(--muted);
}

.super-step-summary {
  color: var(--muted);
  font-size: 11px;
}

.super-step-action {
  margin-top: auto;
  padding: 9px 14px;
  color: var(--surface);
  background: var(--blue);
  border: 1px solid var(--blue);
  border-radius: 999px;
  cursor: pointer;
}

.super-step-action:disabled {
  color: var(--muted);
  background: transparent;
  border-color: var(--line-strong);
  cursor: not-allowed;
}

.superteam-dialog-note {
  margin: 0 0 16px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.6;
}

.superteam-dialog-form {
  display: grid;
  gap: 14px;
}

.superteam-field {
  display: grid;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.superteam-error {
  margin: 0;
  color: var(--ink);
  font-size: 12px;
  font-weight: 500;
}

.superteam-editor {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;
  background: var(--canvas);
}

.superteam-editor-body {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-columns: 360px minmax(0, 1fr);
  gap: 16px;
  padding: 16px;
  overflow: hidden;
}

.superteam-chat {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 12px;
  padding: 18px;
  overflow-y: auto;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.superteam-chat-bubble {
  padding: 12px 14px;
  background: var(--soft);
  border-radius: 12px;
  font-size: 13px;
  line-height: 1.7;
}

.superteam-chat-bubble.agent {
  background: var(--blue-soft);
}

.superteam-chat-role {
  display: block;
  margin-bottom: 4px;
  color: var(--muted);
  font-size: 11px;
}

.superteam-chat-input {
  display: flex;
  gap: 8px;
  margin-top: auto;
  padding-top: 12px;
  border-top: 1px solid var(--line);
}

.superteam-chat-input input {
  flex: 1;
  padding: 9px 12px;
  background: var(--surface);
  border: 1px solid var(--line-strong);
  border-radius: 10px;
}

.superteam-workbench {
  display: flex;
  min-height: 0;
  flex-direction: column;
  padding: 18px;
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.superteam-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  padding-bottom: 14px;
  border-bottom: 1px solid var(--line);
}

.superteam-tab {
  padding: 7px 14px;
  color: var(--muted);
  background: transparent;
  border: 1px solid transparent;
  border-radius: 999px;
  cursor: pointer;
}

.superteam-tab.active {
  color: var(--blue);
  background: var(--blue-soft);
  border-color: rgba(37, 99, 235, 0.3);
}

.superteam-tab-body {
  min-height: 0;
  flex: 1;
  padding-top: 16px;
  overflow-y: auto;
}

.superteam-footer-action {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--line);
}

.superteam-footer-action p {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.superteam-primary {
  padding: 9px 16px;
  color: var(--surface);
  background: var(--blue);
  border: 1px solid var(--blue);
  border-radius: 999px;
  cursor: pointer;
}

.superteam-primary:disabled {
  color: var(--muted);
  background: transparent;
  border-color: var(--line-strong);
  cursor: not-allowed;
}

.super-mindmap {
  display: grid;
  justify-items: center;
  gap: 22px;
  padding: 18px;
}

.super-mindmap-root {
  padding: 10px 22px;
  color: var(--surface);
  background: var(--blue);
  border-radius: 999px;
  font-size: 14px;
}

.super-mindmap-branches {
  display: grid;
  width: 100%;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.super-mindmap-branch {
  padding: 14px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.super-mindmap-branch h4 {
  margin: 0 0 8px;
  font-size: 13px;
}

.super-mindmap-branch ul {
  margin: 0;
  padding-left: 16px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.9;
}

.super-graph {
  display: grid;
  place-items: center;
  padding: 12px;
}

.super-graph svg {
  width: 100%;
  max-width: 640px;
}

.super-graph-edge {
  stroke: var(--line-strong);
  stroke-width: 1.5;
}

.super-graph-node {
  fill: var(--soft);
  stroke: var(--line-strong);
  stroke-width: 1.5;
}

.super-graph-node.is-source {
  fill: var(--blue-soft);
  stroke: var(--blue);
}

.super-graph-node.is-metric {
  fill: var(--surface);
  stroke: var(--muted);
}

.super-graph-label {
  fill: var(--text);
  font-size: 11px;
}

.super-doc {
  display: grid;
  gap: 16px;
  max-width: 720px;
  padding: 8px 4px;
}

.super-doc h3 {
  margin: 0;
  font-size: 16px;
}

.super-doc p {
  margin: 0;
  color: var(--text);
  font-size: 13px;
  line-height: 1.9;
}

.super-doc ul {
  margin: 0;
  padding-left: 18px;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.9;
}

.super-event-list {
  display: grid;
  gap: 10px;
}

.super-event-row {
  display: grid;
  grid-template-columns: 90px minmax(0, 1fr) auto;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.super-event-row.is-ignored {
  color: var(--muted);
  background: var(--soft);
}

.super-event-row strong {
  font-size: 13px;
}

.super-event-row small {
  display: block;
  color: var(--muted);
  font-size: 11px;
}

.super-event-scores {
  display: flex;
  gap: 12px;
  color: var(--muted);
  font-size: 11px;
}

.super-stage-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  padding: 14px 16px;
  background: var(--surface);
  border-bottom: 1px solid var(--line);
}

.super-stage-chip {
  padding: 6px 12px;
  color: var(--muted);
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
  font-size: 12px;
}

.super-stage-chip.is-done {
  color: var(--ink);
  border-color: var(--line-strong);
}

.super-stage-chip.is-active {
  color: var(--blue);
  background: var(--blue-soft);
  border-color: rgba(37, 99, 235, 0.35);
}

.super-inject-panel {
  display: grid;
  gap: 10px;
  margin-top: 16px;
}

.super-inject-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 12px;
  font-size: 12px;
}

.super-chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.super-chart-card {
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
}

.super-chart-card h4 {
  margin: 0 0 4px;
  font-size: 13px;
}

.super-chart-card > span {
  color: var(--muted);
  font-size: 11px;
}

.super-chart-card strong {
  display: block;
  margin: 8px 0 12px;
  font-size: 22px;
}

.super-chart-card svg {
  width: 100%;
  height: 90px;
}

.super-chart-line {
  fill: none;
  stroke: var(--blue);
  stroke-width: 2;
}

.super-chart-area {
  fill: var(--blue-soft);
  opacity: 0.7;
}

.super-task-list {
  display: grid;
  gap: 10px;
}

.super-task-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 14px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.super-task-item strong {
  font-size: 13px;
}

.super-task-item small {
  display: block;
  color: var(--muted);
  font-size: 11px;
}

@media (max-width: 900px) {
  .superteam-page {
    padding: 24px 16px 32px;
  }

  .superteam-source-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .superteam-step-row {
    grid-template-columns: minmax(0, 1fr);
  }

  .superteam-editor-body {
    grid-template-columns: minmax(0, 1fr);
  }

  .super-mindmap-branches {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .super-chart-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}

@media (max-width: 640px) {
  .superteam-source-grid,
  .super-mindmap-branches {
    grid-template-columns: minmax(0, 1fr);
  }

  .super-event-row {
    grid-template-columns: minmax(0, 1fr);
  }
}
```

- [ ] **Step 4: Wire HomeSidebar**

In `packages/web/src/components/home/HomeSidebar.vue`, add to the `navigation` array after `企业工程开发`:

```ts
  { label: '超级团队', icon: 'pi pi-users', route: 'super-team', visible: true },
```

- [ ] **Step 5: Wire HomeView**

In `packages/web/src/views/HomeView.vue`:

Add import near other component imports:

```ts
import SuperTeamPanel from '@/components/superteam/SuperTeamPanel.vue'
```

Add computed near other route flags:

```ts
const isSuperTeamPage = computed(() => route.name === 'super-team')
```

Add branch in the template before the final `<div v-else class="home-main-inner">`:

```vue
      <SuperTeamPanel v-else-if="isSuperTeamPage" />
```

- [ ] **Step 6: Wire router (home route only for now)**

In `packages/web/src/router/index.ts`, add route after `scheduled-tasks`:

```ts
    { path: '/super-team', name: 'super-team', component: HomeView },
```

Add title branch in `afterEach` before `name.startsWith('settings')`:

```ts
                  : name.startsWith('super-team')
                    ? 'Vibebiz · 超级团队'
```

- [ ] **Step 7: Typecheck and lint**

Run:

```bash
cd packages/web && npx vue-tsc --noEmit
cd packages/web && npm run lint
```

Expected: PASS.

- [ ] **Step 8: Commit (only if user approved commits)**

```bash
git add packages/web/src/components/superteam packages/web/src/styles/superteam.css packages/web/src/views/HomeView.vue packages/web/src/components/home/HomeSidebar.vue packages/web/src/router/index.ts
git commit -m "feat(web): add super team home panel and navigation"
```

---

### Task 5: web 需求挖掘演示页

**Files:**
- Create: `packages/web/src/components/superteam/MindMap.vue`
- Create: `packages/web/src/components/superteam/KnowledgeGraph.vue`
- Create: `packages/web/src/views/SuperTeamRequirementsView.vue`
- Modify: `packages/web/src/router/index.ts`

**Interfaces:**
- Consumes: Task 2 store、Task 1 `eventStageLabel`、`noiseBand`、`semanticBand`、`stepStatusLabel`。
- Produces: 路由 `super-team-requirements`。

- [ ] **Step 1: Create MindMap**

`packages/web/src/components/superteam/MindMap.vue`:

```vue
<script setup lang="ts">
const branches = [
  { title: '场景', leaves: ['支付失败反馈', '登录异常', '导出超时'] },
  { title: '用户', leaves: ['运营人员', '财务审核', '外部客户'] },
  { title: '价值', leaves: ['降低人工排查', '缩短响应时间', '提升续费率'] },
  { title: '风险', leaves: ['数据隐私', '噪声误判', '模型成本'] },
]
</script>

<template>
  <div class="super-mindmap">
    <div class="super-mindmap-root">需求洞察</div>
    <div class="super-mindmap-branches">
      <section v-for="branch in branches" :key="branch.title" class="super-mindmap-branch">
        <h4>{{ branch.title }}</h4>
        <ul>
          <li v-for="leaf in branch.leaves" :key="leaf">{{ leaf }}</li>
        </ul>
      </section>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Create KnowledgeGraph**

`packages/web/src/components/superteam/KnowledgeGraph.vue`:

```vue
<script setup lang="ts">
const nodes = [
  { id: 'events', label: '事件源', x: 90, y: 80, kind: 'source' },
  { id: 'pay', label: '支付失败', x: 250, y: 50, kind: 'entity' },
  { id: 'export', label: '导出超时', x: 250, y: 160, kind: 'entity' },
  { id: 'retention', label: '续费率', x: 440, y: 50, kind: 'metric' },
  { id: 'ops', label: '运维效率', x: 440, y: 160, kind: 'metric' },
  { id: 'opportunity', label: '机会', x: 570, y: 105, kind: 'source' },
]

const edges = [
  { id: 'e1', x1: 124, y1: 80, x2: 216, y2: 50 },
  { id: 'e2', x1: 124, y1: 80, x2: 216, y2: 160 },
  { id: 'e3', x1: 284, y1: 50, x2: 406, y2: 50 },
  { id: 'e4', x1: 284, y1: 160, x2: 406, y2: 160 },
  { id: 'e5', x1: 474, y1: 50, x2: 536, y2: 105 },
  { id: 'e6', x1: 474, y1: 160, x2: 536, y2: 105 },
]
</script>

<template>
  <div class="super-graph">
    <svg viewBox="0 0 660 220" role="img" aria-label="需求知识图谱">
      <line
        v-for="edge in edges"
        :key="edge.id"
        :x1="edge.x1"
        :y1="edge.y1"
        :x2="edge.x2"
        :y2="edge.y2"
        class="super-graph-edge"
      />
      <g
        v-for="node in nodes"
        :key="node.id"
        :transform="`translate(${node.x}, ${node.y})`"
      >
        <circle r="34" :class="['super-graph-node', `is-${node.kind}`]" />
        <text text-anchor="middle" dy="4" class="super-graph-label">{{ node.label }}</text>
      </g>
    </svg>
  </div>
</template>
```

- [ ] **Step 3: Create requirements view**

`packages/web/src/views/SuperTeamRequirementsView.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import KnowledgeGraph from '@/components/superteam/KnowledgeGraph.vue'
import MindMap from '@/components/superteam/MindMap.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import type { SuperEvent } from '@/types/superTeam'
import { eventStageLabel, noiseBand, semanticBand } from '@/utils/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'mindmap' | 'graph' | 'doc' | 'events'

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('mindmap')

const tabs: { key: TabKey; label: string }[] = [
  { key: 'mindmap', label: '思维导图' },
  { key: 'graph', label: '知识图谱' },
  { key: 'doc', label: '可行性文档' },
  { key: 'events', label: '事件流' },
]

const step = computed(() => store.steps.find((item) => item.key === 'requirements'))
const events = computed(() => store.events)

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('requirements')
  back()
}

function eventClass(event: SuperEvent): Record<string, boolean> {
  return { 'is-ignored': event.stage === 'ignored' || noiseBand(event.noiseScore) === 'high' }
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString('zh-CN')
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="需求挖掘"
      :project-state="step?.status ?? 'active'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">需求分析师</span>
          已接收 128 条事件，过滤 92 条噪音；当前聚焦支付失败与导出超时两类问题。
        </div>
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">产品经理</span>
          已生成需求思维导图与知识图谱，正在补充经济可行性分析。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">事件摘要</span>
          支付失败影响下单转化，建议优先排查支付网关可用性。
        </div>
        <div class="superteam-chat-input">
          <input type="text" placeholder="本轮为演示效果，输入已禁用" disabled />
        </div>
      </aside>

      <main class="superteam-workbench">
        <div class="superteam-tabs">
          <button
            v-for="item in tabs"
            :key="item.key"
            type="button"
            class="superteam-tab"
            :class="{ active: tab === item.key }"
            @click="tab = item.key"
          >
            {{ item.label }}
          </button>
        </div>

        <div class="superteam-tab-body">
          <MindMap v-if="tab === 'mindmap'" />
          <KnowledgeGraph v-else-if="tab === 'graph'" />
          <div v-else-if="tab === 'doc'" class="super-doc">
            <h3>经济可行性分析</h3>
            <p>
              事件驱动需求挖掘可把客户反馈、监控告警与邮件线索统一收敛，减少人工整理成本。
              以当前 128 条/日事件量估算，噪音过滤可减少约 70% 的无效阅读。
            </p>
            <ul>
              <li>市场规模：企业协作与运维自动化持续增长。</li>
              <li>成本结构：模型调用 + 人工确认，按事件量线性增长。</li>
              <li>收益：缩短需求响应时间，提升续费率与客户满意度。</li>
              <li>风险：数据隐私、误判与模型成本需要持续监控。</li>
            </ul>
            <p>建议先以小范围事件源试点，验证噪音阈值与语义路由准确率后再扩大接入。</p>
          </div>
          <div v-else class="super-event-list">
            <article
              v-for="event in events"
              :key="event.id"
              class="super-event-row"
              :class="eventClass(event)"
            >
              <span class="tag">{{ event.source }}</span>
              <div>
                <strong>{{ event.title }}</strong>
                <small>{{ event.id }} · {{ formatTime(event.receivedAt) }}</small>
              </div>
              <div class="super-event-scores">
                <span>噪音 {{ event.noiseScore }}</span>
                <span>语义 {{ event.semanticScore }}</span>
                <span>{{ eventStageLabel(event.stage) }}</span>
              </div>
            </article>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后解锁「系统开发」。</p>
          <button class="superteam-primary" type="button" @click="complete">
            完成本阶段，解锁系统开发
          </button>
        </div>
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 4: Add route**

In `packages/web/src/router/index.ts`:

Add import:

```ts
import SuperTeamRequirementsView from '@/views/SuperTeamRequirementsView.vue'
```

Add route after `super-team`:

```ts
    {
      path: '/super-team/requirements',
      name: 'super-team-requirements',
      component: SuperTeamRequirementsView,
    },
```

- [ ] **Step 5: Typecheck and lint**

Run:

```bash
cd packages/web && npx vue-tsc --noEmit
cd packages/web && npm run lint
```

Expected: PASS.

- [ ] **Step 6: Commit (only if user approved commits)**

```bash
git add packages/web/src/components/superteam/MindMap.vue packages/web/src/components/superteam/KnowledgeGraph.vue packages/web/src/views/SuperTeamRequirementsView.vue packages/web/src/router/index.ts
git commit -m "feat(web): add super team requirements demo view"
```

---

### Task 6: web 系统开发演示页

**Files:**
- Create: `packages/web/src/views/SuperTeamDevelopmentView.vue`
- Modify: `packages/web/src/router/index.ts`

**Interfaces:**
- Consumes: Task 2 store、`stepStatusLabel`。
- Produces: 路由 `super-team-development`。

- [ ] **Step 1: Create development view**

`packages/web/src/views/SuperTeamDevelopmentView.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'preview' | 'code' | 'terminal'

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('preview')

const step = computed(() => store.steps.find((item) => item.key === 'development'))

const stages = [
  { key: 'requirements', title: '需求梳理', status: 'done' },
  { key: 'architecture', title: '概要设计', status: 'done' },
  { key: 'detail', title: '详细设计', status: 'active' },
  { key: 'test', title: '测试用例', status: 'pending' },
  { key: 'development', title: '全栈开发', status: 'pending' },
  { key: 'artifacts', title: '制品空间', status: 'pending' },
  { key: 'delivery', title: '交付空间', status: 'pending' },
]

const injectedEvents = computed(() =>
  store.events.filter((event) => event.stage === 'routed' || event.stage === 'evaluated'),
)

const tabs: { key: TabKey; label: string }[] = [
  { key: 'preview', label: '应用查看器' },
  { key: 'code', label: '代码' },
  { key: 'terminal', label: '终端' },
]

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('development')
  back()
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="系统开发"
      :project-state="step?.status ?? 'ready'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="super-stage-bar">
      <span
        v-for="stage in stages"
        :key="stage.key"
        class="super-stage-chip"
        :class="{ 'is-done': stage.status === 'done', 'is-active': stage.status === 'active' }"
      >
        {{ stage.title }}
      </span>
    </div>

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">架构师</span>
          已采纳需求挖掘产出的支付可用性需求，正在拆解支付网关与重试策略。
        </div>
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">测试工程师</span>
          已补充支付失败与导出超时的回归用例。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">事件注入</span>
          当前有 {{ injectedEvents.length }} 条事件进入开发流程，均已通过噪音过滤与语义评估。
        </div>
        <div class="superteam-chat-input">
          <input type="text" placeholder="本轮为演示效果，输入已禁用" disabled />
        </div>
      </aside>

      <main class="superteam-workbench">
        <div class="superteam-tabs">
          <button
            v-for="item in tabs"
            :key="item.key"
            type="button"
            class="superteam-tab"
            :class="{ active: tab === item.key }"
            @click="tab = item.key"
          >
            {{ item.label }}
          </button>
        </div>

        <div class="superteam-tab-body">
          <div v-if="tab === 'preview'" class="super-doc">
            <h3>应用查看器（演示）</h3>
            <p>此处复用编辑器预览区效果；本轮为静态占位，不加载真实项目。</p>
          </div>
          <div v-else-if="tab === 'code'" class="super-doc">
            <h3>代码（演示）</h3>
            <p>后续接入真实工作区文件树与只读代码视图。</p>
          </div>
          <div v-else class="super-doc">
            <h3>终端（演示）</h3>
            <p>构建、测试与运行日志将在这里实时输出。</p>
          </div>

          <div class="super-inject-panel">
            <h4>事件注入</h4>
            <div v-for="event in injectedEvents" :key="event.id" class="super-inject-item">
              <span>{{ event.title }}</span>
              <span class="tag">已进入开发</span>
            </div>
            <p v-if="injectedEvents.length === 0" class="superteam-subtitle">暂无事件进入开发流程。</p>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后解锁「观测运维」。</p>
          <button
            class="superteam-primary"
            type="button"
            :disabled="step?.status === 'locked'"
            @click="complete"
          >
            完成本阶段，解锁观测运维
          </button>
        </div>
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Add route**

In `packages/web/src/router/index.ts`:

Add import:

```ts
import SuperTeamDevelopmentView from '@/views/SuperTeamDevelopmentView.vue'
```

Add route after requirements route:

```ts
    {
      path: '/super-team/development',
      name: 'super-team-development',
      component: SuperTeamDevelopmentView,
    },
```

- [ ] **Step 3: Typecheck and lint**

Run:

```bash
cd packages/web && npx vue-tsc --noEmit
cd packages/web && npm run lint
```

Expected: PASS.

- [ ] **Step 4: Commit (only if user approved commits)**

```bash
git add packages/web/src/views/SuperTeamDevelopmentView.vue packages/web/src/router/index.ts
git commit -m "feat(web): add super team development demo view"
```

---

### Task 7: web 观测运维演示页

**Files:**
- Create: `packages/web/src/views/SuperTeamOperationsView.vue`
- Modify: `packages/web/src/router/index.ts`

**Interfaces:**
- Consumes: Task 2 store。
- Produces: 路由 `super-team-operations`。

- [ ] **Step 1: Create operations view**

`packages/web/src/views/SuperTeamOperationsView.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'metrics' | 'reports' | 'tasks' | 'events'

interface MetricCard {
  title: string
  value: string
  caption: string
  values: number[]
}

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('metrics')

const step = computed(() => store.steps.find((item) => item.key === 'operations'))

const metrics: MetricCard[] = [
  { title: '今日事件', value: '128', caption: '较昨日 +12%', values: [18, 24, 20, 32, 28, 36, 42] },
  { title: '有效事件', value: '36', caption: '噪音过滤后', values: [6, 8, 7, 12, 10, 14, 16] },
  { title: '已路由', value: '24', caption: '进入三步流程', values: [4, 5, 6, 8, 7, 9, 11] },
  { title: '告警', value: '3', caption: '待处理', values: [2, 1, 3, 2, 1, 2, 3] },
]

const tasks = [
  { name: '每小时巡检', schedule: '每 1 小时', status: '启用', lastRun: '2026-11-09 09:00', nextRun: '2026-11-09 10:00' },
  { name: '每日指标汇总', schedule: '每日 08:00', status: '启用', lastRun: '2026-11-09 08:00', nextRun: '2026-11-10 08:00' },
  { name: '周报生成', schedule: '每周一 09:00', status: '暂停', lastRun: '2026-11-03 09:00', nextRun: '2026-11-10 09:00' },
]

const reports = [
  { name: '支付可用性日报', updatedAt: '2026-11-09 09:00', status: '可预览' },
  { name: '需求转化周报', updatedAt: '2026-11-08 18:00', status: '可预览' },
  { name: '运维告警月报', updatedAt: '2026-11-01 09:00', status: '可预览' },
]

const events = computed(() => store.events)

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('operations')
  back()
}

function linePoints(values: number[], width = 300, height = 80): string {
  const max = Math.max(...values, 1)
  return values
    .map((value, index) => {
      const x = values.length === 1 ? 0 : (index / (values.length - 1)) * width
      const y = height - (value / max) * height
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

function areaPoints(values: number[], width = 300, height = 80): string {
  return `0,${height} ${linePoints(values, width, height)} ${width},${height}`
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="观测运维"
      :project-state="step?.status ?? 'ready'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">运维 Agent</span>
          默认每 1 小时巡检一次事件源与指标，异常时生成可预览报表。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">最近一次巡检</span>
          2026-11-09 09:00 · 发现 3 条告警 · 已生成支付可用性日报。
        </div>
        <div class="super-team-task-list super-task-list">
          <div v-for="task in tasks" :key="task.name" class="super-task-item">
            <div>
              <strong>{{ task.name }}</strong>
              <small>{{ task.schedule }} · 上次 {{ task.lastRun }}</small>
            </div>
            <span class="tag">{{ task.status }}</span>
          </div>
        </div>
      </aside>

      <main class="superteam-workbench">
        <div class="superteam-tabs">
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'metrics' }"
            @click="tab = 'metrics'"
          >
            指标看板
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'reports' }"
            @click="tab = 'reports'"
          >
            可预览报表
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'tasks' }"
            @click="tab = 'tasks'"
          >
            定时任务
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'events' }"
            @click="tab = 'events'"
          >
            事件源
          </button>
        </div>

        <div class="superteam-tab-body">
          <div v-if="tab === 'metrics'" class="super-chart-grid">
            <article v-for="metric in metrics" :key="metric.title" class="super-chart-card">
              <h4>{{ metric.title }}</h4>
              <span>{{ metric.caption }}</span>
              <strong>{{ metric.value }}</strong>
              <svg viewBox="0 0 300 80" preserveAspectRatio="none" aria-hidden="true">
                <polygon class="super-chart-area" :points="areaPoints(metric.values)" />
                <polyline class="super-chart-line" :points="linePoints(metric.values)" />
              </svg>
            </article>
          </div>

          <div v-else-if="tab === 'reports'" class="super-task-list">
            <div v-for="report in reports" :key="report.name" class="super-task-item">
              <div>
                <strong>{{ report.name }}</strong>
                <small>更新于 {{ report.updatedAt }}</small>
              </div>
              <span class="tag">{{ report.status }}</span>
            </div>
          </div>

          <div v-else-if="tab === 'tasks'" class="super-task-list">
            <div v-for="task in tasks" :key="task.name" class="super-task-item">
              <div>
                <strong>{{ task.name }}</strong>
                <small>{{ task.schedule }} · 下次 {{ task.nextRun }}</small>
              </div>
              <span class="tag">{{ task.status }}</span>
            </div>
          </div>

          <div v-else class="super-event-list">
            <article v-for="event in events" :key="event.id" class="super-event-row">
              <span class="tag">{{ event.source }}</span>
              <div>
                <strong>{{ event.title }}</strong>
                <small>{{ event.id }}</small>
              </div>
              <div class="super-event-scores">
                <span>噪音 {{ event.noiseScore }}</span>
                <span>语义 {{ event.semanticScore }}</span>
              </div>
            </article>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后整个超级团队流程结束。</p>
          <button
            class="superteam-primary"
            type="button"
            :disabled="step?.status === 'locked'"
            @click="complete"
          >
            完成全流程
          </button>
        </div>
      </main>
    </div>
  </div>
</template>
```

- [ ] **Step 2: Add route**

In `packages/web/src/router/index.ts`:

Add import:

```ts
import SuperTeamOperationsView from '@/views/SuperTeamOperationsView.vue'
```

Add route after development route:

```ts
    {
      path: '/super-team/operations',
      name: 'super-team-operations',
      component: SuperTeamOperationsView,
    },
```

- [ ] **Step 3: Typecheck and lint**

Run:

```bash
cd packages/web && npx vue-tsc --noEmit
cd packages/web && npm run lint
```

Expected: PASS.

- [ ] **Step 4: Commit (only if user approved commits)**

```bash
git add packages/web/src/views/SuperTeamOperationsView.vue packages/web/src/router/index.ts
git commit -m "feat(web): add super team operations demo view"
```

---

### Task 8: web 完整测试与构建

**Files:**
- No new files.

- [ ] **Step 1: Run web tests**

Run: `cd packages/web && npm test`
Expected: PASS (existing + new tests).

- [ ] **Step 2: Run web typecheck build**

Run: `cd packages/web && npm run build`
Expected: PASS.

- [ ] **Step 3: Run web lint and format check**

Run:

```bash
cd packages/web && npm run lint
cd packages/web && npm run format:check
```

Expected: PASS. If format fails, run `npm run format` and re-check.

- [ ] **Step 4: Commit (only if user approved commits)**

```bash
git add packages/web
git commit -m "test(web): verify super team frontend"
```

---

### Task 9: ops-web 类型、工具与 mock 数据

**Files:**
- Create: `packages/ops-web/src/types/superTeam.ts`
- Create: `packages/ops-web/src/utils/superTeam.ts`
- Create: `packages/ops-web/src/utils/superTeam.test.ts`
- Create: `packages/ops-web/src/mocks/superTeam.ts`

**Interfaces:**
- Produces: `AdminSuperTeamSource`、`AdminSuperTeamPipeline`、`AdminSuperTeamAction`。
- Produces: `sourceKindLabel`、`superStepLabel`、`agentActionLabel`、`actionStatusLabel`、`scorePercent`、`scoreBand`、`formatDuration`。
- Produces: `adminSuperTeamSources`、`adminSuperTeamPipelines`、`adminSuperTeamActions`。

- [ ] **Step 1: Write failing test**

Create `packages/ops-web/src/utils/superTeam.test.ts`:

```ts
import { describe, expect, it } from 'vitest'

import {
  actionStatusLabel,
  agentActionLabel,
  formatDuration,
  scoreBand,
  scorePercent,
  sourceKindLabel,
  superStepLabel,
} from './superTeam'

describe('ops super team utils', () => {
  it('maps source kinds', () => {
    expect(sourceKindLabel('feishu')).toBe('飞书')
    expect(sourceKindLabel('webhook')).toBe('Webhook')
    expect(sourceKindLabel('queue')).toBe('消息队列')
    expect(sourceKindLabel('unknown')).toBe('unknown')
  })

  it('maps steps and actions', () => {
    expect(superStepLabel('requirements')).toBe('需求挖掘')
    expect(superStepLabel('operations')).toBe('观测运维')
    expect(agentActionLabel('noise_filter')).toBe('噪音过滤')
    expect(agentActionLabel('semantic_eval')).toBe('语义评估')
    expect(agentActionLabel('artifact')).toBe('产物生成')
    expect(actionStatusLabel('running')).toBe('执行中')
  })

  it('formats scores and durations', () => {
    expect(scorePercent(82.6)).toBe('83%')
    expect(scorePercent(-5)).toBe('0%')
    expect(scorePercent(120)).toBe('100%')
    expect(scoreBand(20)).toBe('low')
    expect(scoreBand(50)).toBe('medium')
    expect(scoreBand(90)).toBe('high')
    expect(formatDuration(320)).toBe('320ms')
    expect(formatDuration(1840)).toBe('1.8s')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd packages/ops-web && npx vitest run src/utils/superTeam.test.ts`
Expected: FAIL — module not found.

- [ ] **Step 3: Write types**

Create `packages/ops-web/src/types/superTeam.ts`:

```ts
export interface AdminSuperTeamSource {
  id: number
  owner: string
  kind: string
  status: string
  enabled: boolean
  lastEventAt: string | null
  eventCount: number
  updatedAt: string
}

export interface AdminSuperTeamPipeline {
  id: number
  owner: string
  workspace: string
  requirements: string
  development: string
  operations: string
  currentStep: string
  updatedAt: string
}

export interface AdminSuperTeamAction {
  id: string
  eventId: string
  source: string
  step: string
  agent: string
  action: string
  noiseScore: number
  semanticScore: number
  status: string
  durationMs: number
  createdAt: string
}
```

- [ ] **Step 4: Write utils**

Create `packages/ops-web/src/utils/superTeam.ts`:

```ts
const SOURCE_LABELS: Record<string, string> = {
  feishu: '飞书',
  webhook: 'Webhook',
  email: 'Email',
  queue: '消息队列',
}

const STEP_LABELS: Record<string, string> = {
  requirements: '需求挖掘',
  development: '系统开发',
  operations: '观测运维',
}

const ACTION_LABELS: Record<string, string> = {
  noise_filter: '噪音过滤',
  semantic_eval: '语义评估',
  artifact: '产物生成',
  step_advance: '阶段推进',
}

const STATUS_LABELS: Record<string, string> = {
  success: '成功',
  failed: '失败',
  running: '执行中',
}

export type ScoreBand = 'low' | 'medium' | 'high'

export function sourceKindLabel(kind: string): string {
  return SOURCE_LABELS[kind] ?? kind
}

export function superStepLabel(key: string): string {
  return STEP_LABELS[key] ?? key
}

export function agentActionLabel(action: string): string {
  return ACTION_LABELS[action] ?? action
}

export function actionStatusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

export function scorePercent(score: number): string {
  const normalized = Math.max(0, Math.min(100, Math.round(score)))
  return `${normalized}%`
}

export function scoreBand(score: number): ScoreBand {
  if (score >= 70) {
    return 'high'
  }
  return score >= 40 ? 'medium' : 'low'
}

export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`
  }
  return `${(ms / 1000).toFixed(1)}s`
}
```

- [ ] **Step 5: Write mocks**

Create `packages/ops-web/src/mocks/superTeam.ts`:

```ts
import type {
  AdminSuperTeamAction,
  AdminSuperTeamPipeline,
  AdminSuperTeamSource,
} from '@/types/superTeam'

export const adminSuperTeamSources: AdminSuperTeamSource[] = [
  {
    id: 1,
    owner: 'wanghg11',
    kind: 'feishu',
    status: 'connected',
    enabled: true,
    lastEventAt: '2026-11-09T09:42:00.000Z',
    eventCount: 128,
    updatedAt: '2026-11-09T09:42:00.000Z',
  },
  {
    id: 2,
    owner: 'wanghg11',
    kind: 'webhook',
    status: 'connected',
    enabled: true,
    lastEventAt: '2026-11-09T09:30:00.000Z',
    eventCount: 64,
    updatedAt: '2026-11-09T09:30:00.000Z',
  },
  {
    id: 3,
    owner: 'vistor',
    kind: 'email',
    status: 'unconfigured',
    enabled: false,
    lastEventAt: null,
    eventCount: 0,
    updatedAt: '2026-11-08T18:00:00.000Z',
  },
  {
    id: 4,
    owner: 'vistor',
    kind: 'queue',
    status: 'coming_soon',
    enabled: false,
    lastEventAt: null,
    eventCount: 0,
    updatedAt: '2026-11-08T18:00:00.000Z',
  },
]

export const adminSuperTeamPipelines: AdminSuperTeamPipeline[] = [
  {
    id: 1,
    owner: 'wanghg11',
    workspace: '个人工作区',
    requirements: '进行中',
    development: '锁定',
    operations: '锁定',
    currentStep: 'requirements',
    updatedAt: '2026-11-09T09:42:00.000Z',
  },
  {
    id: 2,
    owner: 'vistor',
    workspace: '访客工作区',
    requirements: '已完成',
    development: '进行中',
    operations: '锁定',
    currentStep: 'development',
    updatedAt: '2026-11-09T08:20:00.000Z',
  },
  {
    id: 3,
    owner: 'wanghg11',
    workspace: '团队工作区',
    requirements: '已完成',
    development: '已完成',
    operations: '已完成',
    currentStep: 'operations',
    updatedAt: '2026-11-08T20:10:00.000Z',
  },
]

export const adminSuperTeamActions: AdminSuperTeamAction[] = [
  {
    id: 'act-1',
    eventId: 'evt-1001',
    source: 'feishu',
    step: 'requirements',
    agent: '需求分析师',
    action: 'noise_filter',
    noiseScore: 8,
    semanticScore: 92,
    status: 'success',
    durationMs: 320,
    createdAt: '2026-11-09T09:42:02.000Z',
  },
  {
    id: 'act-2',
    eventId: 'evt-1001',
    source: 'feishu',
    step: 'requirements',
    agent: '需求分析师',
    action: 'semantic_eval',
    noiseScore: 8,
    semanticScore: 92,
    status: 'success',
    durationMs: 1840,
    createdAt: '2026-11-09T09:42:04.000Z',
  },
  {
    id: 'act-3',
    eventId: 'evt-1001',
    source: 'feishu',
    step: 'requirements',
    agent: '产品经理',
    action: 'artifact',
    noiseScore: 8,
    semanticScore: 92,
    status: 'success',
    durationMs: 5220,
    createdAt: '2026-11-09T09:42:10.000Z',
  },
  {
    id: 'act-4',
    eventId: 'evt-1004',
    source: 'webhook',
    step: 'requirements',
    agent: '需求分析师',
    action: 'noise_filter',
    noiseScore: 91,
    semanticScore: 12,
    status: 'success',
    durationMs: 210,
    createdAt: '2026-11-09T08:55:01.000Z',
  },
  {
    id: 'act-5',
    eventId: 'evt-1006',
    source: 'email',
    step: 'requirements',
    agent: '需求分析师',
    action: 'semantic_eval',
    noiseScore: 11,
    semanticScore: 88,
    status: 'running',
    durationMs: 0,
    createdAt: '2026-11-09T08:20:03.000Z',
  },
  {
    id: 'act-6',
    eventId: 'evt-1002',
    source: 'webhook',
    step: 'development',
    agent: '架构师',
    action: 'step_advance',
    noiseScore: 16,
    semanticScore: 84,
    status: 'failed',
    durationMs: 640,
    createdAt: '2026-11-09T08:10:00.000Z',
  },
]
```

- [ ] **Step 6: Run test to verify it passes**

Run: `cd packages/ops-web && npx vitest run src/utils/superTeam.test.ts`
Expected: PASS.

- [ ] **Step 7: Commit (only if user approved commits)**

```bash
git add packages/ops-web/src/types/superTeam.ts packages/ops-web/src/utils/superTeam.ts packages/ops-web/src/utils/superTeam.test.ts packages/ops-web/src/mocks/superTeam.ts
git commit -m "feat(ops-web): add super team types, utils and mocks"
```

---

### Task 10: ops-web 超级团队页面与导航

**Files:**
- Create: `packages/ops-web/src/views/SuperTeamView.vue`
- Modify: `packages/ops-web/src/router/navigation.ts`
- Modify: `packages/ops-web/src/router/routes.ts`
- Modify: `packages/ops-web/src/router/navigation.test.ts`

**Interfaces:**
- Consumes: Task 9 的 mock 与工具、`DataTable`、`TableSearch`、`TablePagination`。
- Produces: 路由 `super-team`，导航项 `超级团队`。

- [ ] **Step 1: Create view**

Create `packages/ops-web/src/views/SuperTeamView.vue`:

```vue
<script setup lang="ts">
import { computed, ref } from 'vue'

import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import {
  adminSuperTeamActions,
  adminSuperTeamPipelines,
  adminSuperTeamSources,
} from '@/mocks/superTeam'
import type { TableColumn, TableRow } from '@/types'
import {
  actionStatusLabel,
  agentActionLabel,
  formatDuration,
  scorePercent,
  sourceKindLabel,
  superStepLabel,
} from '@/utils/superTeam'

type TabKey = 'sources' | 'pipelines' | 'actions'

const tab = ref<TabKey>('sources')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(10)

const tabs: { key: TabKey; label: string; count: number }[] = [
  { key: 'sources', label: '事件源', count: adminSuperTeamSources.length },
  { key: 'pipelines', label: '三步项目', count: adminSuperTeamPipelines.length },
  { key: 'actions', label: 'Agent 行为审计', count: adminSuperTeamActions.length },
]

const sourceColumns: TableColumn[] = [
  { key: 'owner', label: '归属用户', width: '120px' },
  { key: 'kind', label: '类型', width: '110px', format: (value) => sourceKindLabel(String(value)) },
  { key: 'status', label: '状态', width: '110px' },
  { key: 'enabled', label: '启用', width: '80px', format: (value) => (value ? '是' : '否') },
  { key: 'lastEventAt', label: '最近事件', width: '170px', format: (value) => formatDate(value) },
  { key: 'eventCount', label: '事件数', width: '90px' },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDate(value) },
]

const pipelineColumns: TableColumn[] = [
  { key: 'owner', label: '用户', width: '120px' },
  { key: 'workspace', label: '工作区', width: '140px' },
  { key: 'requirements', label: '需求挖掘', width: '100px' },
  { key: 'development', label: '系统开发', width: '100px' },
  { key: 'operations', label: '观测运维', width: '100px' },
  {
    key: 'currentStep',
    label: '当前步骤',
    width: '120px',
    format: (value) => superStepLabel(String(value)),
  },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDate(value) },
]

const actionColumns: TableColumn[] = [
  { key: 'eventId', label: '事件 ID', width: '120px' },
  {
    key: 'source',
    label: '来源',
    width: '100px',
    format: (value) => sourceKindLabel(String(value)),
  },
  { key: 'step', label: '步骤', width: '100px', format: (value) => superStepLabel(String(value)) },
  { key: 'agent', label: 'Agent', width: '120px' },
  {
    key: 'action',
    label: '动作',
    width: '110px',
    format: (value) => agentActionLabel(String(value)),
  },
  { key: 'noiseScore', label: '噪音分', width: '90px', format: (value) => scorePercent(Number(value)) },
  {
    key: 'semanticScore',
    label: '语义分',
    width: '90px',
    format: (value) => scorePercent(Number(value)),
  },
  {
    key: 'status',
    label: '状态',
    width: '90px',
    format: (value) => actionStatusLabel(String(value)),
  },
  {
    key: 'durationMs',
    label: '耗时',
    width: '90px',
    format: (value) => formatDuration(Number(value)),
  },
  { key: 'createdAt', label: '时间', width: '170px', format: (value) => formatDate(value) },
]

function formatDate(value: unknown): string {
  if (typeof value !== 'string' || !value) {
    return '—'
  }
  return new Date(value).toLocaleString('zh-CN')
}

const rows = computed<TableRow[]>(() => {
  if (tab.value === 'sources') {
    return adminSuperTeamSources
  }
  if (tab.value === 'pipelines') {
    return adminSuperTeamPipelines
  }
  return adminSuperTeamActions
})

const columns = computed<TableColumn[]>(() => {
  if (tab.value === 'sources') {
    return sourceColumns
  }
  if (tab.value === 'pipelines') {
    return pipelineColumns
  }
  return actionColumns
})

const filteredRows = computed<TableRow[]>(() => {
  const query = keyword.value.trim().toLowerCase()
  if (!query) {
    return rows.value
  }
  return rows.value.filter((row) => JSON.stringify(row).toLowerCase().includes(query))
})

const total = computed(() => filteredRows.value.length)
const pagedRows = computed<TableRow[]>(() =>
  filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)

function switchTab(next: TabKey): void {
  tab.value = next
  keyword.value = ''
  page.value = 1
}

function search(): void {
  page.value = 1
}

function setPage(next: number): void {
  page.value = next
}

function setPageSize(next: number): void {
  pageSize.value = next
  page.value = 1
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>超级团队</h1>
        <p class="page-desc">只读视图；事件源由用户在 web 端配置。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <div class="super-tabs">
      <button
        v-for="item in tabs"
        :key="item.key"
        type="button"
        class="super-tab"
        :class="{ active: tab === item.key }"
        @click="switchTab(item.key)"
      >
        {{ item.label }} · {{ item.count }}
      </button>
    </div>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="pagedRows"
      empty-text="暂无数据"
    />

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

<style scoped>
.super-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.super-tab {
  padding: 8px 14px;
  color: var(--muted);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 999px;
  cursor: pointer;
}

.super-tab.active {
  color: var(--blue);
  background: var(--blue-soft);
  border-color: rgba(37, 99, 235, 0.35);
}
</style>
```

- [ ] **Step 2: Add navigation item**

In `packages/ops-web/src/router/navigation.ts`, after the `evolution` entry add:

```ts
  { name: 'super-team', label: '超级团队', path: '/super-team', icon: 'pi pi-users' },
```

- [ ] **Step 3: Add route**

In `packages/ops-web/src/router/routes.ts`:

Add import:

```ts
import SuperTeamView from '@/views/SuperTeamView.vue'
```

Add child route after `evolution`:

```ts
  {
    path: 'super-team',
    name: 'super-team',
    component: SuperTeamView,
    meta: { title: '超级团队' },
  },
```

- [ ] **Step 4: Update navigation test**

In `packages/ops-web/src/router/navigation.test.ts`, insert `'/super-team'` after `'/evolution'` in the paths array, and `'超级团队'` after `'集思进化'` in the labels array.

The expected arrays become:

```ts
    expect(navItems.map((item) => item.path)).toEqual([
      '/',
      '/users',
      '/projects',
      '/cards',
      '/leaderboard',
      '/evolution',
      '/super-team',
      '/sandboxes',
      '/models',
      '/skills',
      '/roles',
      '/fde-resources',
      '/topics',
      '/templates',
      '/channel-guides',
      '/scheduled-tasks',
      '/quotas',
      '/audit',
      '/access-logs',
      '/notifications',
      '/settings',
    ])
    expect(navItems.map((item) => item.label)).toEqual([
      '概览',
      '用户',
      '项目',
      '名片管理',
      '排行榜',
      '集思进化',
      '超级团队',
      '沙箱节点',
      '模型管理',
      '技能',
      '角色管理',
      'FDE 资源',
      '类别管理',
      '模板管理',
      '发布渠道',
      '定时任务',
      '配额',
      '审计日志',
      '访问日志',
      '消息通知',
      '系统设置',
    ])
```

- [ ] **Step 5: Typecheck, test, lint**

Run:

```bash
cd packages/ops-web && npx vue-tsc --noEmit
cd packages/ops-web && npm test
cd packages/ops-web && npm run lint
cd packages/ops-web && npm run format:check
```

Expected: PASS. If format fails, run `npm run format` and re-check.

- [ ] **Step 6: Commit (only if user approved commits)**

```bash
git add packages/ops-web/src/views/SuperTeamView.vue packages/ops-web/src/router
git commit -m "feat(ops-web): add super team admin view"
```

---

### Task 11: 全量验证与检查点

**Files:**
- No new files.

- [ ] **Step 1: Run full test suite**

Run: `make test`
Expected: PASS（Go、pytest、web、ops-web 全绿）。

- [ ] **Step 2: Run quality gate**

Run: `make quality`
Expected: PASS（gofmt / vet / ruff / 两端 eslint / 调色板）。

- [ ] **Step 3: Run validate**

Run: `make validate`
Expected: PASS（errors=0）。

- [ ] **Step 4: Run builds**

Run:

```bash
make -C packages/web build
make -C packages/ops-web build
```

Expected: PASS.

- [ ] **Step 5: Save harness checkpoint**

Run: `make checkpoint`
Expected: 生成 `.ai/checkpoints/` 记录，更新 `.ai/CURRENT_FOCUS`。

- [ ] **Step 6: Commit (only if user approved commits)**

```bash
git add -A
git commit -m "feat: add super team frontend"
```

---

## Self-Review

**Spec coverage**

| Spec 要求 | 对应任务 |
|---|---|
| 侧栏新增超级团队 | Task 4 |
| 首页居中、四个事件卡片 | Task 3 + Task 4 |
| 卡片点击配置（用户级） | Task 3 + Task 4 |
| 消息队列占位禁用 | Task 1 + Task 2 + Task 3 |
| 三步严格门禁 | Task 1 + Task 2 + Task 4 |
| 需求挖掘编辑页 + 思维导图/知识图谱/可行性/事件流 | Task 5 |
| 系统开发企业七阶段 + 事件注入 | Task 6 |
| 观测运维定时任务 + 可预览指标 | Task 7 |
| ops-web 单导航三 tab 只读 | Task 9 + Task 10 |
| 事件源 ops-web 只读 | Task 10 |
| 仅黑白灰蓝、无新依赖 | Global Constraints + Task 8 + Task 11 |
| 后端未来设计 | 仅 spec 第 13 节，不在本计划实现 |

**Placeholder scan:** 无 TBD / TODO / “类似 Task N” / 空泛错误处理描述。

**Type consistency:** web 使用 `SuperStepKey`、`EventSourceKind`、`SuperTeamState`；ops-web 使用独立的 `AdminSuperTeam*` 类型，避免跨包耦合。工具函数名称与任务描述一致。

**已知取舍:** ops-web mock 只有 6 行以内，分页保留但不产生多页；后续接真实 API 时替换 mock 数据源即可。

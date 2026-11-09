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

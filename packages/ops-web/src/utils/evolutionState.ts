import type {
  EvolutionAdminState,
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionTask,
  EvolutionVersion,
} from '@/types/evolution'

import {
  EVOLUTION_STORAGE_KEY,
  OFFICIAL_USER_ID,
  OFFICIAL_USER_NAME,
  OFFICIAL_VERSION_ID,
  OFFICIAL_VERSION_LABEL,
  createTask,
  nextId,
  nextTaskStatus,
} from './evolution'

const MINUTE = 60_000
const HOUR = 60 * MINUTE
const DAY = 24 * HOUR

let idSeq = 0

function uniqueId(prefix: string, now: number): string {
  idSeq += 1
  return nextId(prefix, now, idSeq)
}

function iso(time: number): string {
  return new Date(time).toISOString()
}

function cloneState<T>(state: T): T {
  return JSON.parse(JSON.stringify(state)) as T
}

function message(
  id: string,
  createdAt: string,
  role: EvolutionChatMessage['role'],
  userName: string,
  content: string,
): EvolutionChatMessage {
  return { id, createdAt, role, userName, content }
}

export function createSeedState(now = Date.now()): EvolutionAdminState {
  const versions: EvolutionVersion[] = [
    {
      id: OFFICIAL_VERSION_ID,
      userId: OFFICIAL_USER_ID,
      userName: OFFICIAL_USER_NAME,
      label: OFFICIAL_VERSION_LABEL,
      createdAt: iso(now - 8 * DAY),
      updatedAt: iso(now - 8 * DAY),
      status: 'released',
      scenario: 'baseline',
      summary: '当前线上版本的交互副本，由 CI 构建后写入 origin_version。',
      proposalIds: [],
    },
    {
      id: 'v-1001-2',
      userId: '1001',
      userName: '林岚',
      label: 'v1001.2',
      createdAt: iso(now - 2 * HOUR),
      updatedAt: iso(now - 2 * HOUR),
      status: 'preview',
      scenario: 'compact-sidebar',
      summary: '收紧侧栏与项目卡片密度，修复删除按钮无响应。',
      proposalIds: ['p-1001-1', 'p-1001-2'],
    },
    {
      id: 'v-1001-1',
      userId: '1001',
      userName: '林岚',
      label: 'v1001.1',
      createdAt: iso(now - 1 * DAY),
      updatedAt: iso(now - 1 * DAY),
      status: 'accepted',
      scenario: 'soft-cards',
      summary: '项目卡片改为低对比度圆角样式。',
      proposalIds: [],
    },
    {
      id: 'v-1002-1',
      userId: '1002',
      userName: '陈墨',
      label: 'v1002.1',
      createdAt: iso(now - 5 * HOUR),
      updatedAt: iso(now - 5 * HOUR),
      status: 'preview',
      scenario: 'focus-composer',
      summary: '首页首屏聚焦创作输入，弱化发现区视觉噪音。',
      proposalIds: ['p-1002-1'],
    },
    {
      id: 'v-1003-1',
      userId: '1003',
      userName: '周野',
      label: 'v1003.1',
      createdAt: iso(now - 26 * HOUR),
      updatedAt: iso(now - 26 * HOUR),
      status: 'submitted',
      scenario: 'quick-actions',
      summary: '创作输入下增加快捷操作入口。',
      proposalIds: ['p-1003-1'],
    },
    {
      id: 'v-1004-1',
      userId: '1004',
      userName: '苏晴',
      label: 'v1004.1',
      createdAt: iso(now - 3 * HOUR),
      updatedAt: iso(now - 3 * HOUR),
      status: 'preview',
      scenario: 'soft-cards',
      summary: '调整项目卡片层级与模板区留白。',
      proposalIds: [],
    },
  ]

  const proposals: EvolutionProposal[] = [
    {
      id: 'p-1001-1',
      createdAt: iso(now - 2 * HOUR - 3 * MINUTE),
      userId: '1001',
      userName: '林岚',
      kind: 'bug',
      content: '首页项目卡片的删除按钮点击没有反应。',
      status: 'accepted',
      agentReply: '已定位到该问题属于当前页面的确定性缺陷，已在你的版本副本中直接修复。',
      versionId: 'v-1001-2',
    },
    {
      id: 'p-1001-2',
      createdAt: iso(now - 2 * HOUR),
      userId: '1001',
      userName: '林岚',
      kind: 'visual',
      content: '首页卡片太挤了，希望导航和卡片都更紧凑一点。',
      status: 'accepted',
      agentReply: '属于视觉 / 交互优化，已生成紧凑布局候选版本。',
      versionId: 'v-1001-2',
    },
    {
      id: 'p-1002-1',
      createdAt: iso(now - 5 * HOUR),
      userId: '1002',
      userName: '陈墨',
      kind: 'visual',
      content: '首页首屏输入框不够聚焦，希望打开就能直接创作。',
      status: 'queued',
      agentReply: '属于视觉 / 交互优化，已生成聚焦创作候选版本，请预览确认。',
      versionId: 'v-1002-1',
    },
    {
      id: 'p-1003-1',
      createdAt: iso(now - 26 * HOUR),
      userId: '1003',
      userName: '周野',
      kind: 'visual',
      content: '希望在创作输入下面增加几个快捷操作入口。',
      status: 'accepted',
      agentReply: '快捷入口不改变架构，已生成候选版本。',
      versionId: 'v-1003-1',
    },
    {
      id: 'p-1004-1',
      createdAt: iso(now - 3 * HOUR),
      userId: '1004',
      userName: '苏晴',
      kind: 'feature',
      content: '希望支持多人同时编辑同一个项目。',
      status: 'reviewing',
      agentReply:
        '这个提议涉及功能或架构改动，我不会直接生成。需要先确认权限模型、冲突解决和数据契约。',
      versionId: null,
    },
  ]

  const chats: Record<string, EvolutionChatMessage[]> = {
    '1001': [
      message('c-1001-1', iso(now - 2 * HOUR - 6 * MINUTE), 'user', '林岚', '首页卡片太挤了。'),
      message(
        'c-1001-2',
        iso(now - 2 * HOUR - 5 * MINUTE),
        'agent',
        '集思进化助手',
        '属于视觉 / 交互优化，已生成紧凑布局候选版本，请预览。',
      ),
      message(
        'c-1001-3',
        iso(now - 2 * HOUR - 2 * MINUTE),
        'user',
        '林岚',
        '删除按钮点了没反应，这个也要修。',
      ),
      message(
        'c-1001-4',
        iso(now - 2 * HOUR - MINUTE),
        'agent',
        '集思进化助手',
        '已定位为确定性缺陷，在你的版本副本中直接修复，官方版本不受影响。',
      ),
    ],
    '1002': [
      message(
        'c-1002-1',
        iso(now - 5 * HOUR),
        'user',
        '陈墨',
        '打开首页后视线太分散，想直接开始写需求。',
      ),
      message(
        'c-1002-2',
        iso(now - 5 * HOUR + MINUTE),
        'agent',
        '集思进化助手',
        '我会把创作输入提到视觉中心，同时保留发现区入口，先给你看候选效果。',
      ),
    ],
    '1003': [
      message('c-1003-1', iso(now - 27 * HOUR), 'user', '周野', '能加几个快捷操作吗？'),
      message(
        'c-1003-2',
        iso(now - 26 * HOUR),
        'agent',
        '集思进化助手',
        '快捷入口属于页面交互范围，已生成候选版本。',
      ),
    ],
    '1004': [
      message('c-1004-1', iso(now - 3 * HOUR), 'user', '苏晴', '希望支持多人同时编辑同一个项目。'),
      message(
        'c-1004-2',
        iso(now - 3 * HOUR + MINUTE),
        'agent',
        '集思进化助手',
        '这个提议涉及架构改动，需要先确认权限与冲突处理，暂不直接生成。',
      ),
    ],
  }

  const tasks: EvolutionTask[] = [
    {
      id: 't-seed-1',
      versionId: 'v-1003-1',
      userId: '1003',
      userName: '周野',
      versionLabel: 'v1003.1',
      originVersion: OFFICIAL_VERSION_LABEL,
      chatSummary: '用户希望增加快捷操作入口，已验证候选版本。',
      status: 'ci',
      createdAt: iso(now - 20 * HOUR),
    },
  ]

  return {
    versions,
    proposals,
    chats,
    tasks,
    selectedVersionId: 'v-1001-2',
    selectedUserId: '1001',
  }
}

export function normalizeState(input: Partial<EvolutionAdminState>): EvolutionAdminState {
  return {
    proposals: Array.isArray(input.proposals) ? input.proposals : [],
    versions: Array.isArray(input.versions) ? input.versions : [],
    chats:
      input.chats && typeof input.chats === 'object'
        ? (input.chats as Record<string, EvolutionChatMessage[]>)
        : {},
    tasks: Array.isArray(input.tasks) ? input.tasks : [],
    selectedVersionId: typeof input.selectedVersionId === 'string' ? input.selectedVersionId : null,
    selectedUserId: typeof input.selectedUserId === 'string' ? input.selectedUserId : null,
  }
}

export interface EvolutionStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

export function browserStorage(): EvolutionStorage | null {
  if (typeof window === 'undefined') {
    return null
  }
  const storage = window.localStorage
  if (!storage || typeof storage.getItem !== 'function' || typeof storage.setItem !== 'function') {
    return null
  }
  return storage
}

export function loadEvolutionState(
  now = Date.now(),
  storage: EvolutionStorage | null = browserStorage(),
): EvolutionAdminState {
  if (!storage) {
    return createSeedState(now)
  }
  try {
    const raw = storage.getItem(EVOLUTION_STORAGE_KEY)
    if (!raw) {
      const seed = createSeedState(now)
      storage.setItem(EVOLUTION_STORAGE_KEY, JSON.stringify(seed))
      return seed
    }
    const state = normalizeState(JSON.parse(raw) as Partial<EvolutionAdminState>)
    if (!state.selectedVersionId) {
      state.selectedVersionId = state.versions[0]?.id ?? null
      state.selectedUserId = state.versions[0]?.userId ?? null
    }
    return state
  } catch {
    return createSeedState(now)
  }
}

export function saveEvolutionState(
  state: EvolutionAdminState,
  storage: EvolutionStorage | null = browserStorage(),
): void {
  if (!storage) {
    return
  }
  storage.setItem(EVOLUTION_STORAGE_KEY, JSON.stringify(state))
}

export function sendAdminReplyState(
  state: EvolutionAdminState,
  content: string,
  now = Date.now(),
): EvolutionAdminState {
  const text = content.trim()
  if (!text) {
    return state
  }
  const next = cloneState(state)
  const version = next.versions.find((item) => item.id === next.selectedVersionId)
  if (!version) {
    return state
  }
  next.chats[version.userId] = [
    ...(next.chats[version.userId] ?? []),
    message(uniqueId('c', now), iso(now), 'admin', '运营管理员', text),
  ]
  return next
}

export function submitTaskState(
  state: EvolutionAdminState,
  originVersion: string,
  now = Date.now(),
): EvolutionAdminState {
  const version = state.versions.find((item) => item.id === state.selectedVersionId)
  if (!version) {
    return state
  }
  if (state.tasks.some((task) => task.versionId === version.id)) {
    return state
  }
  const next = cloneState(state)
  const target = next.versions.find((item) => item.id === state.selectedVersionId)
  if (!target) {
    return state
  }
  const chatSummary = summarizeChat(next.chats[target.userId] ?? [], target)
  next.tasks.unshift(createTask(target, originVersion, chatSummary, now))
  target.status = 'submitted'
  return next
}

export function advanceTaskState(state: EvolutionAdminState, taskId: string): EvolutionAdminState {
  const next = cloneState(state)
  const task = next.tasks.find((item) => item.id === taskId)
  if (!task) {
    return state
  }
  task.status = nextTaskStatus(task.status)
  if (task.status === 'released') {
    const version = next.versions.find((item) => item.id === task.versionId)
    if (version) {
      version.status = 'released'
    }
  }
  return next
}

function summarizeChat(messages: EvolutionChatMessage[], version: EvolutionVersion): string {
  const recent = messages.slice(-4).map((item) => item.content)
  if (recent.length) {
    return `${version.summary} 对话摘要：${recent.join(' / ')}`
  }
  return version.summary
}

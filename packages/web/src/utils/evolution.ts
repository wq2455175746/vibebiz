import type {
  EvolutionChatMessage,
  EvolutionVersion,
  EvolutionVersionGroup,
  ProposalKind,
} from '@/types/evolution'

export const EVOLUTION_PROJECT_ID = 'vibebiz-evolve'
export const EVOLUTION_STORAGE_KEY = 'vibebiz:evolution:v1'
export const EVOLUTION_PREVIEW_PATH = '/evolution/origin_version/index.html'
export const OFFICIAL_USER_ID = 'official'
export const OFFICIAL_USER_NAME = 'Vibebiz 官方'
export const OFFICIAL_VERSION_ID = 'v-official-1'
export const OFFICIAL_VERSION_LABEL = 'v1.2026.11'

export type AgentAction = 'patch' | 'preview' | 'pushback'

export interface AgentDecision {
  kind: ProposalKind
  action: AgentAction
  reply: string
  scenario: string
  summary: string
}

const BUG_WORDS = [
  'bug',
  '报错',
  '错误',
  '崩溃',
  '闪退',
  '白屏',
  '空白',
  '失效',
  '没反应',
  '没有反应',
  '无法',
  '不能',
  '异常',
  '卡住',
  '错别字',
  '打不开',
  '加载失败',
]

const FEATURE_WORDS = [
  '新增',
  '增加',
  '支持',
  '接入',
  '功能',
  '模块',
  '重构',
  '架构',
  '数据库',
  '接口',
  '权限',
  '登录',
  '支付',
  '协作',
  '多用户',
]

const VISUAL_WORDS = [
  '颜色',
  '布局',
  '间距',
  '字体',
  '按钮',
  '样式',
  '优化',
  '交互',
  '体验',
  '文案',
  '动画',
  '紧凑',
  '卡片',
  '导航',
  '排版',
  '预览',
  '首屏',
  '输入框',
]

function containsAny(content: string, words: string[]): boolean {
  const text = content.toLowerCase()
  return words.some((word) => text.includes(word))
}

export function classifyProposal(content: string): ProposalKind {
  if (containsAny(content, BUG_WORDS)) {
    return 'bug'
  }
  if (containsAny(content, FEATURE_WORDS)) {
    return 'feature'
  }
  if (containsAny(content, VISUAL_WORDS)) {
    return 'visual'
  }
  return 'feature'
}

export function pickScenario(content: string, kind: ProposalKind): string {
  if (kind === 'bug') {
    return 'bugfix'
  }
  if (kind === 'feature') {
    return 'focus-composer'
  }
  if (containsAny(content, ['紧凑', '密度', '间距', '导航', '布局'])) {
    return 'compact-sidebar'
  }
  if (containsAny(content, ['卡片', '项目', '模板'])) {
    return 'soft-cards'
  }
  if (containsAny(content, ['首屏', '输入框', '聚焦', '创作'])) {
    return 'focus-composer'
  }
  return 'soft-cards'
}

export function buildAgentDecision(content: string): AgentDecision {
  const kind = classifyProposal(content)
  const scenario = pickScenario(content, kind)

  if (kind === 'bug') {
    return {
      kind,
      action: 'patch',
      scenario,
      summary: '修复当前用户版本中的确定性页面缺陷。',
      reply:
        '已定位到这个问题属于当前页面的确定性缺陷。基于现有交互约定，我会在你的版本副本中直接修复并渲染，官方 origin_version 保持不变。请在预览中确认修复结果。',
    }
  }

  if (kind === 'visual') {
    return {
      kind,
      action: 'preview',
      scenario,
      summary: '调整当前页面的视觉与交互呈现，不改动平台架构。',
      reply:
        '这个建议属于视觉 / 交互优化，方向可行。我的判断是：只调整当前页面的呈现与操作路径，不改动平台架构和数据契约。我已生成候选版本，请预览后确认是否采纳。',
    }
  }

  return {
    kind,
    action: 'pushback',
    scenario,
    summary: '功能或架构提议，等待补充场景与验收标准。',
    reply:
      '这个提议涉及功能或架构改动，我不会直接生成。理由：一是会扩大当前版本范围；二是需要先确认与平台现有边界、数据契约不冲突；三是目前缺少可验证的验收标准。建议先补充使用场景、影响范围和成功标准，再评估是否纳入后续版本。',
  }
}

export function nextId(prefix: string, now = Date.now(), seq = 0): string {
  return `${prefix}-${now.toString(36)}-${seq.toString(36)}`
}

export function nextVersionLabel(userId: string, count: number): string {
  return `v${userId}.${Math.max(1, count)}`
}

export function latestVersionsByUser(
  versions: EvolutionVersion[],
  currentUserId: string,
): EvolutionVersion[] {
  const groups = groupVersionsByUser(versions, currentUserId)
  return groups.map((group) => group.latest)
}

export function groupVersionsByUser(
  versions: EvolutionVersion[],
  currentUserId: string,
): EvolutionVersionGroup[] {
  const groups = new Map<string, EvolutionVersionGroup>()
  for (const version of versions) {
    const existing = groups.get(version.userId)
    if (!existing) {
      groups.set(version.userId, {
        userId: version.userId,
        userName: version.userName,
        latest: version,
        versions: [version],
      })
      continue
    }
    existing.versions.push(version)
    if (Date.parse(version.updatedAt) > Date.parse(existing.latest.updatedAt)) {
      existing.latest = version
    }
  }

  const list = Array.from(groups.values())
  for (const group of list) {
    group.versions.sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))
  }

  return list.sort((a, b) => {
    if (a.userId === currentUserId) return -1
    if (b.userId === currentUserId) return 1
    if (a.userId === OFFICIAL_USER_ID) return 1
    if (b.userId === OFFICIAL_USER_ID) return -1
    return Date.parse(b.latest.updatedAt) - Date.parse(a.latest.updatedAt)
  })
}

export function withQuery(base: string, params: Record<string, string | undefined>): string {
  const hashIndex = base.indexOf('#')
  const hash = hashIndex >= 0 ? base.slice(hashIndex) : ''
  const withoutHash = hashIndex >= 0 ? base.slice(0, hashIndex) : base
  const queryIndex = withoutHash.indexOf('?')
  const path = queryIndex >= 0 ? withoutHash.slice(0, queryIndex) : withoutHash
  const query = new URLSearchParams(queryIndex >= 0 ? withoutHash.slice(queryIndex + 1) : '')
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === '') {
      query.delete(key)
      continue
    }
    query.set(key, value)
  }
  const search = query.toString()
  return `${path}${search ? `?${search}` : ''}${hash}`
}

export function buildEvolutionPreviewUrl(
  base: string,
  version: EvolutionVersion,
  viewerName = '',
): string {
  return withQuery(base, {
    scenario: version.scenario,
    version: version.label,
    user: version.userName,
    viewer: viewerName,
  })
}

export function statusLabel(status: string): string {
  const labels: Record<string, string> = {
    queued: '待处理',
    reviewing: '评估中',
    accepted: '已采纳',
    rejected: '未采纳',
    released: '已发布',
    draft: '草案',
    preview: '待确认',
    submitted: '已提交',
    coding: '编码中',
    ci: 'CI 中',
    failed: '失败',
  }
  return labels[status] ?? status
}

export function kindLabel(kind: string): string {
  const labels: Record<string, string> = {
    bug: 'Bug',
    visual: '体验优化',
    feature: '功能建议',
  }
  return labels[kind] ?? kind
}

export function roleLabel(role: string): string {
  const labels: Record<string, string> = {
    user: '用户',
    agent: 'Agent',
    admin: '运营',
  }
  return labels[role] ?? role
}

export function initialOf(name: string): string {
  const trimmed = name.trim()
  return trimmed ? trimmed.slice(0, 1).toUpperCase() : '?'
}

export function formatRelative(iso: string, now = Date.now()): string {
  const time = Date.parse(iso)
  if (Number.isNaN(time)) {
    return ''
  }
  const diff = Math.max(0, now - time)
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  if (diff < minute) return '刚刚'
  if (diff < hour) return `${Math.floor(diff / minute)} 分钟前`
  if (diff < day) return `${Math.floor(diff / hour)} 小时前`
  if (diff < 7 * day) return `${Math.floor(diff / day)} 天前`
  return new Date(time).toLocaleDateString('zh-CN', { month: '2-digit', day: '2-digit' })
}

export function formatShortTime(iso: string): string {
  const time = Date.parse(iso)
  if (Number.isNaN(time)) {
    return ''
  }
  return new Date(time).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export interface ChatDayGroup {
  key: string
  label: string
  messages: EvolutionChatMessage[]
}

export function groupMessagesByDay(messages: EvolutionChatMessage[]): ChatDayGroup[] {
  const groups = new Map<string, ChatDayGroup>()
  for (const message of messages) {
    const key = message.createdAt.slice(0, 10)
    const existing = groups.get(key)
    if (existing) {
      existing.messages.push(message)
      continue
    }
    groups.set(key, {
      key,
      label: new Date(message.createdAt).toLocaleDateString('zh-CN', {
        month: 'long',
        day: 'numeric',
      }),
      messages: [message],
    })
  }
  return Array.from(groups.values()).sort((a, b) => b.key.localeCompare(a.key))
}

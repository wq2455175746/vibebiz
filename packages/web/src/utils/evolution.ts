import type {
  EvolutionChatMessage,
  EvolutionVersion,
  EvolutionVersionGroup,
} from '@/types/evolution'

export const EVOLUTION_PROJECT_ID = 'vibebiz-evolve'
export const EVOLUTION_PREVIEW_PATH = '/evolution/origin_version/index.html'
export const OFFICIAL_USER_ID = 'official'
export const OFFICIAL_VERSION_LABEL = 'v1.2026.11'

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

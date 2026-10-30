import type { AgentRole } from '@/types'

export interface RoleTrigger {
  start: number
  end: number
  query: string
}

export interface RoleSaveContext {
  projectId: string
  version: number
}

export function roleSaveContextMatches(saved: RoleSaveContext, current: RoleSaveContext): boolean {
  return saved.projectId === current.projectId && saved.version === current.version
}

export function roleDisplayLabel(role: AgentRole): string {
  return `@${role.roleCode} ${role.roleNameZh || role.roleNameEn || role.roleCode}`
}

export function findRoleTrigger(value: string, cursor: number): RoleTrigger | null {
  const before = value.slice(0, cursor)
  const start = before.lastIndexOf('#')
  if (start < 0) {
    return null
  }
  const query = before.slice(start + 1)
  if (/\s/.test(query)) {
    return null
  }
  return { start, end: cursor, query }
}

export function consumeRoleTrigger(
  value: string,
  start: number,
  end: number,
): { value: string; cursor: number } {
  return {
    value: `${value.slice(0, start)}${value.slice(end)}`,
    cursor: start,
  }
}

export function filterRoleOptions(roles: AgentRole[], query: string): AgentRole[] {
  const keyword = query.trim().toLowerCase()
  if (!keyword) {
    return roles
  }
  return roles.filter(
    (role) =>
      role.roleCode.toLowerCase().includes(keyword) ||
      role.roleNameEn.toLowerCase().includes(keyword) ||
      role.roleNameZh.toLowerCase().includes(keyword) ||
      role.description.toLowerCase().includes(keyword),
  )
}

export function toggleRoleSelection(roles: AgentRole[], selected: number[], id: number): number[] {
  const target = roles.find((role) => role.id === id)
  if (!target) {
    return selected
  }
  if (target.roleCode === 'team') {
    return selected.includes(id) ? [] : [id]
  }
  const withoutTeam = selected.filter(
    (item) => roles.find((role) => role.id === item)?.roleCode !== 'team',
  )
  if (withoutTeam.includes(id)) {
    return withoutTeam.filter((item) => item !== id)
  }
  return [...withoutTeam, id]
}

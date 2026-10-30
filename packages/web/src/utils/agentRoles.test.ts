import { describe, expect, it } from 'vitest'

import {
  consumeRoleTrigger,
  filterRoleOptions,
  findRoleTrigger,
  roleDisplayLabel,
  roleSaveContextMatches,
  toggleRoleSelection,
} from './agentRoles'

const roles = [
  { id: 6, roleCode: 'team', roleNameEn: 'Team', roleNameZh: '团队', description: 'all', avatarUrl: '' },
  { id: 1, roleCode: 'alex', roleNameEn: 'Alex', roleNameZh: '全栈工程师', description: 'code', avatarUrl: '' },
  { id: 4, roleCode: 'emma', roleNameEn: 'Emma', roleNameZh: '产品经理', description: 'product', avatarUrl: '' },
]

describe('agent role selection', () => {
  it('finds a # token and consumes it', () => {
    expect(findRoleTrigger('做 @看板 #团队', 9)).toEqual({ start: 6, end: 9, query: '团队' })
    expect(consumeRoleTrigger('做 @看板 #团队', 6, 9)).toEqual({ value: '做 @看板 ', cursor: 6 })
  })

  it('returns null when there is no # or whitespace follows the token', () => {
    expect(findRoleTrigger('没有角色', 4)).toBeNull()
    expect(findRoleTrigger('hello #foo bar', 14)).toBeNull()
    expect(findRoleTrigger('# team', 6)).toBeNull()
  })

  it('keeps team exclusive', () => {
    expect(toggleRoleSelection(roles, [1, 4], 6)).toEqual([6])
    expect(toggleRoleSelection(roles, [6], 1)).toEqual([1])
    expect(toggleRoleSelection(roles, [1], 1)).toEqual([])
    expect(toggleRoleSelection(roles, [6], 6)).toEqual([])
  })

  it('adds and removes normal roles without duplicates', () => {
    expect(toggleRoleSelection(roles, [], 1)).toEqual([1])
    expect(toggleRoleSelection(roles, [4], 1)).toEqual([4, 1])
    expect(toggleRoleSelection(roles, [1, 4], 4)).toEqual([1])
    expect(toggleRoleSelection(roles, [1], 4)).toEqual([1, 4])
    expect(toggleRoleSelection(roles, [1], 1)).toEqual([])
  })

  it('deduplicates repeated role toggles', () => {
    expect(toggleRoleSelection(roles, [1, 4], 1)).toEqual([4])
    expect(toggleRoleSelection(roles, [1, 4], 4)).toEqual([1])
  })

  it('returns role chips in selected order', () => {
    const ids = [1, 4]
    expect(ids.map((id) => roles.find((role) => role.id === id)?.roleNameZh)).toEqual([
      '全栈工程师',
      '产品经理',
    ])
  })

  it('returns the selection unchanged for an unknown role', () => {
    expect(toggleRoleSelection(roles, [1, 4], 99)).toEqual([1, 4])
  })
})

describe('agent role save context guard', () => {
  const saved = { projectId: 'project-a', version: 3 }

  it('accepts the same project and context version', () => {
    expect(roleSaveContextMatches(saved, { projectId: 'project-a', version: 3 })).toBe(true)
  })

  it('rejects a response from a different project', () => {
    expect(roleSaveContextMatches(saved, { projectId: 'project-b', version: 3 })).toBe(false)
  })

  it('rejects a response after the project context version advances', () => {
    expect(roleSaveContextMatches(saved, { projectId: 'project-a', version: 4 })).toBe(false)
  })
})

describe('agent role labels', () => {
  it('formats the role code and Chinese name for the editor header', () => {
    expect(roleDisplayLabel(roles[1])).toBe('@alex 全栈工程师')
    expect(roleDisplayLabel(roles[0])).toBe('@team 团队')
  })

  it('falls back to the English name when the Chinese name is empty', () => {
    expect(roleDisplayLabel({ ...roles[1], roleNameZh: '' })).toBe('@alex Alex')
  })
})

describe('agent role filtering', () => {
  it('returns all roles for a blank query', () => {
    expect(filterRoleOptions(roles, '')).toEqual(roles)
    expect(filterRoleOptions(roles, '   ')).toEqual(roles)
  })

  it('filters by role code, names, and description', () => {
    expect(filterRoleOptions(roles, 'alex').map((role) => role.id)).toEqual([1])
    expect(filterRoleOptions(roles, 'Emma').map((role) => role.id)).toEqual([4])
    expect(filterRoleOptions(roles, '全栈').map((role) => role.id)).toEqual([1])
    expect(filterRoleOptions(roles, 'product').map((role) => role.id)).toEqual([4])
  })

  it('matches case-insensitively', () => {
    expect(filterRoleOptions(roles, 'TEAM').map((role) => role.id)).toEqual([6])
    expect(filterRoleOptions(roles, 'PrOdUcT').map((role) => role.id)).toEqual([4])
  })
})

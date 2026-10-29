import { describe, expect, it } from 'vitest'

import { buildRoleInput, type RoleFormState } from './roleForm'

function form(overrides: Partial<RoleFormState> = {}): RoleFormState {
  return {
    roleCode: 'frank',
    roleNameEn: 'Frank',
    roleNameZh: '测试工程师',
    description: '负责测试策略与用例。',
    systemPrompt: '你是 Frank，测试工程师。',
    avatarKey: '',
    sortOrder: 60,
    status: 1,
    ...overrides,
  }
}

describe('buildRoleInput', () => {
  it('builds a role input', () => {
    const result = buildRoleInput(form())
    expect('input' in result && result.input.roleCode).toBe('frank')
  })

  it('rejects missing required fields', () => {
    expect(buildRoleInput(form({ roleCode: '' }))).toEqual({ error: '请填写角色编码' })
    expect(buildRoleInput(form({ systemPrompt: '' }))).toEqual({ error: '请填写 System Prompt' })
  })

  it('rejects invalid role code', () => {
    expect('error' in buildRoleInput(form({ roleCode: 'Frank' }))).toBe(true)
    expect('error' in buildRoleInput(form({ roleCode: 'bad code' }))).toBe(true)
  })

  it('rejects overlong names and description', () => {
    expect('error' in buildRoleInput(form({ roleNameZh: 'a'.repeat(65) }))).toBe(true)
    expect('error' in buildRoleInput(form({ description: 'a'.repeat(1001) }))).toBe(true)
  })

  it('normalizes sort order and status', () => {
    const result = buildRoleInput(form({ sortOrder: Number.NaN, status: 1 }))
    expect('input' in result && result.input.sortOrder).toBe(0)
  })
})

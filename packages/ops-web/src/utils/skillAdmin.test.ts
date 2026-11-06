import { describe, expect, it } from 'vitest'

import { validateSkillScope } from './skillAdmin'

describe('skill admin', () => {
  it('requires a user for user scope', () => {
    expect(validateSkillScope('user', null)).toBe('请选择用户')
    expect(validateSkillScope('user', 7)).toBe('')
  })

  it('does not require a user for public scope', () => {
    expect(validateSkillScope('public', null)).toBe('')
  })
})

import { describe, expect, it } from 'vitest'

import { validateDisableReason, validateNewPassword } from './userAdmin'

describe('user admin validation', () => {
  it('requires a disable reason', () => {
    expect(validateDisableReason('   ')).toBe('请输入禁用原因')
    expect(validateDisableReason('违规内容')).toBe('')
  })

  it('limits the disable reason to 500 characters', () => {
    expect(validateDisableReason('x'.repeat(501))).toBe('禁用原因不能超过 500 字')
  })

  it('requires a password of at least 8 characters', () => {
    expect(validateNewPassword('short', 'short')).toBe('密码至少 8 位')
  })

  it('requires matching passwords', () => {
    expect(validateNewPassword('new-password', 'other')).toBe('两次密码不一致')
    expect(validateNewPassword('new-password', 'new-password')).toBe('')
  })
})

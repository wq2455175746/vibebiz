import { describe, expect, it } from 'vitest'

import {
  validateDisableReason,
  validateNewPassword,
  validatePasswordChange,
} from './userAdmin'

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

  it('validates the current password change form', () => {
    expect(validatePasswordChange('', 'new-password', 'new-password')).toBe('请输入当前密码')
    expect(validatePasswordChange('old-password', 'short', 'short')).toBe('新密码至少 8 位')
    expect(validatePasswordChange('old-password', 'x'.repeat(73), 'x'.repeat(73))).toBe(
      '新密码不能超过 72 位',
    )
    expect(validatePasswordChange('old-password', 'new-password', 'other')).toBe('两次密码不一致')
    expect(validatePasswordChange('old-password', 'old-password', 'old-password')).toBe(
      '新密码不能与当前密码相同',
    )
    expect(validatePasswordChange('old-password', 'new-password', 'new-password')).toBe('')
  })
})

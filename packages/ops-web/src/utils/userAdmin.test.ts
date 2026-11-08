import { describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'

import {
  createUserErrorMessage,
  validateCreateUser,
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

describe('create user validation', () => {
  const valid = {
    username: 'tester01',
    password: 'secret123',
    confirmPassword: 'secret123',
    email: '',
    nickname: '',
    plan: 'free',
    status: 1,
  }

  it('accepts a valid form', () => {
    expect(validateCreateUser(valid)).toBe('')
  })

  it('validates the username', () => {
    expect(validateCreateUser({ ...valid, username: 'ab' })).toBe(
      '用户名需为 3-32 位字母、数字、下划线或中划线',
    )
    expect(validateCreateUser({ ...valid, username: 'bad name' })).toBe(
      '用户名需为 3-32 位字母、数字、下划线或中划线',
    )
  })

  it('validates the password', () => {
    expect(validateCreateUser({ ...valid, password: 'short', confirmPassword: 'short' })).toBe(
      '密码至少 8 位',
    )
    expect(
      validateCreateUser({
        ...valid,
        password: 'x'.repeat(73),
        confirmPassword: 'x'.repeat(73),
      }),
    ).toBe('密码不能超过 72 位')
    expect(validateCreateUser({ ...valid, confirmPassword: 'other' })).toBe('两次密码不一致')
  })

  it('validates the optional email', () => {
    expect(validateCreateUser({ ...valid, email: 'not-an-email' })).toBe('请输入有效的邮箱')
    expect(validateCreateUser({ ...valid, email: 'tester01@example.com' })).toBe('')
  })

  it('validates the plan and status', () => {
    expect(validateCreateUser({ ...valid, plan: 'vip' })).toBe('请选择套餐')
    expect(validateCreateUser({ ...valid, status: 2 })).toBe('请选择状态')
  })
})

describe('create user error message', () => {
  it('maps conflicts to the spec copy', () => {
    expect(
      createUserErrorMessage(new ApiError('username or email already exists', 409, 40901)),
    ).toBe('用户名或邮箱已存在')
  })

  it('falls back to the API error message', () => {
    expect(createUserErrorMessage(new ApiError('server error', 500, 50001))).toBe('server error')
  })

  it('uses the plain error message', () => {
    expect(createUserErrorMessage(new Error('boom'))).toBe('boom')
  })

  it('falls back for non-error input', () => {
    expect(createUserErrorMessage('nope')).toBe('创建用户失败')
  })
})

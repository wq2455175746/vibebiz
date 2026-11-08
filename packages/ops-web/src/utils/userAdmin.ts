import { ApiError } from '@/api/client'

export function validateDisableReason(reason: string): string {
  const value = reason.trim()
  if (!value) {
    return '请输入禁用原因'
  }
  if ([...value].length > 500) {
    return '禁用原因不能超过 500 字'
  }
  return ''
}

export function validateNewPassword(password: string, confirm: string): string {
  if (password.length < 8) {
    return '密码至少 8 位'
  }
  if (password !== confirm) {
    return '两次密码不一致'
  }
  return ''
}

export interface CreateUserForm {
  username: string
  password: string
  confirmPassword: string
  email: string
  nickname: string
  plan: string
  status: number
}

const usernamePattern = /^[A-Za-z0-9_-]{3,32}$/
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

export function validateCreateUser(form: CreateUserForm): string {
  if (!usernamePattern.test(form.username.trim())) {
    return '用户名需为 3-32 位字母、数字、下划线或中划线'
  }
  if (form.password.length < 8) {
    return '密码至少 8 位'
  }
  if (form.password.length > 72) {
    return '密码不能超过 72 位'
  }
  if (form.password !== form.confirmPassword) {
    return '两次密码不一致'
  }
  const email = form.email.trim()
  if (email && (email.length > 255 || !emailPattern.test(email))) {
    return '请输入有效的邮箱'
  }
  if ([...form.nickname.trim()].length > 255) {
    return '昵称不能超过 255 字'
  }
  if (!['free', 'pro', 'max'].includes(form.plan)) {
    return '请选择套餐'
  }
  if (form.status !== 0 && form.status !== 1) {
    return '请选择状态'
  }
  return ''
}

export function createUserErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 409 || error.code === 40901) {
      return '用户名或邮箱已存在'
    }
    return error.message || '创建用户失败'
  }
  return error instanceof Error ? error.message : '创建用户失败'
}

export function validatePasswordChange(
  oldPassword: string,
  newPassword: string,
  confirm: string,
): string {
  if (!oldPassword) {
    return '请输入当前密码'
  }
  if (newPassword.length < 8) {
    return '新密码至少 8 位'
  }
  if (newPassword.length > 72) {
    return '新密码不能超过 72 位'
  }
  if (newPassword !== confirm) {
    return '两次密码不一致'
  }
  if (newPassword === oldPassword) {
    return '新密码不能与当前密码相同'
  }
  return ''
}

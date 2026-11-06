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

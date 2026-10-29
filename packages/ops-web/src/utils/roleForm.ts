import type { RoleInput } from '@/types'

export interface RoleFormState {
  roleCode: string
  roleNameEn: string
  roleNameZh: string
  description: string
  systemPrompt: string
  avatarKey: string
  sortOrder: number
  status: number
}

const roleCodePattern = /^[a-z][a-z0-9_-]{0,31}$/

export function buildRoleInput(
  form: RoleFormState,
): { input: RoleInput } | { error: string } {
  const roleCode = form.roleCode.trim()
  const roleNameEn = form.roleNameEn.trim()
  const roleNameZh = form.roleNameZh.trim()
  const description = form.description.trim()
  const systemPrompt = form.systemPrompt.trim()

  if (!roleCode) return { error: '请填写角色编码' }
  if (!roleCodePattern.test(roleCode)) {
    return { error: '角色编码必须以小写字母开头，只能包含小写字母、数字、下划线和中划线' }
  }
  if (!roleNameEn) return { error: '请填写英文名' }
  if (!roleNameZh) return { error: '请填写中文名' }
  if (roleNameEn.length > 64 || roleNameZh.length > 64) {
    return { error: '角色名称不能超过 64 个字符' }
  }
  if (description.length > 1000) return { error: '角色描述不能超过 1000 个字符' }
  if (!systemPrompt) return { error: '请填写 System Prompt' }
  if (systemPrompt.length > 100000) return { error: 'System Prompt 过长' }

  const sortOrder = Number.isFinite(form.sortOrder) ? Math.trunc(form.sortOrder) : 0
  const status = form.status === 0 ? 0 : 1

  return {
    input: {
      roleCode,
      roleNameEn,
      roleNameZh,
      description,
      systemPrompt,
      avatarKey: form.avatarKey || undefined,
      sortOrder,
      status,
    },
  }
}

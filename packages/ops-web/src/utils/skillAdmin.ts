export type SkillScope = 'public' | 'user'

export function validateSkillScope(scope: SkillScope, userId: number | null): string {
  if (scope === 'user' && (!userId || userId <= 0)) {
    return '请选择用户'
  }
  return ''
}

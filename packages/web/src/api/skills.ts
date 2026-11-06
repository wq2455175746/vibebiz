import { cpHttp, dpHttp, request } from './client'

export interface SkillInfo {
  name: string
  description: string
  source: 'user' | 'project' | 'plugin' | 'public'
  updatedAt?: string
}

export async function listSkills(projectId: string): Promise<SkillInfo[]> {
  const result = await request<{ skills: SkillInfo[] }>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/skills`,
  })
  return result.skills
}

export async function listMySkills(): Promise<SkillInfo[]> {
  const result = await request<{ skills: SkillInfo[] }>(cpHttp, {
    method: 'get',
    url: '/me/skills',
  })
  return result.skills
}

export async function uploadMySkill(file: File, overwrite = false): Promise<SkillInfo> {
  const form = new FormData()
  form.set('file', file)
  form.set('overwrite', String(overwrite))
  return request<SkillInfo>(cpHttp, { method: 'post', url: '/me/skills', data: form })
}

export async function deleteMySkill(name: string): Promise<void> {
  await request<{ deleted: boolean }>(cpHttp, {
    method: 'delete',
    url: `/me/skills/${encodeURIComponent(name)}`,
  })
}

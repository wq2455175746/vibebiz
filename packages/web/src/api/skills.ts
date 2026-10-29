import { dpHttp, request } from './client'

export interface SkillInfo {
  name: string
  description: string
  source: 'user' | 'project' | 'plugin'
}

export async function listSkills(projectId: string): Promise<SkillInfo[]> {
  const result = await request<{ skills: SkillInfo[] }>(dpHttp, {
    method: 'get',
    url: `/projects/${encodeURIComponent(projectId)}/skills`,
  })
  return result.skills
}

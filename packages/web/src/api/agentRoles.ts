import { cpHttp, request } from './client'
import type { AgentRole } from '@/types'

export async function listAgentRoles(): Promise<AgentRole[]> {
  const result = await request<{ roles: AgentRole[] }>(cpHttp, {
    method: 'get',
    url: '/agent-roles',
  })
  return result.roles
}

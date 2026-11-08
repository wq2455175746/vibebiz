import type {
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionTask,
  EvolutionVersion,
  TaskStatus,
} from '@/types/evolution'

import { request } from './client'

export interface EvolutionSnapshot {
  projectId: string
  originVersion: string
  versions: EvolutionVersion[]
  proposals: EvolutionProposal[]
  tasks: EvolutionTask[]
}

export interface EvolutionConversation {
  messages: EvolutionChatMessage[]
  proposals: EvolutionProposal[]
}

export async function getSnapshot(): Promise<EvolutionSnapshot> {
  return request<EvolutionSnapshot>({ method: 'get', url: '/evolve' })
}

export async function getConversation(userId: string): Promise<EvolutionConversation> {
  return request<EvolutionConversation>({
    method: 'get',
    url: `/evolve/conversations/${encodeURIComponent(userId)}`,
  })
}

export async function reply(versionId: string, content: string): Promise<EvolutionChatMessage> {
  const result = await request<{ message: EvolutionChatMessage }>({
    method: 'post',
    url: `/evolve/versions/${encodeURIComponent(versionId)}/reply`,
    data: { content },
  })
  return result.message
}

export async function submitTask(versionId: string): Promise<EvolutionTask> {
  const result = await request<{ task: EvolutionTask }>({
    method: 'post',
    url: '/evolve/tasks',
    data: { versionId },
  })
  return result.task
}

export async function updateTask(id: string, status: TaskStatus): Promise<EvolutionTask> {
  const result = await request<{ task: EvolutionTask }>({
    method: 'patch',
    url: `/evolve/tasks/${encodeURIComponent(id)}`,
    data: { status },
  })
  return result.task
}

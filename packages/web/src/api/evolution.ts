import type { EvolutionChatMessage, EvolutionProposal, EvolutionVersion } from '@/types/evolution'

import { cpHttp, request } from './client'

export interface EvolutionVersionsResponse {
  projectId: string
  originVersion: string
  versions: EvolutionVersion[]
}

export interface EvolutionConversation {
  messages: EvolutionChatMessage[]
  proposals: EvolutionProposal[]
}

export interface EvolutionProposalResult {
  proposal: EvolutionProposal
  version: EvolutionVersion | null
}

export async function listVersions(): Promise<EvolutionVersionsResponse> {
  return request<EvolutionVersionsResponse>(cpHttp, { method: 'get', url: '/evolve' })
}

export async function getConversation(userId: string): Promise<EvolutionConversation> {
  return request<EvolutionConversation>(cpHttp, {
    method: 'get',
    url: `/evolve/conversations/${encodeURIComponent(userId)}`,
  })
}

export async function ensureDraft(): Promise<{ version: EvolutionVersion }> {
  return request<{ version: EvolutionVersion }>(cpHttp, {
    method: 'post',
    url: '/evolve/draft',
  })
}

export async function submitProposal(content: string): Promise<EvolutionProposalResult> {
  return request<EvolutionProposalResult>(cpHttp, {
    method: 'post',
    url: '/evolve/proposals',
    data: { content },
  })
}

export async function submitChat(content: string): Promise<{ messages: EvolutionChatMessage[] }> {
  return request<{ messages: EvolutionChatMessage[] }>(cpHttp, {
    method: 'post',
    url: '/evolve/chat',
    data: { content },
  })
}

export async function confirmVersion(id: string): Promise<{ version: EvolutionVersion }> {
  return request<{ version: EvolutionVersion }>(cpHttp, {
    method: 'post',
    url: `/evolve/versions/${encodeURIComponent(id)}/confirm`,
  })
}

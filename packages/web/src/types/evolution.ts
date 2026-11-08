export type ProposalKind = 'bug' | 'visual' | 'feature'

export type ProposalStatus = 'queued' | 'reviewing' | 'accepted' | 'rejected' | 'released'

export type VersionStatus = 'draft' | 'preview' | 'accepted' | 'submitted' | 'released'

export type ChatRole = 'user' | 'agent' | 'admin'

export interface EvolutionProposal {
  id: string
  createdAt: string
  userId: string
  userName: string
  kind: ProposalKind
  content: string
  status: ProposalStatus
  agentReply: string
  versionId: string | null
}

export interface EvolutionVersion {
  id: string
  userId: string
  userName: string
  label: string
  createdAt: string
  updatedAt: string
  status: VersionStatus
  scenario: string
  summary: string
  originVersion: string
  proposalIds: string[] | null
}

export interface EvolutionChatMessage {
  id: string
  createdAt: string
  role: ChatRole
  userName: string
  content: string
}

export interface EvolutionVersionGroup {
  userId: string
  userName: string
  latest: EvolutionVersion
  versions: EvolutionVersion[]
}

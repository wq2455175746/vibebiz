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
  proposalIds: string[]
}

export interface EvolutionChatMessage {
  id: string
  createdAt: string
  role: ChatRole
  userName: string
  content: string
}

export interface EvolutionState {
  proposals: EvolutionProposal[]
  versions: EvolutionVersion[]
  chats: Record<string, EvolutionChatMessage[]>
  selectedVersionId: string | null
  selectedUserId: string | null
}

export interface EvolutionVersionGroup {
  userId: string
  userName: string
  latest: EvolutionVersion
  versions: EvolutionVersion[]
}

export interface EvolutionViewer {
  id: string
  name: string
}

export interface EvolutionTask {
  id: string
  versionId: string
  userId: string
  userName: string
  versionLabel: string
  originVersion: string
  chatSummary: string
  status: 'queued' | 'coding' | 'ci' | 'released' | 'failed'
  createdAt: string
}

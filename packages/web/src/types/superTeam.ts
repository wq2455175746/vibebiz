export type EventSourceKind = 'feishu' | 'webhook' | 'email' | 'queue'
export type EventSourceStatus = 'connected' | 'configured' | 'unconfigured' | 'coming_soon'
export type SuperStepKey = 'requirements' | 'development' | 'operations'
export type SuperStepStatus = 'locked' | 'ready' | 'active' | 'done'
export type SuperEventStage = 'received' | 'filtered' | 'evaluated' | 'routed' | 'ignored'
export type AgentActionKind = 'noise_filter' | 'semantic_eval' | 'artifact' | 'step_advance'
export type AgentActionStatus = 'success' | 'failed' | 'running'

export interface EventSourceField {
  key: string
  label: string
  placeholder: string
  secret?: boolean
}

export interface EventSourceConfig {
  kind: EventSourceKind
  enabled: boolean
  status: EventSourceStatus
  lastEventAt: string | null
  eventCount: number
  fields: Record<string, string>
}

export interface SuperStepState {
  key: SuperStepKey
  status: SuperStepStatus
  progress: number
  updatedAt: string | null
  summary: string
}

export interface SuperEvent {
  id: string
  source: EventSourceKind
  title: string
  receivedAt: string
  noiseScore: number
  semanticScore: number
  stage: SuperEventStage
  routedStep: SuperStepKey | null
}

export interface AgentActionAudit {
  id: string
  eventId: string
  step: SuperStepKey
  agent: string
  action: AgentActionKind
  status: AgentActionStatus
  durationMs: number
  createdAt: string
}

export interface SuperTeamState {
  sources: EventSourceConfig[]
  steps: SuperStepState[]
  events: SuperEvent[]
  audit: AgentActionAudit[]
}

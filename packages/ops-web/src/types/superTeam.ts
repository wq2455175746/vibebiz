export interface AdminSuperTeamSource {
  id: number
  owner: string
  kind: string
  status: string
  enabled: boolean
  lastEventAt: string | null
  eventCount: number
  updatedAt: string
}

export interface AdminSuperTeamPipeline {
  id: number
  owner: string
  workspace: string
  requirements: string
  development: string
  operations: string
  currentStep: string
  updatedAt: string
}

export interface AdminSuperTeamAction {
  id: string
  eventId: string
  source: string
  step: string
  agent: string
  action: string
  noiseScore: number
  semanticScore: number
  status: string
  durationMs: number
  createdAt: string
}

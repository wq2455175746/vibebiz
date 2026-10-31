export type CloneStageKey = 'base_design' | 'architecture' | 'test' | 'development' | 'artifacts'

export type CloneStageStatus = 'locked' | 'active' | 'done'

export type CloneStageWorkspace = 'document' | 'test' | 'development' | 'artifact'

export interface CloneStageState {
  status: CloneStageStatus
  startedAt: string | null
  completedAt: string | null
}

export interface CloneStageDefinition {
  key: CloneStageKey
  seq: number
  title: string
  roleCode: string
  roleLabel: string
  artifactPath: string
  workspace: CloneStageWorkspace
  summary: string
}

export type CloneDraftAssetRole = 'image' | 'unit_test' | 'docs'

export interface CloneDraftAsset {
  id: string
  name: string
  size: number
  mime: string
  role: CloneDraftAssetRole
  previewUrl?: string
  file?: File
}

export type CloneAssetRole = 'cover' | 'gallery' | 'unit_test' | 'docs'

export interface CloneAssetView {
  id: string
  resourceId: string
  name: string
  displayName: string
  path: string
  contentType: string
  size: number
  kind: string
  role: CloneAssetRole
  sortOrder: number
  createdAt: string
}

export interface CloneProject {
  id: string
  name: string
  description: string
  sourceUrl: string
  mode: 'clone'
  state: string
  workspaceId: number
  coverS3Path: string
  coverUrl: string
  assets: CloneAssetView[]
  createdAt: string
  updatedAt: string
}

export interface CloneDraft {
  name: string
  description: string
  sourceUrl: string
}

export type CloneDraftField = 'name' | 'description' | 'sourceUrl'

export type CloneDraftErrors = Partial<Record<CloneDraftField, string>>

export interface CloneCreatePayload {
  draft: CloneDraft
  assets: CloneDraftAsset[]
  coverAssetId: string | null
}

export interface CloneTestResult {
  status: 'passed' | 'failed' | 'skipped'
  command: string[]
  exitCode: number
  output: string
  durationMs: number
  reportPath: string
}

export interface FileLike {
  name: string
  size: number
  type: string
}

import type {
  ArtifactFlowState,
  DeliveryFlowState,
  EnterpriseFlow,
  HarborConnection,
  MockStepState,
} from '@/types/enterpriseSpace'
import type { EnterpriseStageStateMap } from '@/utils/enterprise'

export const FLOWS_STORAGE_KEY = 'vibebiz:enterprise:flows:v1'

interface PersistedFlows {
  version: 1
  projects: Record<string, EnterpriseFlow>
}

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>

export function createEmptyStep(): MockStepState {
  return { status: 'idle', startedAt: null, completedAt: null, error: '' }
}

export function createArtifactFlow(active = false): ArtifactFlowState {
  return {
    status: active ? 'active' : 'locked',
    startedAt: active ? new Date().toISOString() : null,
    completedAt: null,
    codeRepoId: '',
    branch: '',
    commitMessage: 'chore: enterprise delivery snapshot',
    code: createEmptyStep(),
    jenkinsId: '',
    jenkinsJob: '',
    jenkins: createEmptyStep(),
    harborId: '',
    image: 'vibebiz-app',
    tag: 'latest',
    harbor: createEmptyStep(),
  }
}

export function createDeliveryFlow(): DeliveryFlowState {
  return {
    status: 'locked',
    startedAt: null,
    completedAt: null,
    method: null,
    targetId: '',
    namespace: '',
    note: '',
    execution: createEmptyStep(),
  }
}

export function createEnterpriseFlow(projectId: string): EnterpriseFlow {
  return {
    projectId,
    artifact: createArtifactFlow(),
    delivery: createDeliveryFlow(),
    updatedAt: new Date().toISOString(),
  }
}

export function loadFlows(storage: ReadStorage = localStorage): Record<string, EnterpriseFlow> {
  try {
    const raw = storage.getItem(FLOWS_STORAGE_KEY)
    if (!raw) return {}
    const parsed = JSON.parse(raw) as Partial<PersistedFlows>
    if (parsed.version !== 1 || !parsed.projects || typeof parsed.projects !== 'object') return {}
    return parsed.projects
  } catch {
    return {}
  }
}

export function saveFlows(
  flows: Record<string, EnterpriseFlow>,
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(FLOWS_STORAGE_KEY, JSON.stringify({ version: 1, projects: flows }))
}

export function artifactStepUnlocked(
  flow: ArtifactFlowState,
  step: 'code' | 'jenkins' | 'harbor',
): boolean {
  if (flow.status === 'done') return true
  if (flow.status !== 'active') return false
  if (step === 'code') return true
  if (step === 'jenkins') return flow.code.status === 'success'
  return flow.jenkins.status === 'success'
}

export function artifactReady(flow: ArtifactFlowState): boolean {
  return (
    flow.code.status === 'success' &&
    flow.jenkins.status === 'success' &&
    flow.harbor.status === 'success'
  )
}

export function deliveryReady(flow: DeliveryFlowState): boolean {
  return (flow.status === 'active' || flow.status === 'done') && flow.execution.status === 'success'
}

export function dockerPullCommand(flow: ArtifactFlowState, harbor: HarborConnection): string {
  const parts = [
    harbor.registryUrl.replace(/\/+$/, ''),
    harbor.namespace.replace(/^\/+|\/+$/g, ''),
    flow.image.trim(),
  ].filter(Boolean)
  return `${parts.join('/')}:${flow.tag.trim() || 'latest'}`
}

export function mergeEnterpriseStageStates(
  states: EnterpriseStageStateMap,
  flow: EnterpriseFlow,
): EnterpriseStageStateMap {
  const next: EnterpriseStageStateMap = {
    ...states,
    artifacts: { ...states.artifacts },
    delivery: { ...states.delivery },
  }
  if (states.development.status !== 'done') return next

  if (states.artifacts.status === 'placeholder') {
    next.artifacts = {
      status: flow.artifact.status === 'done' ? 'done' : 'active',
      startedAt: flow.artifact.startedAt,
      completedAt: flow.artifact.completedAt,
    }
  }

  if (states.delivery.status === 'placeholder') {
    next.delivery = {
      status:
        flow.delivery.status === 'done'
          ? 'done'
          : flow.delivery.status === 'active'
            ? 'active'
            : 'locked',
      startedAt: flow.delivery.startedAt,
      completedAt: flow.delivery.completedAt,
    }
  }

  return next
}

export function downloadTextFile(filename: string, content: string): void {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = filename
  anchor.click()
  URL.revokeObjectURL(url)
}

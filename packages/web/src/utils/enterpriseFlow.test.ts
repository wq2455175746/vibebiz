import { describe, expect, it } from 'vitest'

import type { HarborConnection } from '@/types/enterpriseSpace'
import { createInitialStageState } from './enterprise'
import {
  FLOWS_STORAGE_KEY,
  artifactReady,
  artifactStepUnlocked,
  createArtifactFlow,
  createDeliveryFlow,
  createEnterpriseFlow,
  deliveryReady,
  dockerPullCommand,
  loadFlows,
  mergeEnterpriseStageStates,
  saveFlows,
} from './enterpriseFlow'

const harbor: HarborConnection = {
  id: 'h1',
  kind: 'harbor',
  name: '镜像库',
  registryUrl: 'https://harbor.example.com/',
  namespace: 'vibebiz',
  username: 'robot',
  secret: 'password',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
  }
}

describe('enterprise flow utilities', () => {
  it('unlocks artifact steps in order', () => {
    const flow = createArtifactFlow(true)
    expect(artifactStepUnlocked(flow, 'code')).toBe(true)
    expect(artifactStepUnlocked(flow, 'jenkins')).toBe(false)

    flow.code = { ...flow.code, status: 'success' }
    expect(artifactStepUnlocked(flow, 'jenkins')).toBe(true)
    expect(artifactStepUnlocked(flow, 'harbor')).toBe(false)

    flow.jenkins = { ...flow.jenkins, status: 'success' }
    expect(artifactStepUnlocked(flow, 'harbor')).toBe(true)
  })

  it('requires all three steps before artifact confirmation', () => {
    const flow = createArtifactFlow(true)
    expect(artifactReady(flow)).toBe(false)
    flow.code = { ...flow.code, status: 'success' }
    flow.jenkins = { ...flow.jenkins, status: 'success' }
    expect(artifactReady(flow)).toBe(false)
    flow.harbor = { ...flow.harbor, status: 'success' }
    expect(artifactReady(flow)).toBe(true)
  })

  it('builds a docker pull command without duplicate slashes', () => {
    const flow = createArtifactFlow(true)
    flow.image = 'vibebiz-app'
    flow.tag = 'v1'
    expect(dockerPullCommand(flow, harbor)).toBe(
      'https://harbor.example.com/vibebiz/vibebiz-app:v1',
    )
  })

  it('overlays stages 6/7 only after development is done', () => {
    const states = createInitialStageState()
    const flow = createEnterpriseFlow('p1')
    flow.artifact = { ...flow.artifact, status: 'active', startedAt: '2026-10-31T00:00:00Z' }

    expect(mergeEnterpriseStageStates(states, flow).artifacts.status).toBe('placeholder')

    states.development.status = 'done'
    const merged = mergeEnterpriseStageStates(states, flow)
    expect(merged.artifacts.status).toBe('active')
    expect(merged.delivery.status).toBe('locked')

    flow.artifact.status = 'done'
    flow.delivery = { ...flow.delivery, status: 'active' }
    const next = mergeEnterpriseStageStates(states, flow)
    expect(next.artifacts.status).toBe('done')
    expect(next.delivery.status).toBe('active')
  })

  it('round-trips flows and falls back to empty data', () => {
    const storage = memoryStorage()
    const flow = createEnterpriseFlow('p1')
    saveFlows({ p1: flow }, storage)
    expect(loadFlows(storage).p1.projectId).toBe('p1')

    storage.setItem(FLOWS_STORAGE_KEY, '{bad')
    expect(loadFlows(storage)).toEqual({})
  })

  it('creates locked delivery state by default', () => {
    expect(createDeliveryFlow().status).toBe('locked')
  })

  it('keeps the delivery result visible after confirmation', () => {
    const flow = createDeliveryFlow()
    flow.status = 'done'
    flow.execution = { ...flow.execution, status: 'success' }
    expect(deliveryReady(flow)).toBe(true)
  })
})

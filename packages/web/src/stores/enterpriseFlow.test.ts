import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useEnterpriseFlowStore } from './enterpriseFlow'

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  setActivePinia(createPinia())
})

describe('useEnterpriseFlowStore', () => {
  it('creates artifact flow when development completes', () => {
    const store = useEnterpriseFlowStore()
    const flow = store.ensureArtifact('p1')
    expect(flow.artifact.status).toBe('active')
    expect(flow.delivery.status).toBe('locked')
  })

  it('requires all steps before confirming artifact', () => {
    const store = useEnterpriseFlowStore()
    store.ensureArtifact('p1')
    expect(store.confirmArtifact('p1')).toBe(false)

    store.patchArtifactStep('p1', 'code', { status: 'success' })
    store.patchArtifactStep('p1', 'jenkins', { status: 'success' })
    store.patchArtifactStep('p1', 'harbor', { status: 'success' })
    expect(store.confirmArtifact('p1')).toBe(true)
    expect(store.get('p1')?.artifact.status).toBe('done')
    expect(store.get('p1')?.delivery.status).toBe('active')
  })

  it('only confirms delivery after a successful execution', () => {
    const store = useEnterpriseFlowStore()
    store.ensureArtifact('p1')
    store.patchArtifactStep('p1', 'code', { status: 'success' })
    store.patchArtifactStep('p1', 'jenkins', { status: 'success' })
    store.patchArtifactStep('p1', 'harbor', { status: 'success' })
    store.confirmArtifact('p1')

    expect(store.confirmDelivery('p1')).toBe(false)
    store.patchDelivery('p1', { method: 'k8s', targetId: 'k1' })
    store.patchDeliveryExecution('p1', { status: 'success' })
    expect(store.confirmDelivery('p1')).toBe(true)
    expect(store.get('p1')?.delivery.status).toBe('done')
  })

  it('persists flows across store instances', () => {
    const first = useEnterpriseFlowStore()
    first.ensureArtifact('p1')

    setActivePinia(createPinia())
    const second = useEnterpriseFlowStore()
    expect(second.get('p1')?.artifact.status).toBe('active')
  })
})

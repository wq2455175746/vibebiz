import { defineStore } from 'pinia'
import { ref } from 'vue'

import type {
  ArtifactFlowState,
  ArtifactStepKey,
  DeliveryFlowState,
  EnterpriseFlow,
  MockStepState,
} from '@/types/enterpriseSpace'
import {
  artifactReady,
  createArtifactFlow,
  createEnterpriseFlow,
  deliveryReady,
  loadFlows,
  saveFlows,
} from '@/utils/enterpriseFlow'

export const useEnterpriseFlowStore = defineStore('enterpriseFlow', () => {
  const flows = ref<Record<string, EnterpriseFlow>>(loadFlows())

  function persist(): void {
    saveFlows(flows.value)
  }

  function get(projectId: string): EnterpriseFlow | null {
    return flows.value[projectId] ?? null
  }

  function ensureArtifact(projectId: string): EnterpriseFlow {
    const existing = flows.value[projectId]
    if (existing) {
      if (existing.artifact.status === 'locked') {
        const next = {
          ...existing,
          artifact: {
            ...existing.artifact,
            status: 'active' as const,
            startedAt: new Date().toISOString(),
          },
          updatedAt: new Date().toISOString(),
        }
        flows.value = { ...flows.value, [projectId]: next }
        persist()
        return next
      }
      return existing
    }
    const flow = createEnterpriseFlow(projectId)
    flow.artifact = createArtifactFlow(true)
    flows.value = { ...flows.value, [projectId]: flow }
    persist()
    return flow
  }

  function patchArtifact(projectId: string, patch: Partial<ArtifactFlowState>): ArtifactFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      artifact: { ...flow.artifact, ...patch },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.artifact
  }

  function patchArtifactStep(
    projectId: string,
    step: ArtifactStepKey,
    patch: Partial<MockStepState>,
  ): ArtifactFlowState {
    const flow = ensureArtifact(projectId)
    const artifact = {
      ...flow.artifact,
      [step]: { ...flow.artifact[step], ...patch },
    }
    const next = {
      ...flow,
      artifact,
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.artifact
  }

  function confirmArtifact(projectId: string): boolean {
    const flow = flows.value[projectId]
    if (!flow || !artifactReady(flow.artifact)) return false
    const now = new Date().toISOString()
    const next = {
      ...flow,
      artifact: { ...flow.artifact, status: 'done' as const, completedAt: now },
      delivery: {
        ...flow.delivery,
        status: 'active' as const,
        startedAt: flow.delivery.startedAt ?? now,
      },
      updatedAt: now,
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return true
  }

  function patchDelivery(projectId: string, patch: Partial<DeliveryFlowState>): DeliveryFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      delivery: { ...flow.delivery, ...patch },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.delivery
  }

  function patchDeliveryExecution(
    projectId: string,
    patch: Partial<MockStepState>,
  ): DeliveryFlowState {
    const flow = ensureArtifact(projectId)
    const next = {
      ...flow,
      delivery: {
        ...flow.delivery,
        execution: { ...flow.delivery.execution, ...patch },
      },
      updatedAt: new Date().toISOString(),
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return next.delivery
  }

  function confirmDelivery(projectId: string): boolean {
    const flow = flows.value[projectId]
    if (!flow || !deliveryReady(flow.delivery)) return false
    const now = new Date().toISOString()
    const next = {
      ...flow,
      delivery: { ...flow.delivery, status: 'done' as const, completedAt: now },
      updatedAt: now,
    }
    flows.value = { ...flows.value, [projectId]: next }
    persist()
    return true
  }

  return {
    flows,
    get,
    ensureArtifact,
    patchArtifact,
    patchArtifactStep,
    confirmArtifact,
    patchDelivery,
    patchDeliveryExecution,
    confirmDelivery,
  }
})

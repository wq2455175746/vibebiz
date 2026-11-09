import { defineStore } from 'pinia'
import { ref } from 'vue'

import type {
  EventSourceConfig,
  EventSourceKind,
  SuperStepKey,
  SuperTeamBindings,
  SuperTeamState,
} from '@/types/superTeam'
import type { Project, ScheduledTask } from '@/types'
import {
  SUPER_TEAM_STORAGE_KEY,
  advanceStep,
  initialState,
  loadState,
  reconcileSteps,
  serializeState,
} from '@/utils/superTeam'

function readStored(): SuperTeamState {
  try {
    const raw =
      typeof localStorage === 'undefined' ? null : localStorage.getItem(SUPER_TEAM_STORAGE_KEY)
    return loadState(raw)
  } catch {
    return initialState()
  }
}

function sourceStatusFor(config: EventSourceConfig): EventSourceConfig['status'] {
  if (config.kind === 'queue') {
    return 'coming_soon'
  }
  if (config.enabled) {
    return 'connected'
  }
  const hasFields = Object.values(config.fields).some((value) => value.trim() !== '')
  return hasFields ? 'configured' : 'unconfigured'
}

export const useSuperTeamStore = defineStore('superTeam', () => {
  const initial = readStored()
  const sources = ref<EventSourceConfig[]>(initial.sources)
  const steps = ref(initial.steps)
  const events = ref(initial.events)
  const audit = ref(initial.audit)
  const bindings = ref<SuperTeamBindings>(initial.bindings)

  function persist(): void {
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.setItem(
          SUPER_TEAM_STORAGE_KEY,
          serializeState({
            sources: sources.value,
            steps: steps.value,
            events: events.value,
            audit: audit.value,
            bindings: bindings.value,
          }),
        )
      }
    } catch {
      return
    }
  }

  function updateSource(kind: EventSourceKind, patch: Partial<EventSourceConfig>): void {
    sources.value = sources.value.map((source) => {
      if (source.kind !== kind) {
        return source
      }
      const merged: EventSourceConfig = {
        ...source,
        ...patch,
        kind,
        fields: { ...source.fields, ...(patch.fields ?? {}) },
      }
      if (kind === 'queue') {
        return { ...merged, enabled: false, status: 'coming_soon' }
      }
      return { ...merged, status: sourceStatusFor(merged) }
    })
    persist()
  }

  function toggleSource(kind: EventSourceKind): void {
    if (kind === 'queue') {
      return
    }
    const source = sources.value.find((item) => item.kind === kind)
    if (!source) {
      return
    }
    updateSource(kind, { enabled: !source.enabled })
  }

  function advance(key: SuperStepKey): void {
    steps.value = advanceStep(steps.value, key)
    persist()
  }

  function setBinding<K extends keyof SuperTeamBindings>(
    key: K,
    value: SuperTeamBindings[K],
  ): void {
    bindings.value = { ...bindings.value, [key]: value }
    const stepKey: SuperStepKey =
      key === 'requirementsProjectId'
        ? 'requirements'
        : key === 'developmentProjectId'
          ? 'development'
          : 'operations'
    steps.value = steps.value.map((step) =>
      step.key === stepKey && step.status === 'ready' ? { ...step, status: 'active' } : step,
    )
    persist()
  }

  function reconcile(projects: Project[], tasks: ScheduledTask[]): void {
    const projectIds = new Set(projects.map((item) => item.id))
    const taskIds = new Set(tasks.map((item) => item.id))
    const result = reconcileSteps(steps.value, bindings.value, projectIds, taskIds)
    steps.value = result.steps
    bindings.value = result.bindings
    persist()
  }

  function completeStep(key: SuperStepKey): void {
    advance(key)
  }

  function reset(): void {
    const base = initialState()
    sources.value = base.sources
    steps.value = base.steps
    events.value = base.events
    audit.value = base.audit
    bindings.value = base.bindings
    try {
      if (typeof localStorage !== 'undefined') {
        localStorage.removeItem(SUPER_TEAM_STORAGE_KEY)
      }
    } catch {
      return
    }
  }

  return {
    sources,
    steps,
    events,
    audit,
    bindings,
    persist,
    updateSource,
    toggleSource,
    setBinding,
    reconcile,
    advance,
    completeStep,
    reset,
  }
})

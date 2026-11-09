import { defineStore } from 'pinia'
import { ref } from 'vue'

import type {
  EventSourceConfig,
  EventSourceKind,
  SuperStepKey,
  SuperTeamState,
} from '@/types/superTeam'
import {
  SUPER_TEAM_STORAGE_KEY,
  advanceStep,
  initialState,
  loadState,
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

  function reset(): void {
    const base = initialState()
    sources.value = base.sources
    steps.value = base.steps
    events.value = base.events
    audit.value = base.audit
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
    persist,
    updateSource,
    toggleSource,
    advance,
    reset,
  }
})

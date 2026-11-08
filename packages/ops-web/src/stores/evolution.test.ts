import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { EvolutionStorage } from '@/utils/evolutionState'

import { useEvolutionAdminStore } from './evolution'

let storage: EvolutionStorage

function memoryStorage(): EvolutionStorage {
  const data = new Map<string, string>()
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => {
      data.set(key, value)
    },
  }
}

function setup() {
  const store = useEvolutionAdminStore()
  store.init(storage)
  return store
}

beforeEach(() => {
  setActivePinia(createPinia())
  storage = memoryStorage()
})

describe('ops evolution store', () => {
  it('seeds all user versions and a selected version', () => {
    const store = setup()

    expect(store.versionGroups.length).toBeGreaterThan(1)
    expect(store.selectedVersion?.id).toBe('v-1001-2')
    expect(store.tasks).toHaveLength(1)
  })

  it('appends an admin reply to the selected user chat', () => {
    const store = setup()
    const before = store.activeChat.length

    expect(store.sendReply('这个方向可以，补充一下验收标准。')).toBe(true)

    expect(store.activeChat.length).toBe(before + 1)
    expect(store.activeChat[store.activeChat.length - 1]?.role).toBe('admin')
  })

  it('submits a version to the coding queue only once', () => {
    const store = setup()
    const before = store.tasks.length

    expect(store.submitTask()).toBe(true)
    expect(store.tasks).toHaveLength(before + 1)
    expect(store.selectedVersion?.status).toBe('submitted')
    expect(store.selectedTask).not.toBeNull()

    expect(store.submitTask()).toBe(false)
    expect(store.tasks).toHaveLength(before + 1)
  })

  it('does not submit the official baseline', () => {
    const store = setup()
    const official = store.versions.find((version) => version.userId === 'official')
    expect(official).toBeDefined()
    store.selectVersion(official?.id ?? '')

    expect(store.canSubmit).toBe(false)
    expect(store.submitTask()).toBe(false)
  })

  it('advances a task status and releases the version', () => {
    const store = setup()
    store.submitTask()
    const task = store.selectedTask
    expect(task).not.toBeNull()

    store.advanceTask(task?.id ?? '')
    expect(store.tasks.find((item) => item.id === task?.id)?.status).toBe('coding')
  })
})

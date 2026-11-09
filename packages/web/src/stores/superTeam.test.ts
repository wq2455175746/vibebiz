import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SUPER_TEAM_STORAGE_KEY } from '@/utils/superTeam'
import { useSuperTeamStore } from './superTeam'

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

let storage: ReturnType<typeof memoryStorage>

beforeEach(() => {
  storage = memoryStorage()
  vi.stubGlobal('localStorage', storage)
  setActivePinia(createPinia())
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('super team store', () => {
  it('loads defaults', () => {
    const store = useSuperTeamStore()
    expect(store.sources).toHaveLength(4)
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
  })

  it('advances steps and persists', () => {
    const store = useSuperTeamStore()
    store.advance('requirements')
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('done')
    expect(store.steps.find((step) => step.key === 'development')?.status).toBe('ready')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toContain('"done"')
  })

  it('does not advance locked steps', () => {
    const store = useSuperTeamStore()
    store.advance('operations')
    expect(store.steps.find((step) => step.key === 'operations')?.status).toBe('locked')
  })

  it('updates source fields and recomputes status', () => {
    const store = useSuperTeamStore()
    store.updateSource('email', {
      enabled: true,
      fields: {
        host: 'imap.test',
        port: '993',
        user: 'a@b.c',
        password: 'x',
        intervalMinutes: '5',
      },
    })
    const email = store.sources.find((source) => source.kind === 'email')
    expect(email?.status).toBe('connected')
    expect(email?.fields.host).toBe('imap.test')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toContain('imap.test')
  })

  it('turns off a configured source', () => {
    const store = useSuperTeamStore()
    store.updateSource('webhook', { enabled: false })
    const webhook = store.sources.find((source) => source.kind === 'webhook')
    expect(webhook?.status).toBe('configured')
  })

  it('never enables the queue source', () => {
    const store = useSuperTeamStore()
    store.toggleSource('queue')
    store.updateSource('queue', { enabled: true })
    const queue = store.sources.find((source) => source.kind === 'queue')
    expect(queue?.enabled).toBe(false)
    expect(queue?.status).toBe('coming_soon')
  })

  it('resets to defaults', () => {
    const store = useSuperTeamStore()
    store.advance('requirements')
    store.reset()
    expect(store.steps.find((step) => step.key === 'requirements')?.status).toBe('active')
    expect(localStorage.getItem(SUPER_TEAM_STORAGE_KEY)).toBeNull()
  })
})

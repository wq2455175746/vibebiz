import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchChatLogs } from '@/api/chat'
import { useLogStore } from './logs'

vi.mock('@/api/chat', () => ({
  fetchChatLogs: vi.fn(),
}))

const fetchLogsMock = vi.mocked(fetchChatLogs)

beforeEach(() => {
  setActivePinia(createPinia())
  fetchLogsMock.mockReset()
})

describe('log store project logs', () => {
  it('loads the latest project logs into the console', async () => {
    fetchLogsMock.mockResolvedValue([
      {
        ts: '2026-10-30T01:00:00.000Z',
        level: 'info',
        source: 'agent',
        message: 'agent.run.start',
      },
      {
        ts: '2026-10-30T01:00:01.000Z',
        level: 'info',
        source: 'tool',
        message: 'Read index.html',
      },
    ])
    const logs = useLogStore()
    logs.append({ level: 'error', source: 'agent', message: 'stale' })

    await logs.load('p1')

    expect(fetchLogsMock).toHaveBeenCalledWith('p1', 100)
    expect(logs.entries.map((entry) => entry.message)).toEqual([
      'agent.run.start',
      'Read index.html',
    ])
    expect(logs.entries.map((entry) => entry.seq)).toEqual([1, 2])
  })

  it('replaces local entries with the server snapshot', async () => {
    fetchLogsMock.mockResolvedValue([
      {
        ts: '2026-10-30T01:00:00.000Z',
        level: 'warn',
        source: 'agent',
        message: 'from server',
      },
    ])
    const logs = useLogStore()
    logs.append({ level: 'info', source: 'agent', message: 'local only' })

    await logs.load('p1')

    expect(logs.entries.map((entry) => entry.message)).toEqual(['from server'])
  })
})

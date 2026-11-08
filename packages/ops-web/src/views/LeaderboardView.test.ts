import { createApp, type App } from 'vue'
import PrimeVue from 'primevue/config'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from '@/api/admin'
import LeaderboardView from './LeaderboardView.vue'

vi.mock('@/api/admin', () => ({
  getLeaderboard: vi.fn(),
  updateLeaderboardHidden: vi.fn(),
  deleteLeaderboardEntry: vi.fn(),
}))

const getLeaderboardMock = vi.mocked(getLeaderboard)
const hideMock = vi.mocked(updateLeaderboardHidden)
const deleteMock = vi.mocked(deleteLeaderboardEntry)

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

beforeEach(() => {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
  getLeaderboardMock.mockResolvedValue({
    items: [
      {
        rank: 1,
        projectId: 'p1',
        name: '小游戏',
        author: 'alice',
        visits: 12,
        exposure: 24,
        clicks: 3,
        hidden: false,
        publishedAt: '2026-11-07T10:00:00Z',
      },
    ],
    total: 1,
    page: 1,
    pageSize: 10,
  })
  hideMock.mockResolvedValue(undefined)
  deleteMock.mockResolvedValue(undefined)
  vi.spyOn(window, 'confirm').mockReturnValue(true)
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(LeaderboardView)
  app.use(PrimeVue)
  app.mount(host)
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  vi.restoreAllMocks()
})

describe('LeaderboardView', () => {
  it('renders leaderboard rows', async () => {
    await flush()
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('12')
  })

  it('hides a row', async () => {
    await flush()
    const button = Array.from(host.querySelectorAll('button')).find((item) =>
      item.textContent?.includes('隐藏'),
    )
    expect(button).toBeTruthy()
    button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    expect(hideMock).toHaveBeenCalledWith('p1', true)
  })

  it('deletes a row after confirmation', async () => {
    await flush()
    const button = Array.from(host.querySelectorAll('button')).find((item) =>
      item.textContent?.includes('删除'),
    )
    expect(button).toBeTruthy()
    button?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    await flush()
    expect(deleteMock).toHaveBeenCalledWith('p1')
  })
})

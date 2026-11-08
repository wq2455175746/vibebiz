import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { getLeaderboard } from '@/api/leaderboard'
import LeaderboardPanel from './LeaderboardPanel.vue'

vi.mock('@/api/leaderboard', () => ({ getLeaderboard: vi.fn() }))

const getLeaderboardMock = vi.mocked(getLeaderboard)
let app: App | null = null
let host: HTMLElement

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(LeaderboardPanel)
  app.mount(host)
}

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('LeaderboardPanel', () => {
  it('renders ranked cards', async () => {
    getLeaderboardMock.mockResolvedValue([
      {
        rank: 1,
        projectId: 'p1',
        name: '小游戏',
        author: 'alice',
        visits: 12,
        coverUrl: 'https://cdn.example.com/p1.png',
        url: 'https://play.example.com/p/p1/',
      },
      {
        rank: 2,
        projectId: 'p2',
        name: '答题',
        author: 'bob',
        visits: 8,
        coverUrl: '',
        url: 'https://play.example.com/p/p2/',
      },
    ])
    mount()
    await flush()
    expect(host.querySelectorAll('.leaderboard-card')).toHaveLength(2)
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('12')
    expect(host.querySelector('img')?.getAttribute('src')).toBe('https://cdn.example.com/p1.png')
    expect(host.textContent).toContain('暂无预览图')
  })

  it('shows the empty state', async () => {
    getLeaderboardMock.mockResolvedValue([])
    mount()
    await flush()
    expect(host.textContent).toContain('暂无排行榜数据')
  })

  it('shows the error state', async () => {
    getLeaderboardMock.mockRejectedValue(new Error('network down'))
    mount()
    await flush()
    expect(host.textContent).toContain('network down')
  })
})

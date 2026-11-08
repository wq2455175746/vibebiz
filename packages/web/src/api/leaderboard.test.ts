import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { getLeaderboard } from './leaderboard'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
})

describe('leaderboard api', () => {
  it('unwraps leaderboard items', async () => {
    requestMock.mockResolvedValue({
      items: [
        {
          rank: 1,
          projectId: 'p1',
          name: '小游戏',
          author: 'alice',
          visits: 12,
          coverUrl: 'https://cdn.example.com/p1.png',
          url: 'https://play.example.com/p/p1/',
        },
      ],
    })
    await expect(getLeaderboard()).resolves.toEqual([
      expect.objectContaining({ projectId: 'p1', visits: 12 }),
    ])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/leaderboard',
      params: { limit: 50 },
    })
  })
})

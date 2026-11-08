import { describe, expect, it, vi } from 'vitest'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from './admin'
import { http } from './client'

describe('leaderboard admin api', () => {
  it('loads a page of leaderboard rows', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: {
        code: 0,
        message: 'ok',
        result: {
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
        },
      },
    })
    const page = await getLeaderboard({ page: 1, pageSize: 10, keyword: '小' })
    expect(page.items).toHaveLength(1)
    expect(page.total).toBe(1)
    expect(spy).toHaveBeenCalledWith({
      method: 'get',
      url: '/leaderboard',
      params: { page: 1, pageSize: 10, keyword: '小' },
    })
  })

  it('hides and deletes leaderboard entries', async () => {
    const hideSpy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { projectId: 'p1', hidden: true } },
    })
    await updateLeaderboardHidden('p1', true)
    expect(hideSpy).toHaveBeenCalledWith({
      method: 'put',
      url: '/leaderboard/p1/hidden',
      data: { hidden: true },
    })

    const deleteSpy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { projectId: 'p1' } },
    })
    await deleteLeaderboardEntry('p1')
    expect(deleteSpy).toHaveBeenCalledWith({
      method: 'delete',
      url: '/leaderboard/p1',
    })
  })
})

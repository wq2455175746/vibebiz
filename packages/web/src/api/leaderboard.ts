import { cpHttp, request } from './client'
import type { LeaderboardEntry } from '@/types'

export async function getLeaderboard(limit = 50): Promise<LeaderboardEntry[]> {
  const result = await request<{ items: LeaderboardEntry[] }>(cpHttp, {
    method: 'get',
    url: '/leaderboard',
    params: { limit },
  })
  return result.items ?? []
}

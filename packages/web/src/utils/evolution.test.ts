import { describe, expect, it } from 'vitest'

import type { EvolutionVersion } from '@/types/evolution'
import {
  OFFICIAL_USER_ID,
  buildEvolutionPreviewUrl,
  formatRelative,
  groupMessagesByDay,
  groupVersionsByUser,
  latestVersionsByUser,
  statusLabel,
  withQuery,
} from './evolution'

function version(
  id: string,
  userId: string,
  updatedAt: string,
  overrides: Partial<EvolutionVersion> = {},
): EvolutionVersion {
  return {
    id,
    userId,
    userName: userId,
    label: id,
    createdAt: updatedAt,
    updatedAt,
    status: 'preview',
    scenario: 'baseline',
    summary: '',
    originVersion: 'v1.2026.11',
    proposalIds: [],
    ...overrides,
  }
}

describe('groupVersionsByUser', () => {
  const versions = [
    version('current-old', '7', '2026-11-01T00:00:00.000Z'),
    version('current-new', '7', '2026-11-07T00:00:00.000Z'),
    version('other', '9', '2026-11-08T00:00:00.000Z'),
    version('v-official-1', OFFICIAL_USER_ID, '2026-11-09T00:00:00.000Z'),
  ]

  it('pins the current user first and the official baseline last', () => {
    const groups = groupVersionsByUser(versions, '7')
    expect(groups.map((group) => group.userId)).toEqual(['7', '9', OFFICIAL_USER_ID])
    expect(groups[0].latest.id).toBe('current-new')
    expect(groups[0].versions.map((item) => item.id)).toEqual(['current-new', 'current-old'])
  })

  it('returns the latest version per user', () => {
    const latest = latestVersionsByUser(versions, '7')
    expect(latest.map((item) => item.id)).toEqual(['current-new', 'other', 'v-official-1'])
  })
})

describe('preview URL', () => {
  it('appends query parameters without losing the path', () => {
    expect(withQuery('/a/b.html', { scenario: 'soft-cards', user: '林 岚' })).toBe(
      '/a/b.html?scenario=soft-cards&user=%E6%9E%97+%E5%B2%9A',
    )
  })

  it('builds an absolute preview URL', () => {
    const url = buildEvolutionPreviewUrl(
      'http://localhost:5173/evolution/origin_version/index.html',
      version('v1', '7', '2026-11-08T00:00:00.000Z', {
        label: 'v7.2',
        userName: '林岚',
        scenario: 'compact-sidebar',
      }),
      '王海广',
    )
    expect(url).toContain('scenario=compact-sidebar')
    expect(url).toContain('version=v7.2')
    expect(url).toContain('viewer=')
  })
})

describe('formatting', () => {
  it('maps known statuses', () => {
    expect(statusLabel('draft')).toBe('草案')
    expect(statusLabel('preview')).toBe('待确认')
    expect(statusLabel('queued')).toBe('待处理')
    expect(statusLabel('unknown')).toBe('unknown')
  })

  it('formats relative times', () => {
    const now = Date.parse('2026-11-08T12:00:00.000Z')
    expect(formatRelative('2026-11-08T11:59:30.000Z', now)).toBe('刚刚')
    expect(formatRelative('2026-11-08T11:30:00.000Z', now)).toBe('30 分钟前')
  })

  it('groups chat messages by day', () => {
    const groups = groupMessagesByDay([
      { id: 'a', createdAt: '2026-11-07T10:00:00Z', role: 'user', userName: 'u', content: 'a' },
      { id: 'b', createdAt: '2026-11-08T10:00:00Z', role: 'agent', userName: 'a', content: 'b' },
      { id: 'c', createdAt: '2026-11-08T11:00:00Z', role: 'user', userName: 'u', content: 'c' },
    ])
    expect(groups).toHaveLength(2)
    expect(groups[0].messages).toHaveLength(2)
  })
})

import { describe, expect, it } from 'vitest'

import type { EvolutionVersion } from '@/types/evolution'
import {
  OFFICIAL_USER_ID,
  OFFICIAL_VERSION_ID,
  buildEvolutionPreviewUrl,
  createTask,
  groupVersionsByUser,
  nextTaskStatus,
  statusLabel,
} from './evolution'

function version(id: string, userId: string, updatedAt: string): EvolutionVersion {
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
    proposalIds: [],
  }
}

describe('ops evolution utils', () => {
  it('orders user groups by recency with the official baseline last', () => {
    const groups = groupVersionsByUser(
      [
        version('a', '1', '2026-11-01T00:00:00.000Z'),
        version('b', '2', '2026-11-08T00:00:00.000Z'),
        version(OFFICIAL_VERSION_ID, OFFICIAL_USER_ID, '2026-11-09T00:00:00.000Z'),
      ],
      '',
    )
    expect(groups.map((group) => group.userId)).toEqual(['2', '1', OFFICIAL_USER_ID])
  })

  it('creates a queued coding task from a version', () => {
    const task = createTask(
      version('v1', '7', '2026-11-08T00:00:00.000Z'),
      'v1.2026.11',
      '用户确认了紧凑布局',
      1_700_000_000_000,
    )
    expect(task.status).toBe('queued')
    expect(task.originVersion).toBe('v1.2026.11')
    expect(task.chatSummary).toContain('紧凑布局')
  })

  it('advances task status up to released', () => {
    expect(nextTaskStatus('queued')).toBe('coding')
    expect(nextTaskStatus('coding')).toBe('ci')
    expect(nextTaskStatus('ci')).toBe('released')
    expect(nextTaskStatus('released')).toBe('released')
  })

  it('builds the preview URL with scenario and version', () => {
    const url = buildEvolutionPreviewUrl(
      'http://localhost:5173/evolution/origin_version/index.html',
      version('v1', '7', '2026-11-08T00:00:00.000Z'),
      '运营预览',
    )
    expect(url).toContain('scenario=baseline')
    expect(url).toContain('version=v1')
    expect(url).toContain('viewer=')
  })

  it('maps coding task statuses', () => {
    expect(statusLabel('coding')).toBe('编码中')
    expect(statusLabel('ci')).toBe('CI 中')
  })
})

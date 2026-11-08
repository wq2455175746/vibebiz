import { describe, expect, it } from 'vitest'

import type { EvolutionVersion } from '@/types/evolution'
import {
  OFFICIAL_USER_ID,
  evolutionProjectID,
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
    originVersion: 'v1.2026.11',
    proposalIds: [],
  }
}

describe('ops evolution utils', () => {
  it('orders user groups by recency with the official baseline last', () => {
    const groups = groupVersionsByUser(
      [
        version('a', '1', '2026-11-01T00:00:00.000Z'),
        version('b', '2', '2026-11-08T00:00:00.000Z'),
        version('v-official-1', OFFICIAL_USER_ID, '2026-11-09T00:00:00.000Z'),
      ],
      '',
    )
    expect(groups.map((group) => group.userId)).toEqual(['2', '1', OFFICIAL_USER_ID])
  })

  it('derives the hidden sandbox project id from a user id', () => {
    expect(evolutionProjectID('7')).toBe('evolve-7')
  })

  it('advances task status up to released', () => {
    expect(nextTaskStatus('queued')).toBe('coding')
    expect(nextTaskStatus('coding')).toBe('ci')
    expect(nextTaskStatus('ci')).toBe('released')
    expect(nextTaskStatus('released')).toBe('released')
  })

  it('maps coding task statuses', () => {
    expect(statusLabel('coding')).toBe('编码中')
    expect(statusLabel('ci')).toBe('CI 中')
    expect(statusLabel('preview')).toBe('待确认')
  })
})

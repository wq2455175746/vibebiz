import { describe, expect, it } from 'vitest'

import {
  buildIPStatsOption,
  buildProjectsOption,
  dateRange,
  dateRangeForDays,
  fillUsageSeries,
} from './usage'

describe('dateRange', () => {
  it('returns inclusive dates', () => {
    expect(dateRange(new Date('2026-11-04T00:00:00Z'), new Date('2026-11-06T00:00:00Z'))).toEqual([
      '2026-11-04',
      '2026-11-05',
      '2026-11-06',
    ])
  })

  it('builds a trailing range', () => {
    expect(dateRangeForDays(3, new Date('2026-11-06T12:00:00Z'))).toEqual([
      '2026-11-04',
      '2026-11-05',
      '2026-11-06',
    ])
  })
})

describe('fillUsageSeries', () => {
  it('fills missing dates with zeros and merges duplicates', () => {
    const rows = [
      {
        date: '2026-11-05',
        projectsCreated: 2,
        inputTokens: 10,
        outputTokens: 5,
        modelRequests: 1,
        uploadCount: 1,
        uploadBytes: 100,
      },
      {
        date: '2026-11-05',
        projectsCreated: 1,
        inputTokens: 4,
        outputTokens: 2,
        modelRequests: 1,
        uploadCount: 2,
        uploadBytes: 200,
      },
    ]
    const filled = fillUsageSeries(rows, ['2026-11-04', '2026-11-05'])
    expect(filled.map((item) => item.date)).toEqual(['2026-11-04', '2026-11-05'])
    expect(filled[0].projectsCreated).toBe(0)
    expect(filled[1].projectsCreated).toBe(3)
    expect(filled[1].uploadBytes).toBe(300)
  })
})

describe('buildIPStatsOption', () => {
  it('uses ip categories and request values', () => {
    const option = buildIPStatsOption([
      { ip: '1.2.3.4', requests: 9 },
      { ip: '5.6.7.8', requests: 3 },
    ])
    const yAxis = option.yAxis as { data: string[]; inverse: boolean }
    const series = option.series as Array<{ data: number[] }>
    expect(yAxis.data).toEqual(['1.2.3.4', '5.6.7.8'])
    expect(yAxis.inverse).toBe(true)
    expect(series[0].data).toEqual([9, 3])
  })
})

describe('buildProjectsOption', () => {
  it('uses the projects series', () => {
    const option = buildProjectsOption([
      {
        date: '2026-11-06',
        projectsCreated: 2,
        inputTokens: 0,
        outputTokens: 0,
        modelRequests: 0,
        uploadCount: 0,
        uploadBytes: 0,
      },
    ])
    const series = option.series as Array<{ data: number[] }>
    expect(series[0].data).toEqual([2])
  })
})

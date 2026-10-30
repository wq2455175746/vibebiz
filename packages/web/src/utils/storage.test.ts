import { describe, expect, it } from 'vitest'

import { formatBytes, formatDate, formatDateTime, storageRatio } from './storage'

describe('formatBytes', () => {
  it('renders zero and sub-kilobyte sizes', () => {
    expect(formatBytes(0)).toBe('0 B')
    expect(formatBytes(512)).toBe('512 B')
  })

  it('renders readable units with one decimal place', () => {
    expect(formatBytes(1024)).toBe('1 KB')
    expect(formatBytes(1536)).toBe('1.5 KB')
    expect(formatBytes(10 * 1024 * 1024 * 1024)).toBe('10 GB')
  })

  it('treats negative values as zero', () => {
    expect(formatBytes(-1)).toBe('0 B')
  })
})

describe('formatDateTime', () => {
  it('renders a dash for missing or invalid timestamps', () => {
    expect(formatDateTime(null)).toBe('—')
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime('not-a-date')).toBe('—')
  })

  it('renders a valid timestamp', () => {
    expect(formatDateTime('2026-10-28T12:00:00Z')).toContain('2026')
  })
})

describe('formatDate', () => {
  it('renders a date without the time', () => {
    expect(formatDate('2026-10-30T12:00:00Z')).toContain('2026')
    expect(formatDate(null)).toBe('—')
  })
})

describe('storageRatio', () => {
  it('returns the used fraction of the quota', () => {
    expect(storageRatio(5, 10)).toBe(0.5)
  })

  it('clamps invalid or overflowing values', () => {
    expect(storageRatio(5, 0)).toBe(0)
    expect(storageRatio(15, 10)).toBe(1)
    expect(storageRatio(-1, 10)).toBe(0)
  })
})

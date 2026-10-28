import { describe, expect, it } from 'vitest'

import { formatDateTime, formatNumber } from './format'

describe('formatDateTime', () => {
  it('returns a dash for missing values', () => {
    expect(formatDateTime(undefined)).toBe('—')
    expect(formatDateTime('')).toBe('—')
  })

  it('keeps unparseable values unchanged', () => {
    expect(formatDateTime('not-a-date')).toBe('not-a-date')
  })
})

describe('formatNumber', () => {
  it('formats finite numbers', () => {
    expect(formatNumber(12000)).toBe('12,000')
  })

  it('returns a dash for missing values', () => {
    expect(formatNumber(null)).toBe('—')
    expect(formatNumber('')).toBe('—')
  })
})

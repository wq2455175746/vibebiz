import { describe, expect, it } from 'vitest'

import { COVER_HEIGHT, COVER_WIDTH, coverDrawRect, validateCoverFile } from './cover'

describe('coverDrawRect', () => {
  it('crops a square source by height to fill 16:9', () => {
    const rect = coverDrawRect(1000, 1000)
    expect(rect.dw).toBe(1200)
    expect(rect.dh).toBe(1200)
    expect(rect.dx).toBe(0)
    expect(rect.dy).toBe((COVER_HEIGHT - 1200) / 2)
  })

  it('crops a wide source by width to fill 16:9', () => {
    const rect = coverDrawRect(2000, 1000)
    expect(rect.dw).toBe(1350)
    expect(rect.dh).toBe(675)
    expect(rect.dx).toBe((COVER_WIDTH - 1350) / 2)
    expect(rect.dy).toBe(0)
  })

  it('keeps a source that already matches the cover ratio', () => {
    expect(coverDrawRect(1200, 675)).toEqual({ dx: 0, dy: 0, dw: 1200, dh: 675 })
  })

  it('rejects non-positive dimensions', () => {
    expect(() => coverDrawRect(0, 100)).toThrow()
  })
})

describe('validateCoverFile', () => {
  it('accepts image files within the size limit', () => {
    expect(validateCoverFile({ type: 'image/jpeg', size: 1024 })).toBe('')
  })

  it('rejects non-image files', () => {
    expect(validateCoverFile({ type: 'application/pdf', size: 1024 })).not.toBe('')
  })

  it('rejects oversized files', () => {
    expect(validateCoverFile({ type: 'image/png', size: 21 * 1024 * 1024 })).not.toBe('')
  })
})

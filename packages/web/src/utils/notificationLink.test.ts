import { describe, expect, it } from 'vitest'

import { safeNotificationLink } from './notificationLink'

describe('safeNotificationLink', () => {
  it('allows http and https links', () => {
    expect(safeNotificationLink('https://example.com/release')).toBe(
      'https://example.com/release',
    )
    expect(safeNotificationLink('http://example.com')).toBe('http://example.com')
  })

  it('allows root-relative links', () => {
    expect(safeNotificationLink('/docs/release')).toBe('/docs/release')
    expect(safeNotificationLink('  /docs/release  ')).toBe('/docs/release')
  })

  it('rejects javascript and data urls', () => {
    expect(safeNotificationLink('javascript:alert(1)')).toBe('')
    expect(safeNotificationLink('JavaScript:alert(1)')).toBe('')
    expect(safeNotificationLink('data:text/html,<script>alert(1)</script>')).toBe('')
  })

  it('rejects protocol-relative and empty links', () => {
    expect(safeNotificationLink('//evil.example.com')).toBe('')
    expect(safeNotificationLink('')).toBe('')
    expect(safeNotificationLink('   ')).toBe('')
  })
})

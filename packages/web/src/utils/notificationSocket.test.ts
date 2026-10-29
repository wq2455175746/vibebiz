import { describe, expect, it } from 'vitest'

import { buildNotificationSocketUrl } from './notificationSocket'

describe('buildNotificationSocketUrl', () => {
  it('builds ws url from relative base', () => {
    const location = { protocol: 'https:', host: 'vibebiz-dev.com:5173' } as Location
    expect(buildNotificationSocketUrl('/cp/api/v1', location)).toBe(
      'wss://vibebiz-dev.com:5173/cp/api/v1/notifications/ws',
    )
  })

  it('builds ws url from absolute base', () => {
    const location = { protocol: 'http:', host: 'localhost:5173' } as Location
    expect(buildNotificationSocketUrl('http://cp:8080/cp/api/v1', location)).toBe(
      'ws://cp:8080/cp/api/v1/notifications/ws',
    )
  })

  it('normalizes a trailing slash in the base', () => {
    const location = { protocol: 'http:', host: 'localhost:5173' } as Location
    expect(buildNotificationSocketUrl('/cp/api/v1/', location)).toBe(
      'ws://localhost:5173/cp/api/v1/notifications/ws',
    )
  })
})

import { describe, expect, it } from 'vitest'

import {
  canEdit,
  canPublish,
  canUnpublish,
  isExpired,
  notificationDateBounds,
  notificationStatusLabel,
  notificationTypeLabel,
  resolveExpireAt,
} from './notificationStatus'

describe('notificationStatusLabel', () => {
  it('labels draft, published, unpublished and expired', () => {
    expect(notificationStatusLabel('draft')).toBe('草稿')
    expect(notificationStatusLabel('published')).toBe('已发布')
    expect(notificationStatusLabel('unpublished')).toBe('已取消发布')
    expect(notificationStatusLabel('expired')).toBe('已过期')
  })
})

describe('notificationTypeLabel', () => {
  it('labels version release and inbox', () => {
    expect(notificationTypeLabel('version_release')).toBe('版本发布')
    expect(notificationTypeLabel('inbox')).toBe('站内信')
  })
})

describe('isExpired', () => {
  const now = new Date('2026-10-29T10:00:00.000Z')

  it('treats the derived expired status as expired', () => {
    expect(isExpired({ status: 'expired' }, now)).toBe(true)
    expect(isExpired({ status: 'published' }, now)).toBe(false)
  })

  it('treats a past expireAt as expired for any status', () => {
    expect(isExpired({ status: 'draft', expireAt: '2026-10-29T09:59:59.000Z' }, now)).toBe(true)
    expect(isExpired({ status: 'draft', expireAt: '2026-10-29T10:00:01.000Z' }, now)).toBe(false)
  })
})

describe('canPublish', () => {
  const now = new Date('2026-10-29T10:00:00.000Z')

  it('rejects published and expired notifications', () => {
    expect(canPublish({ status: 'published' }, now)).toBe(false)
    expect(canPublish({ status: 'expired' }, now)).toBe(false)
  })

  it('rejects a past expireAt regardless of status', () => {
    expect(canPublish({ status: 'draft', expireAt: '2026-10-29T09:59:59.000Z' }, now)).toBe(false)
    expect(canPublish({ status: 'unpublished', expireAt: '2026-10-29T09:00:00.000Z' }, now)).toBe(
      false,
    )
  })

  it('allows draft and unpublished notifications with a future expireAt', () => {
    expect(canPublish({ status: 'draft', expireAt: '2026-10-30T00:00:00.000Z' }, now)).toBe(true)
    expect(canPublish({ status: 'unpublished', expireAt: '2026-10-30T00:00:00.000Z' }, now)).toBe(
      true,
    )
  })
})

describe('canUnpublish', () => {
  it('allows published and expired notifications', () => {
    expect(canUnpublish({ status: 'published' })).toBe(true)
    expect(canUnpublish({ status: 'expired' })).toBe(true)
  })

  it('rejects draft and unpublished notifications', () => {
    expect(canUnpublish({ status: 'draft' })).toBe(false)
    expect(canUnpublish({ status: 'unpublished' })).toBe(false)
  })
})

describe('canEdit', () => {
  it('allows draft and unpublished only', () => {
    expect(canEdit({ status: 'draft' } as never)).toBe(true)
    expect(canEdit({ status: 'unpublished' } as never)).toBe(true)
    expect(canEdit({ status: 'published' } as never)).toBe(false)
  })
})

describe('notificationDateBounds', () => {
  it('uses local midnight today and 30 days later', () => {
    const { minDate, maxDate } = notificationDateBounds(new Date(2026, 9, 29, 15, 30))
    expect(minDate).toEqual(new Date(2026, 9, 29))
    expect(maxDate).toEqual(new Date(2026, 10, 28))
  })
})

describe('resolveExpireAt', () => {
  it('uses the end of the selected local day', () => {
    const now = new Date(2026, 9, 29, 10, 0, 0)
    expect(resolveExpireAt(new Date(2026, 9, 30), now)).toBe(
      new Date(2026, 9, 30, 23, 59, 59).toISOString(),
    )
  })

  it('clamps the max date inside the backend retention window', () => {
    const now = new Date(2026, 9, 29, 10, 0, 0)
    const maxDate = new Date(2026, 10, 28)
    expect(resolveExpireAt(maxDate, now)).toBe(
      new Date(now.getTime() + 30 * 24 * 60 * 60 * 1000 - 60_000).toISOString(),
    )
  })
})

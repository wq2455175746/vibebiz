import { describe, expect, it } from 'vitest'

import {
  canEdit,
  isExpired,
  notificationStatusLabel,
  notificationTypeLabel,
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
  it('treats the derived expired status as expired', () => {
    expect(isExpired({ status: 'expired' })).toBe(true)
    expect(isExpired({ status: 'published' })).toBe(false)
  })
})

describe('canEdit', () => {
  it('allows draft and unpublished only', () => {
    expect(canEdit({ status: 'draft' } as never)).toBe(true)
    expect(canEdit({ status: 'unpublished' } as never)).toBe(true)
    expect(canEdit({ status: 'published' } as never)).toBe(false)
  })
})

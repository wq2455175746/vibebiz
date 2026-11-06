import { describe, expect, it } from 'vitest'

import type { NotificationItem } from '@/types'

import { newNotifications } from './newNotifications'

function notification(id: number, title: string): NotificationItem {
  return {
    id,
    title,
    content: '',
    type: 'inbox',
    version: '',
    link: '',
    publishedAt: '2026-11-06T10:00:00Z',
    expireAt: '2026-12-06T10:00:00Z',
  }
}

describe('newNotifications', () => {
  it('returns only notifications that were not present before', () => {
    const previous = [notification(1, '旧消息')]
    const current = [notification(2, '新消息'), notification(1, '旧消息')]

    expect(newNotifications(previous, current).map((item) => item.id)).toEqual([2])
  })

  it('returns all notifications when there was no previous snapshot', () => {
    expect(newNotifications([], [notification(1, '新消息')]).map((item) => item.id)).toEqual([1])
  })
})

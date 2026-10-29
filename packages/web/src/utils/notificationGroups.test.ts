import { describe, expect, it } from 'vitest'

import type { NotificationItem } from '@/types'

import { groupNotifications } from './notificationGroups'

function item(id: number, type: NotificationItem['type']): NotificationItem {
  return {
    id,
    title: `通知 ${id}`,
    content: '内容',
    type,
    version: '',
    link: '',
    publishedAt: '2026-10-29T10:00:00Z',
    expireAt: '2026-11-29T10:00:00Z',
  }
}

describe('groupNotifications', () => {
  it('splits notifications by type while keeping order', () => {
    const items = [item(3, 'version_release'), item(2, 'inbox'), item(1, 'version_release')]

    const groups = groupNotifications(items)

    expect(groups.version_release.map((entry) => entry.id)).toEqual([3, 1])
    expect(groups.inbox.map((entry) => entry.id)).toEqual([2])
  })

  it('returns empty groups for an empty list', () => {
    expect(groupNotifications([])).toEqual({ version_release: [], inbox: [] })
  })
})

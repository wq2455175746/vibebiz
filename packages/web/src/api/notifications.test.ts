import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { NotificationItem } from '@/types'

import { cpHttp, request } from './client'
import { listNotifications, markNotificationsRead } from './notifications'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

const notification: NotificationItem = {
  id: 1,
  title: '版本发布',
  content: '更新内容',
  type: 'version_release',
  version: '1.2.0',
  link: 'https://example.com',
  publishedAt: '2026-10-29T10:00:00Z',
  expireAt: '2026-11-29T10:00:00Z',
}

beforeEach(() => {
  requestMock.mockReset()
})

describe('listNotifications', () => {
  it('unwraps the notifications envelope and unread flag', async () => {
    requestMock.mockResolvedValue({ notifications: [notification], unread: true })

    await expect(listNotifications()).resolves.toEqual({
      notifications: [notification],
      unread: true,
    })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/notifications',
    })
  })

  it('falls back to an empty read list when notifications are missing', async () => {
    requestMock.mockResolvedValue({})

    await expect(listNotifications()).resolves.toEqual({ notifications: [], unread: false })
  })
})

describe('markNotificationsRead', () => {
  it('posts the newest seen publishedAt', async () => {
    requestMock.mockResolvedValue({ unread: false })

    await markNotificationsRead(notification.publishedAt)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/notifications/read',
      data: { readAt: notification.publishedAt },
    })
  })
})

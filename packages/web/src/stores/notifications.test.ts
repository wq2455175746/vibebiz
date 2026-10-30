import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as notificationsApi from '@/api/notifications'
import { ApiError } from '@/api/client'
import type { NotificationItem } from '@/types'
import type { NotificationListResult } from '@/api/notifications'

import { useNotificationsStore } from './notifications'

vi.mock('@/api/notifications', () => ({
  listNotifications: vi.fn(),
  markNotificationsRead: vi.fn(),
}))

const notification: NotificationItem = {
  id: 1,
  title: '版本发布',
  content: '更新内容',
  type: 'version_release',
  version: '1.2.0',
  link: '',
  publishedAt: '2026-10-29T10:00:00Z',
  expireAt: '2026-11-29T10:00:00Z',
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('useNotificationsStore', () => {
  it('loads notifications and clears the error', async () => {
    vi.mocked(notificationsApi.listNotifications).mockResolvedValue({
      notifications: [notification],
      unread: true,
    })
    const store = useNotificationsStore()

    await store.load()

    expect(store.items).toEqual([notification])
    expect(store.unread).toBe(true)
    expect(store.loading).toBe(false)
    expect(store.error).toBe('')
  })

  it('stores the error message when loading fails', async () => {
    vi.mocked(notificationsApi.listNotifications).mockRejectedValue(
      new ApiError('加载失败', 500, 50001),
    )
    const store = useNotificationsStore()

    await store.load()

    expect(store.items).toEqual([])
    expect(store.error).toBe('加载失败')
    expect(store.loading).toBe(false)
  })

  it('does not toggle loading on a silent reload', async () => {
    let resolveLoad: (result: NotificationListResult) => void = () => {}
    vi.mocked(notificationsApi.listNotifications).mockReturnValue(
      new Promise((resolve) => {
        resolveLoad = resolve
      }),
    )
    const store = useNotificationsStore()

    const pending = store.load(true)
    expect(store.loading).toBe(false)

    resolveLoad({ notifications: [notification], unread: false })
    await pending
    expect(store.items).toEqual([notification])
    expect(store.unread).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('keeps cached items and no error when a silent reload fails', async () => {
    vi.mocked(notificationsApi.listNotifications).mockResolvedValueOnce({
      notifications: [notification],
      unread: true,
    })
    const store = useNotificationsStore()
    await store.load()

    vi.mocked(notificationsApi.listNotifications).mockRejectedValueOnce(
      new ApiError('网络错误', 500, 50001),
    )
    await store.load(true)

    expect(store.items).toEqual([notification])
    expect(store.unread).toBe(true)
    expect(store.error).toBe('')
    expect(store.loading).toBe(false)
  })

  it('marks unread notifications as read using the newest timestamp', async () => {
    vi.mocked(notificationsApi.listNotifications).mockResolvedValue({
      notifications: [notification],
      unread: true,
    })
    vi.mocked(notificationsApi.markNotificationsRead).mockResolvedValue()
    const store = useNotificationsStore()
    await store.load()

    await store.markRead()

    expect(notificationsApi.markNotificationsRead).toHaveBeenCalledWith(notification.publishedAt)
    expect(store.unread).toBe(false)
  })

  it('clears items, unread state and error', async () => {
    vi.mocked(notificationsApi.listNotifications).mockRejectedValue(new ApiError('加载失败'))
    const store = useNotificationsStore()
    await store.load()

    store.clear()

    expect(store.items).toEqual([])
    expect(store.unread).toBe(false)
    expect(store.error).toBe('')
  })
})

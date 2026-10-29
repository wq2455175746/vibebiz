import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  createNotification,
  deleteNotification,
  listNotifications,
  publishNotification,
  unpublishNotification,
  updateNotification,
} from './notifications'
import { request } from './client'

vi.mock('./client', () => ({ request: vi.fn() }))

const mockedRequest = vi.mocked(request)

const input = {
  title: 'v1.2.0',
  content: '# 更新',
  type: 'version_release' as const,
  version: '1.2.0',
  link: 'https://example.com',
  expireAt: '2026-11-30T15:59:59.000Z',
}

const entity = {
  id: 1,
  ...input,
  status: 'draft' as const,
  publishedAt: null,
  createdBy: 7,
  createdByName: 'ops',
  createdAt: '2026-10-29T00:00:00.000Z',
  updatedAt: '2026-10-29T00:00:00.000Z',
}

beforeEach(() => {
  mockedRequest.mockReset()
})

describe('listNotifications', () => {
  it('unwraps the paginated notification list', async () => {
    mockedRequest.mockResolvedValueOnce({ notifications: [entity], total: 41, page: 2, pageSize: 20 })

    await expect(listNotifications({ page: 2, pageSize: 20 })).resolves.toEqual({
      notifications: [entity],
      total: 41,
      page: 2,
      pageSize: 20,
    })
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'get',
      url: '/notifications',
      params: { page: 2, pageSize: 20 },
    })
  })

  it('defaults to the first page', async () => {
    mockedRequest.mockResolvedValueOnce({ notifications: [], total: 0, page: 1, pageSize: 20 })

    await listNotifications()
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'get',
      url: '/notifications',
      params: { page: 1, pageSize: 20 },
    })
  })
})

describe('createNotification', () => {
  it('posts the input and returns the created entity', async () => {
    mockedRequest.mockResolvedValueOnce(entity)

    await expect(createNotification(input)).resolves.toEqual(entity)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'post',
      url: '/notifications',
      data: input,
    })
  })
})

describe('updateNotification', () => {
  it('puts the input to the id url', async () => {
    mockedRequest.mockResolvedValueOnce(entity)

    await expect(updateNotification(1, input)).resolves.toEqual(entity)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'put',
      url: '/notifications/1',
      data: input,
    })
  })
})

describe('publishNotification', () => {
  it('posts to the publish url', async () => {
    mockedRequest.mockResolvedValueOnce(entity)

    await publishNotification(1)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'post',
      url: '/notifications/1/publish',
    })
  })
})

describe('unpublishNotification', () => {
  it('posts to the unpublish url', async () => {
    mockedRequest.mockResolvedValueOnce(entity)

    await unpublishNotification(1)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'post',
      url: '/notifications/1/unpublish',
    })
  })
})

describe('deleteNotification', () => {
  it('deletes the id url', async () => {
    mockedRequest.mockResolvedValueOnce({ deleted: true })

    await deleteNotification(1)
    expect(mockedRequest).toHaveBeenCalledWith({
      method: 'delete',
      url: '/notifications/1',
    })
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import {
  confirmAvatar,
  createFolder,
  getCurrentWorkspace,
  listFolders,
  getProfile,
  inviteMember,
  listMembers,
  presignAvatar,
  switchWorkspace,
  updateCurrentWorkspace,
  updateProfile,
  uploadAvatar,
  uploadWorkspaceAvatar,
  listNotificationChannels,
  createNotificationChannel,
  updateNotificationChannel,
  deleteNotificationChannel,
  testNotificationChannel,
} from './settings'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({})
})

describe('settings api', () => {
  it('switches workspace with a PUT to the auth workspace endpoint', async () => {
    await switchWorkspace(3)

    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'put',
      url: '/auth/workspace',
      data: { workspaceId: 3 },
    })
  })

  it('reads and updates the profile', async () => {
    await getProfile()
    expect(requestMock).toHaveBeenCalledWith(cpHttp, { method: 'get', url: '/me/profile' })

    await updateProfile('new-name')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'put',
      url: '/me/profile',
      data: { username: 'new-name' },
    })
  })

  it('presigns and confirms the avatar', async () => {
    await presignAvatar({ filename: 'a.png', contentType: 'image/png', size: 12 })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/me/avatar/presign',
      data: { filename: 'a.png', contentType: 'image/png', size: 12 },
    })

    await confirmAvatar('avatar/1/a.png')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'put',
      url: '/me/avatar',
      data: { key: 'avatar/1/a.png' },
    })
  })

  it('uploads avatars as raw request bodies for the CORS fallback', async () => {
    const file = new File(['avatar'], 'a.png', { type: 'image/png' })

    await uploadAvatar(file)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/me/avatar/upload',
      data: file,
      headers: { 'Content-Type': 'image/png' },
      timeout: 60_000,
    })

    await uploadWorkspaceAvatar(file)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/workspaces/current/avatar/upload',
      data: file,
      headers: { 'Content-Type': 'image/png' },
      timeout: 60_000,
    })
  })

  it('reads and updates the current workspace', async () => {
    await getCurrentWorkspace()
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/workspaces/current',
    })

    await updateCurrentWorkspace('团队', '说明')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'put',
      url: '/workspaces/current',
      data: { name: '团队', description: '说明' },
    })
  })

  it('lists and creates workspace folders', async () => {
    requestMock.mockResolvedValueOnce({ folders: [] })
    await expect(listFolders()).resolves.toEqual([])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/workspaces/current/folders',
    })

    await createFolder('设计稿')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/workspaces/current/folders',
      data: { name: '设计稿', parentId: 0 },
    })
  })

  it('lists and invites members', async () => {
    requestMock.mockResolvedValueOnce({ members: [] })
    await expect(listMembers()).resolves.toEqual([])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/workspaces/current/members',
    })

    await inviteMember('a@example.com')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/workspaces/current/members',
      data: { email: 'a@example.com' },
    })
  })
})

describe('notification channel api', () => {
  it('uses the personal channel endpoints', async () => {
    requestMock.mockResolvedValueOnce({ channels: [] })
    await listNotificationChannels()
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/me/notification-channels',
    })

    const input = {
      name: '研发群',
      type: 'webhook' as const,
      enabled: true,
      config: { url: 'https://example.com/hook' },
    }
    await createNotificationChannel(input)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/me/notification-channels',
      data: input,
    })

    await updateNotificationChannel(3, input)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'put',
      url: '/me/notification-channels/3',
      data: input,
    })

    await deleteNotificationChannel(3)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'delete',
      url: '/me/notification-channels/3',
    })

    await testNotificationChannel(3)
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/me/notification-channels/3/test',
    })
  })
})

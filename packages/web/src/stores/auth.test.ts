import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as authApi from '@/api/auth'
import { ApiError } from '@/api/client'
import type { AuthUser } from '@/types'

import { useAuthStore } from './auth'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
  switchWorkspace: vi.fn(),
}))

const user: AuthUser = {
  id: 1,
  username: 'wanghg11',
  email: 'wanghg11@example.com',
  avatarUrl: null,
  currentWorkspaceId: 1,
  workspaces: [
    { id: 1, name: 'wanghg11 的个人工作区', isOwner: true, role: 'owner' },
    { id: 2, name: '共享工作区', isOwner: false, role: 'editor' },
  ],
}

beforeEach(() => {
  setActivePinia(createPinia())
  vi.clearAllMocks()
})

describe('useAuthStore', () => {
  it('sets the user on a successful login', async () => {
    vi.mocked(authApi.login).mockResolvedValue(user)
    const store = useAuthStore()

    await expect(store.login('wanghg11', 'secret')).resolves.toEqual(user)

    expect(store.user).toEqual(user)
    expect(store.isAuthenticated).toBe(true)
  })

  it('leaves the user null and rethrows when login fails', async () => {
    vi.mocked(authApi.login).mockRejectedValue(new ApiError('invalid credentials', 401, 40102))
    const store = useAuthStore()

    await expect(store.login('wanghg11', 'wrong')).rejects.toThrow('invalid credentials')

    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
  })

  it('calls me once and caches ready', async () => {
    vi.mocked(authApi.me).mockResolvedValue(user)
    const store = useAuthStore()

    await store.ensure()
    await store.ensure()

    expect(authApi.me).toHaveBeenCalledTimes(1)
    expect(store.user).toEqual(user)
    expect(store.ready).toBe(true)
  })

  it('clears the user and marks ready on a 401 from me', async () => {
    vi.mocked(authApi.me).mockRejectedValue(new ApiError('unauthorized', 401, 40101))
    const store = useAuthStore()

    await expect(store.ensure()).resolves.toBeUndefined()

    expect(store.user).toBeNull()
    expect(store.ready).toBe(true)
  })

  it('clears the user and still calls logout when the API fails', async () => {
    vi.mocked(authApi.logout).mockRejectedValue(new Error('network down'))
    const store = useAuthStore()
    store.user = user

    await expect(store.logout()).resolves.toBeUndefined()

    expect(authApi.logout).toHaveBeenCalledTimes(1)
    expect(store.user).toBeNull()
    expect(store.ready).toBe(true)
  })

  it('switchWorkspace stores the refreshed me', async () => {
    const switched = { ...user, currentWorkspaceId: 2 }
    vi.mocked(authApi.switchWorkspace).mockResolvedValue(switched)
    const store = useAuthStore()
    store.user = user

    await expect(store.switchWorkspace(2)).resolves.toEqual(switched)

    expect(authApi.switchWorkspace).toHaveBeenCalledWith(2)
    expect(store.user).toEqual(switched)
  })

  it('refresh replaces the user without resetting ready', async () => {
    const updated = { ...user, username: 'renamed', avatarUrl: 'https://cdn/avatar.png' }
    vi.mocked(authApi.me).mockResolvedValue(updated)
    const store = useAuthStore()
    store.user = user
    store.ready = true

    await expect(store.refresh()).resolves.toEqual(updated)

    expect(store.user).toEqual(updated)
    expect(store.ready).toBe(true)
  })
})

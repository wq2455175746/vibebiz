import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError } from '@/api/client'
import type { OpsUser } from '@/types'

vi.mock('@/api/auth', () => ({
  login: vi.fn(),
  me: vi.fn(),
  logout: vi.fn(),
}))

import { login as loginApi, logout as logoutApi, me as meApi } from '@/api/auth'

import { useOpsAuthStore } from './auth'

const mockedLogin = vi.mocked(loginApi)
const mockedMe = vi.mocked(meApi)
const mockedLogout = vi.mocked(logoutApi)

const admin: OpsUser = {
  id: 1,
  username: 'admin',
  email: 'admin_vibebiz@qq.com',
  roleCode: 'super_admin',
}

describe('ops auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('sets the user after a successful login', async () => {
    mockedLogin.mockResolvedValue(admin)

    const store = useOpsAuthStore()
    await store.login('admin', 'secret')

    expect(mockedLogin).toHaveBeenCalledWith('admin', 'secret')
    expect(store.user).toEqual(admin)
    expect(store.isAuthenticated).toBe(true)
    expect(store.ready).toBe(true)
    expect(store.loading).toBe(false)
  })

  it('leaves the user empty and rethrows when login fails', async () => {
    mockedLogin.mockRejectedValue(new ApiError('invalid credentials', 401, 40102))

    const store = useOpsAuthStore()

    await expect(store.login('admin', 'wrong')).rejects.toBeInstanceOf(ApiError)
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('loads the session once and caches readiness', async () => {
    mockedMe.mockResolvedValue(admin)

    const store = useOpsAuthStore()
    await store.ensure()
    await store.ensure()

    expect(mockedMe).toHaveBeenCalledTimes(1)
    expect(store.user).toEqual(admin)
    expect(store.ready).toBe(true)
  })

  it('marks readiness without a user when the session is unauthorized', async () => {
    mockedMe.mockRejectedValue(new ApiError('unauthorized', 401, 40101))

    const store = useOpsAuthStore()
    await store.ensure()
    await store.ensure()

    expect(mockedMe).toHaveBeenCalledTimes(1)
    expect(store.user).toBeNull()
    expect(store.ready).toBe(true)
  })

  it('clears the user and still calls the API on logout', async () => {
    mockedLogin.mockResolvedValue(admin)
    mockedLogout.mockRejectedValue(new Error('network down'))

    const store = useOpsAuthStore()
    await store.login('admin', 'secret')
    await store.logout()

    expect(mockedLogout).toHaveBeenCalledTimes(1)
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.ready).toBe(true)
  })

  it('clears local identity through clear', async () => {
    mockedLogin.mockResolvedValue(admin)

    const store = useOpsAuthStore()
    await store.login('admin', 'secret')
    store.clear()

    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.ready).toBe(true)
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import * as agentRolesApi from '@/api/agentRoles'
import type { AgentRole } from '@/types'

import { useAgentRolesStore } from './agentRoles'

vi.mock('@/api/agentRoles', () => ({
  listAgentRoles: vi.fn(),
}))

const roles: AgentRole[] = [
  {
    id: 6,
    roleCode: 'team',
    roleNameEn: 'Team',
    roleNameZh: '团队',
    description: 'all',
    avatarUrl: '',
  },
  {
    id: 1,
    roleCode: 'alex',
    roleNameEn: 'Alex',
    roleNameZh: '全栈工程师',
    description: 'code',
    avatarUrl: '',
  },
]

describe('agentRoles store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.mocked(agentRolesApi.listAgentRoles).mockReset()
    vi.mocked(agentRolesApi.listAgentRoles).mockResolvedValue(roles)
  })

  it('loads roles once and finds by id', async () => {
    const store = useAgentRolesStore()
    await store.load()
    await store.load()
    expect(store.roles).toHaveLength(2)
    expect(store.byId(1)?.roleCode).toBe('alex')
    expect(store.team?.id).toBe(6)
    expect(agentRolesApi.listAgentRoles).toHaveBeenCalledTimes(1)
  })

  it('refetches when forced', async () => {
    const store = useAgentRolesStore()
    await store.load()
    await store.load(true)
    expect(agentRolesApi.listAgentRoles).toHaveBeenCalledTimes(2)
  })

  it('does not mark loaded when loading fails', async () => {
    vi.mocked(agentRolesApi.listAgentRoles).mockRejectedValueOnce(new Error('boom'))
    const store = useAgentRolesStore()

    await store.load()

    expect(store.error).toBe('boom')
    expect(store.loaded).toBe(false)

    await store.load()

    expect(agentRolesApi.listAgentRoles).toHaveBeenCalledTimes(2)
    expect(store.loaded).toBe(true)
    expect(store.error).toBe('')
  })

  it('shares an in-flight load across concurrent calls', async () => {
    let resolveLoad: (value: AgentRole[]) => void = () => {}
    vi.mocked(agentRolesApi.listAgentRoles).mockReturnValue(
      new Promise((resolve) => {
        resolveLoad = resolve
      }),
    )
    const store = useAgentRolesStore()

    const first = store.load()
    const second = store.load()
    resolveLoad(roles)
    await Promise.all([first, second])

    expect(agentRolesApi.listAgentRoles).toHaveBeenCalledTimes(1)
    expect(store.roles).toHaveLength(2)
    expect(store.loading).toBe(false)
  })
})

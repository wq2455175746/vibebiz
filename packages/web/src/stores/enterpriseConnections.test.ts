import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { CodeRepoConnection } from '@/types/enterpriseSpace'
import { useEnterpriseConnectionsStore } from './enterpriseConnections'

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  setActivePinia(createPinia())
})

describe('useEnterpriseConnectionsStore', () => {
  it('creates, updates and removes a connection', () => {
    const store = useEnterpriseConnectionsStore()
    const created = store.create({
      kind: 'code-repo',
      name: '主仓库',
      provider: 'github',
      url: 'https://github.com/acme/app',
      username: 'acme',
      secret: 'token',
      defaultBranch: 'main',
    }) as CodeRepoConnection

    expect(store.codeRepos).toHaveLength(1)
    expect(created.id).toBeTruthy()

    store.update({ ...created, name: '改名仓库' })
    expect(store.codeRepos[0].name).toBe('改名仓库')

    store.remove(created.id)
    expect(store.codeRepos).toHaveLength(0)
  })

  it('keeps an existing secret when the edit value is empty', () => {
    const store = useEnterpriseConnectionsStore()
    const created = store.create({
      kind: 'code-repo',
      name: '主仓库',
      provider: 'gitlab',
      url: 'https://gitlab.com/acme/app',
      username: 'acme',
      secret: 'token',
      defaultBranch: 'main',
    }) as CodeRepoConnection

    store.update({ ...created, secret: '' })
    expect(store.codeRepos[0].secret).toBe('token')
  })

  it('loads persisted connections on a new store instance', () => {
    const first = useEnterpriseConnectionsStore()
    first.create({
      kind: 'harbor',
      name: '镜像库',
      registryUrl: 'https://harbor.example.com',
      namespace: 'vibebiz',
      username: 'robot',
      secret: 'password',
    })

    setActivePinia(createPinia())
    const second = useEnterpriseConnectionsStore()
    expect(second.harbors).toHaveLength(1)
  })
})

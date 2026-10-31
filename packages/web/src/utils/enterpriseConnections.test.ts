import { describe, expect, it } from 'vitest'

import type { CodeRepoConnection, HarborConnection } from '@/types/enterpriseSpace'
import {
  CONNECTIONS_STORAGE_KEY,
  connectionDescription,
  loadConnections,
  maskedCredential,
  saveConnections,
} from './enterpriseConnections'

const repo: CodeRepoConnection = {
  id: 'r1',
  kind: 'code-repo',
  name: '主仓库',
  provider: 'github',
  url: 'https://github.com/acme/app',
  username: 'acme',
  secret: 'token',
  defaultBranch: 'main',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

const harbor: HarborConnection = {
  id: 'h1',
  kind: 'harbor',
  name: '主镜像库',
  registryUrl: 'https://harbor.example.com/',
  namespace: 'vibebiz',
  username: 'robot',
  secret: 'password',
  createdAt: '2026-10-31T00:00:00Z',
  updatedAt: '2026-10-31T00:00:00Z',
}

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

describe('enterprise connections storage', () => {
  it('round-trips connections', () => {
    const storage = memoryStorage()
    saveConnections([repo, harbor], storage)
    expect(loadConnections(storage)).toEqual([repo, harbor])
  })

  it('falls back to empty data for invalid JSON or version', () => {
    const storage = memoryStorage({ [CONNECTIONS_STORAGE_KEY]: '{bad' })
    expect(loadConnections(storage)).toEqual([])
    storage.setItem(CONNECTIONS_STORAGE_KEY, JSON.stringify({ version: 2, connections: [repo] }))
    expect(loadConnections(storage)).toEqual([])
  })

  it('describes connections without exposing secrets', () => {
    expect(connectionDescription(repo)).toContain('github')
    expect(connectionDescription(repo)).not.toContain('token')
    expect(connectionDescription(harbor)).toContain('harbor.example.com/vibebiz')
    expect(maskedCredential('secret')).toBe('••••••••')
    expect(maskedCredential('')).toBe('未配置')
  })
})

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { mockCards } from '@/mocks/cards'
import { useCardStore } from './card'
import type { CardProjectItem } from '@/types'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 7, username: 'alice', avatarUrl: null } }),
}))

vi.mock('@/api/projects', () => ({
  getProject: vi.fn().mockResolvedValue({ previewPath: 'docs/demo.pdf' }),
}))

vi.mock('@/api/publish', () => ({
  listPublishedProjects: vi.fn().mockResolvedValue([]),
}))

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

function seedItem(store: ReturnType<typeof useCardStore>, projectId: string) {
  const item: CardProjectItem = {
    projectId,
    name: projectId,
    description: '',
    coverUrl: null,
    type: 'html',
    shareUrl: `/p/${projectId}`,
    addedAt: '2026-11-08T00:00:00.000Z',
  }
  store.$patch({ mine: { ...store.mine, items: [...store.mine.items, item] } })
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  setActivePinia(createPinia())
})

describe('card store', () => {
  it('initializes an empty own card and 12 mock cards', () => {
    const store = useCardStore()
    store.load()
    expect(store.mine.id).toBe('me')
    expect(store.mine.ownerName).toBe('alice')
    expect(store.mine.items).toEqual([])
    expect(store.discoverCards.filter((card) => card.id.startsWith('demo-'))).toHaveLength(
      mockCards.length,
    )
  })

  it('adds, deduplicates and limits tags', () => {
    const store = useCardStore()
    store.load()
    expect(store.addTag('Vue')).toBe('')
    expect(store.addTag('Vue')).toBe('标签已存在')
    for (const tag of ['Go', 'Rust', 'SQL', 'AI']) store.addTag(tag)
    expect(store.addTag('Next')).toContain('最多')
    expect(store.mine.tags).toEqual(['Vue', 'Go', 'Rust', 'SQL', 'AI'])
  })

  it('adds a published project with preview type from project detail', async () => {
    const store = useCardStore()
    store.load()
    const item = await store.addPublishedProject({
      projectId: 'p1',
      name: '作品一',
      description: '介绍',
      tags: [],
      coverUrl: null,
      publishedAt: '2026-11-08T00:00:00.000Z',
      url: '/p/p1',
    })
    expect(item.type).toBe('pdf')
    expect(store.mine.items).toHaveLength(1)
  })

  it('clears featured when its project is removed', () => {
    const store = useCardStore()
    store.load()
    seedItem(store, 'p1')
    store.setFeatured({ kind: 'project', projectId: 'p1' })
    store.removeItem('p1')
    expect(store.mine.featured).toBeNull()
    expect(store.mine.items).toEqual([])
  })

  it('requires items before publishing and persists the card', () => {
    const store = useCardStore()
    store.load()
    expect(() => store.publish()).toThrow('请先添加项目')
    seedItem(store, 'p1')
    store.publish()
    expect(store.mine.published).toBe(true)
    expect(localStorage.getItem('vibebiz:cards:mine:v1')).toContain('"published":true')
  })

  it('finds a project from the own card and paginates discover cards', () => {
    const store = useCardStore()
    store.load()
    seedItem(store, 'p1')
    expect(store.findProject('p1')?.item.projectId).toBe('p1')
    const first = store.listDiscover(1, 6)
    expect(first.items).toHaveLength(6)
    expect(first.total).toBe(mockCards.length)
  })
})

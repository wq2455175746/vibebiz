import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { mockCards } from '@/mocks/cards'
import { getProject } from '@/api/projects'
import { useCardStore } from './card'
import type { CardProjectItem, Project, PublishedProject } from '@/types'

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

const getProjectMock = vi.mocked(getProject)

function published(projectId: string): PublishedProject {
  return {
    projectId,
    name: projectId,
    description: '',
    tags: [],
    coverUrl: null,
    publishedAt: '2026-11-08T00:00:00.000Z',
    url: `/p/${projectId}`,
  }
}

function projectDetail(previewPath: string): Project {
  return {
    id: 'p1',
    name: 'p1',
    prompt: '',
    previewPath,
    state: 'active',
    createdAt: '2026-11-08T00:00:00.000Z',
    updatedAt: '2026-11-08T00:00:00.000Z',
  }
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  getProjectMock.mockReset()
  getProjectMock.mockResolvedValue(projectDetail('docs/demo.pdf'))
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

  it('rejects a concurrent duplicate add after the detail request resolves', async () => {
    const store = useCardStore()
    store.load()
    const resolvers: Array<(project: Project) => void> = []
    getProjectMock.mockImplementation(
      () => new Promise<Project>((resolve) => resolvers.push(resolve)),
    )
    const first = store.addPublishedProject(published('p1'))
    const second = store.addPublishedProject(published('p1'))
    expect(resolvers).toHaveLength(2)
    resolvers[0](projectDetail('index.html'))
    await first
    resolvers[1](projectDetail('index.html'))
    await expect(second).rejects.toThrow('项目已添加或已达到 5 个上限')
    expect(store.mine.items).toHaveLength(1)
  })

  it('marks saving immediately and clears after the debounce', async () => {
    vi.useFakeTimers()
    try {
      const store = useCardStore()
      store.load()
      store.setHeadline('hello')
      expect(store.saving).toBe(true)
      await vi.runAllTimersAsync()
      expect(store.saving).toBe(false)
    } finally {
      vi.useRealTimers()
    }
  })
})

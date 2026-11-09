import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import DiscoverPanel from './DiscoverPanel.vue'
import { getDiscoverCards } from '@/api/cards'
import { emptyCard } from '@/utils/card'
import type { BusinessCard } from '@/types'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 7, username: 'alice', avatarUrl: null } }),
}))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

vi.mock('@/api/cards', () => ({
  getMyCard: vi.fn(),
  updateMyCard: vi.fn(),
  addCardProject: vi.fn(),
  removeCardProject: vi.fn(),
  setCardFeaturedProject: vi.fn(),
  uploadCardFeatured: vi.fn(),
  clearCardFeatured: vi.fn(),
  publishMyCard: vi.fn(),
  getDiscoverCards: vi.fn(),
  getPublicCard: vi.fn(),
  getPublicCardProject: vi.fn(),
}))

function makeCard(index: number): BusinessCard {
  return {
    ...emptyCard(),
    id: `card-${index}`,
    ownerName: index === 1 ? '林一' : `开发者 ${index}`,
    headline: '前端工程师',
    tags: ['Vue', '游戏'],
    items: [
      {
        projectId: `p${index}`,
        name: `作品${index}`,
        description: '',
        coverUrl: null,
        type: 'html',
        shareUrl: `/p/p${index}`,
      },
    ],
    featured: { kind: 'project', projectId: `p${index}` },
    commentCount: index,
  }
}

const pageOne = Array.from({ length: 6 }, (_, index) => makeCard(index + 1))
const pageTwo = Array.from({ length: 6 }, (_, index) => makeCard(index + 7))

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(DiscoverPanel)
  app.use(createPinia())
  app.mount(host)
}

beforeEach(() => {
  vi.stubGlobal('localStorage', memoryStorage())
  vi.mocked(getDiscoverCards).mockImplementation(async (params = {}) => {
    const page = params.page ?? 1
    return {
      cards: page === 1 ? pageOne : pageTwo,
      total: 12,
      page,
      pageSize: 6,
    }
  })
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  vi.clearAllMocks()
})

describe('DiscoverPanel', () => {
  it('renders the first page from the backend', async () => {
    mount()
    await flush()
    expect(getDiscoverCards).toHaveBeenCalledWith({ page: 1, pageSize: 12 })
    expect(host.querySelectorAll('.card-tile')).toHaveLength(6)
    expect(host.textContent).toContain('林一')
  })

  it('loads the next page on click', async () => {
    mount()
    await flush()
    host.querySelector<HTMLButtonElement>('.discover-more')?.click()
    await flush()
    expect(getDiscoverCards).toHaveBeenLastCalledWith({ page: 2, pageSize: 12 })
    expect(host.querySelectorAll('.card-tile')).toHaveLength(12)
    expect(host.textContent).toContain('已到底')
  })

  it('shows loading instead of the end state before the first page resolves', async () => {
    vi.mocked(getDiscoverCards).mockReturnValue(new Promise(() => {}))
    mount()
    await flush()
    expect(host.textContent).toContain('加载中')
    expect(host.textContent).not.toContain('已到底')
  })

  it('shows the empty state without the end state for an empty result', async () => {
    vi.mocked(getDiscoverCards).mockResolvedValue({
      cards: [],
      total: 0,
      page: 1,
      pageSize: 12,
    })
    mount()
    await flush()
    expect(host.textContent).toContain('还没有人发布名片。')
    expect(host.textContent).not.toContain('已到底')
  })
})

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '@/api/cards'
import { useCardStore } from './card'

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

const card = (overrides = {}) => ({
  id: 'card-1',
  ownerUserId: 7,
  ownerName: 'alice',
  avatarUrl: '',
  headline: '',
  tags: [],
  items: [],
  featured: null,
  published: false,
  commentCount: 0,
  createdAt: '2026-11-09T00:00:00.000Z',
  updatedAt: '2026-11-09T00:00:00.000Z',
  ...overrides,
})

beforeEach(() => {
  vi.useRealTimers()
  vi.clearAllMocks()
  setActivePinia(createPinia())
})

describe('card store', () => {
  it('loads my card', async () => {
    vi.mocked(api.getMyCard).mockResolvedValue(card({ headline: 'hi' }))
    const store = useCardStore()
    await store.loadMine()
    expect(store.mine?.headline).toBe('hi')
    expect(store.loading).toBe(false)
  })

  it('optimistically updates headline and flushes to API', async () => {
    vi.useFakeTimers()
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    vi.mocked(api.updateMyCard).mockResolvedValue(card({ headline: 'new' }))
    const store = useCardStore()
    await store.loadMine()
    store.setHeadline('new')
    expect(store.mine?.headline).toBe('new')
    expect(store.saving).toBe(true)
    await vi.advanceTimersByTimeAsync(400)
    expect(api.updateMyCard).toHaveBeenCalledWith({ headline: 'new', tags: [] })
    expect(store.saving).toBe(false)
  })

  it('adds and removes projects through the API', async () => {
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    vi.mocked(api.addCardProject).mockResolvedValue(card({ items: [{ projectId: 'p1' }] }))
    vi.mocked(api.removeCardProject).mockResolvedValue(card())
    const store = useCardStore()
    await store.loadMine()
    await store.addProject('p1')
    expect(store.items).toHaveLength(1)
    await store.removeProject('p1')
    expect(store.items).toHaveLength(0)
  })

  it('publishes and returns a share link', async () => {
    vi.mocked(api.publishMyCard).mockResolvedValue(card({ published: true }))
    const store = useCardStore()
    await expect(store.publish()).resolves.toBe('/share/card/card-1')
  })

  it('appends discovery pages', async () => {
    vi.mocked(api.getDiscoverCards)
      .mockResolvedValueOnce({ cards: [card()], total: 2, page: 1, pageSize: 12 })
      .mockResolvedValueOnce({ cards: [card({ id: 'card-2' })], total: 2, page: 2, pageSize: 12 })
    const store = useCardStore()
    await store.loadDiscover(1, false)
    await store.loadDiscover(2, true)
    expect(store.discoverCards.map((item) => item.id)).toEqual(['card-1', 'card-2'])
    expect(store.discoverTotal).toBe(2)
  })
})

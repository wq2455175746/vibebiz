import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '@/api/cards'
import type { BusinessCard } from '@/types'
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
  likeCard: vi.fn(),
  unlikeCard: vi.fn(),
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
  likeCount: 0,
  liked: false,
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

  it('optimistically likes and reconciles with the API state', async () => {
    let resolveLike!: (value: { liked: boolean; likeCount: number }) => void
    vi.mocked(api.likeCard).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveLike = resolve
        }),
    )
    const store = useCardStore()
    const target = card({ likeCount: 2, liked: false })
    store.discoverCards = [target]

    const pending = store.toggleLike(target)
    expect(target.liked).toBe(true)
    expect(target.likeCount).toBe(3)

    resolveLike({ liked: true, likeCount: 9 })
    await pending
    expect(api.likeCard).toHaveBeenCalledWith('card-1')
    expect(target.liked).toBe(true)
    expect(target.likeCount).toBe(9)
  })

  it('optimistically unlikes and reconciles with the API state', async () => {
    vi.mocked(api.unlikeCard).mockResolvedValue({ liked: false, likeCount: 0 })
    const store = useCardStore()
    const target = card({ likeCount: 1, liked: true })
    store.discoverCards = [target]

    await store.toggleLike(target)
    expect(api.unlikeCard).toHaveBeenCalledWith('card-1')
    expect(target.liked).toBe(false)
    expect(target.likeCount).toBe(0)
  })

  it('rolls back an optimistic like when the API fails', async () => {
    vi.mocked(api.likeCard).mockRejectedValue(new Error('boom'))
    const store = useCardStore()
    const target = card({ likeCount: 2, liked: false })
    store.discoverCards = [target]

    await expect(store.toggleLike(target)).rejects.toThrow('boom')
    expect(target.liked).toBe(false)
    expect(target.likeCount).toBe(2)
    expect(store.error).toBe('boom')
  })

  it('keeps newer edits when an earlier save resolves late', async () => {
    vi.useFakeTimers()
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    let resolveFirst!: (value: BusinessCard) => void
    vi.mocked(api.updateMyCard)
      .mockImplementationOnce(
        () =>
          new Promise<BusinessCard>((resolve) => {
            resolveFirst = resolve
          }),
      )
      .mockResolvedValueOnce(card({ headline: 'second' }))

    const store = useCardStore()
    await store.loadMine()

    store.setHeadline('first')
    await vi.advanceTimersByTimeAsync(400)
    expect(api.updateMyCard).toHaveBeenCalledTimes(1)
    expect(store.saving).toBe(true)

    store.setHeadline('second')
    expect(store.saving).toBe(true)

    resolveFirst(card({ headline: 'first' }))
    await vi.advanceTimersByTimeAsync(0)
    expect(store.mine?.headline).toBe('second')
    expect(store.saving).toBe(true)

    await vi.advanceTimersByTimeAsync(400)
    expect(api.updateMyCard).toHaveBeenCalledTimes(2)
    expect(api.updateMyCard).toHaveBeenLastCalledWith({ headline: 'second', tags: [] })
    expect(store.mine?.headline).toBe('second')
    expect(store.saving).toBe(false)
  })

  it('keeps item changes made while a profile save is in flight', async () => {
    vi.useFakeTimers()
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    let resolveProfile!: (value: BusinessCard) => void
    vi.mocked(api.updateMyCard).mockImplementationOnce(
      () =>
        new Promise<BusinessCard>((resolve) => {
          resolveProfile = resolve
        }),
    )
    vi.mocked(api.addCardProject).mockResolvedValue(
      card({ items: [{ projectId: 'p1' }] }),
    )

    const store = useCardStore()
    await store.loadMine()

    store.setHeadline('x')
    await vi.advanceTimersByTimeAsync(400)
    await store.addProject('p1')
    expect(store.items).toHaveLength(1)

    resolveProfile(card({ headline: 'x' }))
    await vi.advanceTimersByTimeAsync(0)
    expect(store.mine?.headline).toBe('x')
    expect(store.items.map((item) => item.projectId)).toEqual(['p1'])
  })

  it('clears saving after a failed profile save and keeps the error', async () => {
    vi.useFakeTimers()
    vi.mocked(api.getMyCard).mockResolvedValue(card())
    vi.mocked(api.updateMyCard).mockRejectedValue(new Error('boom'))

    const store = useCardStore()
    await store.loadMine()

    store.setHeadline('x')
    expect(store.saving).toBe(true)
    await vi.advanceTimersByTimeAsync(400)
    expect(store.saving).toBe(false)
    expect(store.error).toBe('boom')
  })
})

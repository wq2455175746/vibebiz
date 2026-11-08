import { beforeEach, describe, expect, it, vi } from 'vitest'

import { deleteCard, listCards, updateCardHidden } from './cards'

const store = new Map<string, string>()

beforeEach(() => {
  store.clear()
  vi.stubGlobal('localStorage', {
    getItem: (key: string) => store.get(key) ?? null,
    setItem: (key: string, value: string) => {
      store.set(key, value)
    },
    removeItem: (key: string) => {
      store.delete(key)
    },
    clear: () => store.clear(),
  })
})

describe('cards api', () => {
  it('seeds and paginates cards', async () => {
    const result = await listCards({ page: 1, pageSize: 5 })
    expect(result.items).toHaveLength(5)
    expect(result.total).toBeGreaterThan(5)
  })

  it('filters by keyword', async () => {
    const result = await listCards({ keyword: '林一' })
    expect(result.items.some((card) => card.ownerName.includes('林一'))).toBe(true)
  })

  it('toggles hidden state', async () => {
    const first = (await listCards({ page: 1, pageSize: 1 })).items[0]!
    await updateCardHidden(first.id, true)
    const reloaded = await listCards({ keyword: first.ownerName })
    expect(reloaded.items.find((card) => card.id === first.id)?.hidden).toBe(true)
  })

  it('deletes a card', async () => {
    const first = (await listCards({ page: 1, pageSize: 1 })).items[0]!
    await deleteCard(first.id)
    const reloaded = await listCards({})
    expect(reloaded.items.some((card) => card.id === first.id)).toBe(false)
  })
})

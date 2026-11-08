import { cardSeed } from '@/mocks/cards'
import type { AdminCardSummary, PageQuery, PageResult } from '@/types'

const KEY = 'vibebiz:ops:cards:v1'

function delay(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 200))
}

function read(): AdminCardSummary[] {
  const raw = localStorage.getItem(KEY)
  if (!raw) {
    const seed = cardSeed.map((card) => ({ ...card }))
    localStorage.setItem(KEY, JSON.stringify(seed))
    return seed
  }
  try {
    const parsed = JSON.parse(raw) as AdminCardSummary[]
    return Array.isArray(parsed) ? parsed : []
  } catch {
    return []
  }
}

function write(cards: AdminCardSummary[]): void {
  localStorage.setItem(KEY, JSON.stringify(cards))
}

export async function listCards(query: PageQuery = {}): Promise<PageResult<AdminCardSummary>> {
  await delay()
  const keyword = (query.keyword ?? '').trim().toLowerCase()
  const page = Math.max(1, query.page ?? 1)
  const pageSize = Math.max(1, query.pageSize ?? 10)
  const filtered = read().filter((card) => {
    if (!keyword) return true
    return (
      card.ownerName.toLowerCase().includes(keyword) ||
      card.user.toLowerCase().includes(keyword)
    )
  })
  return {
    items: filtered.slice((page - 1) * pageSize, page * pageSize),
    total: filtered.length,
    page,
    pageSize,
  }
}

export async function updateCardHidden(id: string, hidden: boolean): Promise<void> {
  await delay()
  write(read().map((card) => (card.id === id ? { ...card, hidden } : card)))
}

export async function deleteCard(id: string): Promise<void> {
  await delay()
  write(read().filter((card) => card.id !== id))
}

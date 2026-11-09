import { createApp, type App } from 'vue'
import PrimeVue from 'primevue/config'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { deleteCard, listCards, updateCardHidden } from '@/api/cards'
import type { AdminCardSummary } from '@/types'
import CardsView from './CardsView.vue'

vi.mock('@/api/cards', () => ({
  listCards: vi.fn(),
  updateCardHidden: vi.fn(),
  deleteCard: vi.fn(),
}))

const listMock = vi.mocked(listCards)
const hideMock = vi.mocked(updateCardHidden)
const deleteMock = vi.mocked(deleteCard)

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function card(id: string): AdminCardSummary {
  return {
    id,
    ownerUserId: 1,
    ownerName: `owner-${id}`,
    headline: `headline-${id}`,
    tags: ['Vue'],
    projectCount: 1,
    featuredType: '项目',
    published: false,
    hidden: false,
    status: '未发布',
    updatedAt: '2026-11-08T10:00:00.000Z',
  }
}

beforeEach(() => {
  vi.clearAllMocks()
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  })
  listMock.mockResolvedValue({
    items: [card('c1')],
    total: 1,
    page: 1,
    pageSize: 10,
  })
  hideMock.mockResolvedValue(undefined)
  deleteMock.mockResolvedValue(undefined)
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardsView)
  app.use(PrimeVue)
  app.mount(host)
}

describe('CardsView', () => {
  it('renders owner name, headline and status', async () => {
    mount()
    await flush()
    expect(host.textContent).toContain('owner-c1')
    expect(host.textContent).toContain('headline-c1')
    expect(host.textContent).toContain('未发布')
  })

  it('hides a card and reloads', async () => {
    mount()
    await flush()
    const hideButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('隐藏'),
    )
    hideButton?.click()
    await flush()
    expect(hideMock).toHaveBeenCalledWith('c1', true)
    expect(listMock).toHaveBeenCalledTimes(2)
  })

  it('deletes a card after confirmation', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    mount()
    await flush()
    const deleteButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('删除'),
    )
    deleteButton?.click()
    await flush()
    expect(deleteMock).toHaveBeenCalledWith('c1')
  })

  it('returns to the previous page after deleting the last row on a later page', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true)
    listMock.mockImplementation(async (query) => {
      if ((query?.page ?? 1) === 2) {
        return { items: [card('c11')], total: 11, page: 2, pageSize: 10 }
      }
      return {
        items: Array.from({ length: 10 }, (_, index) => card(`c${index + 1}`)),
        total: 11,
        page: 1,
        pageSize: 10,
      }
    })
    mount()
    await flush()
    const pageTwo = Array.from(host.querySelectorAll<HTMLButtonElement>('.p-paginator-page')).find(
      (button) => button.textContent?.trim() === '2',
    )
    pageTwo?.click()
    await flush()
    expect(listMock).toHaveBeenLastCalledWith(expect.objectContaining({ page: 2 }))
    const deleteButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('删除'),
    )
    deleteButton?.click()
    await flush()
    expect(deleteMock).toHaveBeenCalledWith('c11')
    expect(listMock).toHaveBeenLastCalledWith(expect.objectContaining({ page: 1 }))
  })
})

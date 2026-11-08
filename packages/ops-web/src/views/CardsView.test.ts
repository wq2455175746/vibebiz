import { createApp, type App } from 'vue'
import PrimeVue from 'primevue/config'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { deleteCard, listCards, updateCardHidden } from '@/api/cards'
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
    items: [
      {
        id: 'demo-1',
        ownerName: '林一',
        user: 'alice',
        tags: ['Vue', '游戏'],
        projectCount: 3,
        featuredType: 'HTML',
        hidden: false,
        updatedAt: '2026-11-08T10:00:00.000Z',
      },
    ],
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
  it('renders card rows', async () => {
    mount()
    await flush()
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('Vue')
  })

  it('hides a card and reloads', async () => {
    mount()
    await flush()
    const hideButton = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.includes('隐藏'),
    )
    hideButton?.click()
    await flush()
    expect(hideMock).toHaveBeenCalledWith('demo-1', true)
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
    expect(deleteMock).toHaveBeenCalledWith('demo-1')
  })
})

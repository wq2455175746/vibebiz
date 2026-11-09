import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CardShareView from './CardShareView.vue'
import * as cardsApi from '@/api/cards'
import type { BusinessCard } from '@/types'

const routeState = vi.hoisted(() => ({ id: 'demo-1' }))
const routerState = vi.hoisted(() => ({ push: vi.fn() }))
const authState = vi.hoisted(() => ({
  user: null as { id: number } | null,
  ensure: vi.fn(async () => {}),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: routeState.id } }),
  useRouter: () => ({ push: routerState.push }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/api/cards', () => ({ getPublicCard: vi.fn() }))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeCard(overrides: Partial<BusinessCard> = {}): BusinessCard {
  return {
    id: 'demo-1',
    ownerUserId: 7,
    ownerName: '林一',
    avatarUrl: null,
    headline: '前端工程师',
    tags: ['Vue', '游戏'],
    items: [
      {
        projectId: 'demo-1-p1',
        name: '作品一',
        description: '',
        coverUrl: null,
        type: 'html',
        shareUrl: '/share/project/demo-1-p1',
        addedAt: '2026-11-08T00:00:00.000Z',
      },
    ],
    featured: { kind: 'project', projectId: 'demo-1-p1' },
    published: true,
    commentCount: 3,
    createdAt: '2026-11-08T00:00:00.000Z',
    updatedAt: '2026-11-08T00:00:00.000Z',
    ...overrides,
  }
}

let app: App | null = null
let host: HTMLElement

async function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardShareView)
  app.use(createPinia())
  app.mount(host)
  await flushPromises()
}

beforeEach(() => {
  vi.mocked(cardsApi.getPublicCard).mockReset()
  vi.mocked(cardsApi.getPublicCard).mockResolvedValue(makeCard())
  authState.user = null
  authState.ensure.mockClear()
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  routeState.id = 'demo-1'
  routerState.push.mockReset()
  vi.restoreAllMocks()
})

describe('CardShareView', () => {
  it('renders the card profile and projects from the public API', async () => {
    await mount()
    expect(cardsApi.getPublicCard).toHaveBeenCalledWith('demo-1')
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('作品一')
  })

  it('keeps comment and message entries disabled', async () => {
    await mount()
    expect(host.querySelector<HTMLButtonElement>('.card-social-comment')?.disabled).toBe(true)
    expect(host.querySelector<HTMLButtonElement>('.card-social-message')?.disabled).toBe(true)
  })

  it('opens the featured project from the main preview', async () => {
    await mount()
    host.querySelector<HTMLElement>('.share-featured')?.click()
    expect(routerState.push).toHaveBeenCalledWith({
      name: 'project-share',
      params: { projectId: 'demo-1-p1' },
    })
  })

  it('opens the featured file in a preview tab instead of downloading it', async () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    vi.mocked(cardsApi.getPublicCard).mockResolvedValue(
      makeCard({
        featured: {
          kind: 'file',
          fileType: 'image',
          name: 'cover.png',
          size: 1024,
          url: 'https://example.com/cover.png',
        },
      }),
    )
    await mount()
    host.querySelector<HTMLElement>('.share-featured')?.click()
    expect(open).toHaveBeenCalledWith('/share/card/demo-1/file', '_blank', 'noopener,noreferrer')
  })

  it('does not render remove buttons on the public share page', async () => {
    await mount()
    expect(host.querySelectorAll('.card-project-remove')).toHaveLength(0)
  })

  it('shows an empty state when the card cannot be loaded', async () => {
    vi.mocked(cardsApi.getPublicCard).mockRejectedValue(new Error('名片不存在'))
    await mount()
    expect(host.querySelector('.share-empty')).not.toBeNull()
    expect(host.textContent).toContain('名片不存在')
  })

  it('shows an edit entry only for the own card', async () => {
    authState.user = null
    await mount()
    expect(host.textContent).not.toContain('编辑名片')
    app?.unmount()
    host.remove()
    authState.user = { id: 7 }
    await mount()
    const edit = Array.from(host.querySelectorAll<HTMLAnchorElement>('a')).find(
      (link) => link.textContent === '编辑名片',
    )
    expect(edit?.getAttribute('href')).toBe('/my-card')
  })
})

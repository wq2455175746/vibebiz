import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CardFilePreviewView from './CardFilePreviewView.vue'
import * as cardsApi from '@/api/cards'
import type { BusinessCard, CardFeatured } from '@/types'

const routeState = vi.hoisted(() => ({ id: 'demo-1' }))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: routeState.id } }),
}))
vi.mock('@/api/cards', () => ({ getPublicCard: vi.fn() }))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeCard(featured: CardFeatured): BusinessCard {
  return {
    id: 'demo-1',
    ownerUserId: 7,
    ownerName: '林一',
    avatarUrl: null,
    headline: '前端工程师',
    tags: [],
    items: [],
    featured,
    published: true,
    commentCount: 0,
    createdAt: '2026-11-08T00:00:00.000Z',
    updatedAt: '2026-11-08T00:00:00.000Z',
  }
}

let app: App | null = null
let host: HTMLElement

async function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardFilePreviewView)
  app.use(createPinia())
  app.mount(host)
  await flushPromises()
}

beforeEach(() => {
  if (typeof URL.createObjectURL !== 'function') {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:card-file-preview'),
    })
  }
  if (typeof URL.revokeObjectURL !== 'function') {
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
  }
  routeState.id = 'demo-1'
  vi.mocked(cardsApi.getPublicCard).mockReset()
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardFilePreviewView', () => {
  it('renders a featured image inline instead of downloading it', async () => {
    vi.mocked(cardsApi.getPublicCard).mockResolvedValue(
      makeCard({
        kind: 'file',
        fileType: 'image',
        name: 'me.png',
        size: 1024,
        url: 'https://example.com/me.png',
      }),
    )
    await mount()
    expect(cardsApi.getPublicCard).toHaveBeenCalledWith('demo-1')
    const image = host.querySelector('img')
    expect(image?.getAttribute('src')).toBe('https://example.com/me.png')
    expect(host.textContent).toContain('me.png')
  })

  it('renders a featured pdf in an iframe', async () => {
    vi.mocked(cardsApi.getPublicCard).mockResolvedValue(
      makeCard({
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 2048,
        url: 'https://example.com/resume.pdf',
      }),
    )
    await mount()
    expect(host.querySelector('.share-stage iframe')).not.toBeNull()
    expect(host.querySelector('img')).toBeNull()
  })

  it('shows an empty state when the featured content is not a file', async () => {
    vi.mocked(cardsApi.getPublicCard).mockResolvedValue(
      makeCard({ kind: 'project', projectId: 'demo-1-p1' }),
    )
    await mount()
    expect(host.textContent).toContain('暂无可预览内容')
  })

  it('shows an error when the card cannot be loaded', async () => {
    vi.mocked(cardsApi.getPublicCard).mockRejectedValue(new Error('名片不存在'))
    await mount()
    expect(host.textContent).toContain('名片不存在')
  })
})

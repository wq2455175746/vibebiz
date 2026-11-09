import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ProjectShareView from './ProjectShareView.vue'
import * as cardsApi from '@/api/cards'
import type { CardContentType, PublicProjectCard } from '@/types'

const projectState = vi.hoisted(() => ({
  cardId: 'demo-1',
  shareUrl: '/p/demo-1-p1',
}))

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { projectId: 'demo-1-p1' } }) }))
vi.mock('@/api/cards', () => ({ getPublicCardProject: vi.fn() }))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeProject(type: CardContentType, previewUrl?: string): PublicProjectCard {
  const item: PublicProjectCard['item'] = {
    projectId: 'demo-1-p1',
    name: '作品一',
    description: '',
    coverUrl: null,
    type,
    shareUrl: projectState.shareUrl,
    addedAt: '2026-11-08T00:00:00.000Z',
  }
  if (previewUrl !== undefined) item.previewUrl = previewUrl
  return {
    card: { id: projectState.cardId, ownerName: '林一', avatarUrl: '' },
    item,
  }
}

let app: App | null = null
let host: HTMLElement

async function mount(type: CardContentType, previewUrl?: string) {
  vi.mocked(cardsApi.getPublicCardProject).mockResolvedValue(makeProject(type, previewUrl))
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(ProjectShareView)
  app.use(createPinia())
  app.mount(host)
  await flushPromises()
}

beforeEach(() => {
  if (typeof URL.createObjectURL !== 'function') {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:project-share'),
    })
  }
  if (typeof URL.revokeObjectURL !== 'function') {
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
  }
  vi.mocked(cardsApi.getPublicCardProject).mockReset()
  projectState.cardId = 'demo-1'
  projectState.shareUrl = '/p/demo-1-p1'
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('ProjectShareView', () => {
  it('loads the project from the public API', async () => {
    await mount('html')
    expect(cardsApi.getPublicCardProject).toHaveBeenCalledWith('demo-1-p1')
    expect(host.textContent).toContain('作品一')
  })

  it('renders html in an iframe from the share url', async () => {
    await mount('html')
    const frame = host.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')).toBe('/p/demo-1-p1')
  })

  it('does not grant same-origin to html iframes', async () => {
    await mount('html')
    const sandbox = host.querySelector('iframe')?.getAttribute('sandbox') ?? ''
    expect(sandbox).not.toContain('allow-same-origin')
    expect(sandbox).toContain('allow-scripts')
  })

  it.each(['image', 'pdf', 'video', 'document'] as const)(
    'shows the fallback for %s projects without previewUrl',
    async (type) => {
      await mount(type)
      expect(host.textContent).toContain('暂无可预览内容')
      expect(host.querySelector('img')).toBeNull()
      expect(host.querySelector('video')).toBeNull()
      expect(host.querySelector('iframe')).toBeNull()
      expect(host.querySelector('.share-document')).toBeNull()
    },
  )

  it('renders previewUrl when supplied (forward-compatible)', async () => {
    await mount('image', 'https://example.com/preview.png')
    const image = host.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('https://example.com/preview.png')
  })

  it('hides the original project entry without a real url', async () => {
    projectState.shareUrl = 'about:blank'
    await mount('image')
    expect(host.textContent).not.toContain('打开原项目')
  })

  it('hides the card entry when the project is not in any card', async () => {
    projectState.cardId = ''
    await mount('html')
    expect(host.textContent).not.toContain('查看名片')
  })
})

import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ProjectShareView from './ProjectShareView.vue'
import * as cardsApi from '@/api/cards'
import type { CardContentType, PublicProjectCard } from '@/types'

const projectState = vi.hoisted(() => ({
  cardId: 'demo-1',
  shareUrl: '/p/demo-1-p1',
  previewUrl: 'https://example.com/demo' as string | undefined,
}))

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { projectId: 'demo-1-p1' } }) }))
vi.mock('@/api/cards', () => ({ getPublicCardProject: vi.fn() }))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeProject(type: CardContentType, previewUrl?: string): PublicProjectCard {
  projectState.previewUrl =
    previewUrl ??
    (type === 'pdf' ? 'data:application/pdf;base64,JVBERi0xLjQK' : 'https://example.com/demo')
  return {
    card: { id: projectState.cardId, ownerName: '林一', avatarUrl: '' },
    item: {
      projectId: 'demo-1-p1',
      name: '作品一',
      description: '',
      coverUrl: null,
      type,
      shareUrl: projectState.shareUrl,
      previewUrl: projectState.previewUrl,
      addedAt: '2026-11-08T00:00:00.000Z',
    },
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

  it('renders html in an iframe', async () => {
    await mount('html')
    expect(host.querySelector('iframe')).not.toBeNull()
  })

  it('does not grant same-origin to html iframes', async () => {
    await mount('html')
    const sandbox = host.querySelector('iframe')?.getAttribute('sandbox') ?? ''
    expect(sandbox).not.toContain('allow-same-origin')
    expect(sandbox).toContain('allow-scripts')
  })

  it('does not sandbox pdf iframes so Chrome can render them', async () => {
    await mount('pdf')
    expect(host.querySelector('iframe')?.getAttribute('sandbox')).toBeNull()
  })

  it('renders an image', async () => {
    await mount('image')
    expect(host.querySelector('img')).not.toBeNull()
  })

  it('renders a pdf iframe through a blob url', async () => {
    await mount('pdf')
    const frame = host.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')?.startsWith('blob:')).toBe(true)
  })

  it('renders a video', async () => {
    await mount('video')
    expect(host.querySelector('video')).not.toBeNull()
  })

  it('renders a document', async () => {
    await mount('document')
    expect(host.querySelector('.share-document')).not.toBeNull()
  })

  it('shows a placeholder when no preview is available', async () => {
    await mount('image', '')
    expect(host.textContent).toContain('暂无可预览内容')
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

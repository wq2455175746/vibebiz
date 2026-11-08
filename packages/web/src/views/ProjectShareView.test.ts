import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ProjectShareView from './ProjectShareView.vue'
import type { CardContentType } from '@/types'

let currentType: CardContentType = 'html'

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
})

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { projectId: 'demo-1-p1' } }) }))

vi.mock('@/stores/card', () => ({
  useCardStore: () => ({
    load: vi.fn(),
    findProject: () => ({
      card: { id: 'demo-1', ownerName: '林一' },
      item: {
        projectId: 'demo-1-p1',
        name: '作品一',
        description: '',
        coverUrl: null,
        type: currentType,
        shareUrl: '/p/demo-1-p1',
        previewUrl:
          currentType === 'pdf'
            ? 'data:application/pdf;base64,JVBERi0xLjQK'
            : 'https://example.com/demo',
        addedAt: '2026-11-08T00:00:00.000Z',
      },
    }),
  }),
}))

let app: App | null = null
let host: HTMLElement

function mount(type: CardContentType) {
  currentType = type
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(ProjectShareView)
  app.use(createPinia())
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('ProjectShareView', () => {
  it('renders html in an iframe', () => {
    mount('html')
    expect(host.querySelector('iframe')).not.toBeNull()
  })

  it('does not grant same-origin to html iframes', () => {
    mount('html')
    const sandbox = host.querySelector('iframe')?.getAttribute('sandbox') ?? ''
    expect(sandbox).not.toContain('allow-same-origin')
    expect(sandbox).toContain('allow-scripts')
  })

  it('uses a pdf-viewer-compatible sandbox for pdf iframes', () => {
    mount('pdf')
    const sandbox = host.querySelector('iframe')?.getAttribute('sandbox') ?? ''
    expect(sandbox).toContain('allow-same-origin')
    expect(sandbox).toContain('allow-scripts')
  })

  it('renders an image', () => {
    mount('image')
    expect(host.querySelector('img')).not.toBeNull()
  })

  it('renders a pdf iframe through a blob url', () => {
    mount('pdf')
    const frame = host.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')?.startsWith('blob:')).toBe(true)
  })

  it('renders a video', () => {
    mount('video')
    expect(host.querySelector('video')).not.toBeNull()
  })

  it('renders a document', () => {
    mount('document')
    expect(host.querySelector('.share-document')).not.toBeNull()
  })
})

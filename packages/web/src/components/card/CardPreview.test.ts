import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CardPreview from './CardPreview.vue'

let app: App | null = null
let host: HTMLElement

beforeEach(() => {
  if (typeof URL.createObjectURL !== 'function') {
    Object.defineProperty(URL, 'createObjectURL', {
      configurable: true,
      value: vi.fn(() => 'blob:card-preview'),
    })
  }
  if (typeof URL.revokeObjectURL !== 'function') {
    Object.defineProperty(URL, 'revokeObjectURL', { configurable: true, value: vi.fn() })
  }
})

function mount(props: Record<string, unknown>) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardPreview, props)
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardPreview', () => {
  it('renders uploaded pdf bytes through a blob url', () => {
    mount({
      large: true,
      items: [],
      featured: {
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 1024,
        dataUrl: 'data:application/pdf;base64,JVBERi0xLjQK',
      },
    })
    expect(host.querySelector('iframe')?.getAttribute('src')?.startsWith('blob:')).toBe(true)
  })

  it('embeds the pdf viewer even in compact discover cards', () => {
    mount({
      items: [],
      featured: {
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 1024,
        dataUrl: 'data:application/pdf;base64,JVBERi0xLjQK',
      },
    })
    const frame = host.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')?.startsWith('blob:')).toBe(true)
    expect(frame?.getAttribute('sandbox')).toBeNull()
    expect(host.querySelector('.card-preview-file')).toBeNull()
  })

  it('does not sandbox uploaded pdf previews so Chrome can render them', () => {
    mount({
      large: true,
      items: [],
      featured: {
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 1024,
        dataUrl: 'data:application/pdf;base64,AA==',
      },
    })
    expect(host.querySelector('iframe')?.getAttribute('sandbox')).toBeNull()
  })
})

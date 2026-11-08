import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import CardPreview from './CardPreview.vue'

let app: App | null = null
let host: HTMLElement

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
  it('uses a pdf-viewer-compatible sandbox for uploaded pdf previews', () => {
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
    const sandbox = host.querySelector('iframe')?.getAttribute('sandbox') ?? ''
    expect(sandbox).toContain('allow-same-origin')
    expect(sandbox).toContain('allow-scripts')
  })
})

import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import CardProfilePanel from './CardProfilePanel.vue'
import { emptyCard } from '@/utils/card'
import type { BusinessCard, CardProjectItem } from '@/types'

let app: App | null = null
let host: HTMLElement

const item: CardProjectItem = {
  projectId: 'p1',
  name: '作品一',
  description: '',
  coverUrl: null,
  type: 'html',
  shareUrl: '/p/p1',
  addedAt: '2026-11-08T00:00:00.000Z',
}

function mount(card: BusinessCard) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardProfilePanel, { card })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardProfilePanel', () => {
  it('renders the owner, headline and tags', () => {
    mount({ ...emptyCard(), ownerName: 'alice', headline: '前端工程师', tags: ['Vue', 'Go'] })
    expect(host.textContent).toContain('alice')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('Go')
  })

  it('shows the project selector only when items exist', () => {
    mount({ ...emptyCard(), items: [item] })
    expect(host.querySelector('select')?.textContent).toContain('作品一')
    app?.unmount()
    host.remove()
    mount({ ...emptyCard(), items: [] })
    expect(host.querySelector('select')).toBeNull()
  })

  it('renders an uploaded pdf as a file preview', () => {
    mount({
      ...emptyCard(),
      featured: {
        kind: 'file',
        fileType: 'pdf',
        name: 'resume.pdf',
        size: 1024,
        url: 'data:application/pdf;base64,AA==',
      },
    })
    expect(host.textContent).toContain('resume.pdf')
  })
})

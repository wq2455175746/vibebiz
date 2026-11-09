import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import CardTile from './CardTile.vue'
import { emptyCard } from '@/utils/card'
import type { BusinessCard, CardProjectItem } from '@/types'

let app: App | null = null
let host: HTMLElement

const item: CardProjectItem = {
  projectId: 'p1',
  name: '作品一',
  description: '',
  coverUrl:
    'data:image/svg+xml;charset=utf-8,%3Csvg%20xmlns%3D%22http%3A%2F%2Fwww.w3.org%2F2000%2Fsvg%22%2F%3E',
  type: 'html',
  shareUrl: '/p/p1',
  addedAt: '2026-11-08T00:00:00.000Z',
}

function mount(card: BusinessCard, listeners: Record<string, unknown> = {}) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardTile, { card, ...listeners })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardTile', () => {
  it('falls back to the first project preview when no featured is set', () => {
    mount({ ...emptyCard(), id: 'demo-1', ownerName: '林一', items: [item], featured: null })
    expect(host.querySelector('.card-tile-preview img')).not.toBeNull()
  })

  it('renders the personal info module', () => {
    mount({
      ...emptyCard(),
      id: 'demo-1',
      ownerName: '林一',
      headline: '前端工程师',
      tags: ['Vue', '游戏'],
      items: [item],
      featured: { kind: 'project', projectId: 'p1' },
      commentCount: 3,
    })
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
  })

  it('opens the card from personal info and the content from the preview', () => {
    const open = vi.fn()
    const openFeatured = vi.fn()
    mount(
      {
        ...emptyCard(),
        id: 'demo-1',
        ownerName: '林一',
        items: [item],
        featured: { kind: 'project', projectId: 'p1' },
      },
      { onOpen: open, onOpenFeatured: openFeatured },
    )

    host.querySelector<HTMLElement>('.card-tile-person')?.click()
    expect(open).toHaveBeenCalledWith('demo-1')
    expect(openFeatured).not.toHaveBeenCalled()

    host.querySelector<HTMLElement>('.card-tile-preview')?.click()
    expect(openFeatured).toHaveBeenCalledWith('demo-1')
  })
})

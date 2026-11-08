import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'

import CardShareView from './CardShareView.vue'

vi.mock('vue-router', () => ({ useRoute: () => ({ params: { id: 'demo-1' } }) }))

vi.mock('@/stores/card', () => ({
  useCardStore: () => ({
    load: vi.fn(),
    getCardById: () => ({
      id: 'demo-1',
      ownerUserId: null,
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
    }),
  }),
}))

let app: App | null = null
let host: HTMLElement

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardShareView)
  app.use(createPinia())
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('CardShareView', () => {
  it('renders the card profile and projects', () => {
    mount()
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('作品一')
  })

  it('keeps comment and message entries disabled', () => {
    mount()
    expect(host.querySelector<HTMLButtonElement>('.card-social-comment')?.disabled).toBe(true)
    expect(host.querySelector<HTMLButtonElement>('.card-social-message')?.disabled).toBe(true)
  })
})

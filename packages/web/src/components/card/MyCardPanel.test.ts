import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import MyCardPanel from './MyCardPanel.vue'
import { emptyCard } from '@/utils/card'
import type { CardProjectItem } from '@/types'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: { id: 7, username: 'alice', avatarUrl: null },
    ensure: vi.fn().mockResolvedValue(undefined),
  }),
}))

vi.mock('@/api/publish', () => ({ listPublishedProjects: vi.fn().mockResolvedValue([]) }))

function memoryStorage() {
  const values = new Map<string, string>()
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
    clear: () => values.clear(),
  }
}

let app: App | null = null
let host: HTMLElement

async function flush() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(MyCardPanel)
  app.use(createPinia())
  app.mount(host)
}

beforeEach(() => vi.stubGlobal('localStorage', memoryStorage()))
afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('MyCardPanel', () => {
  it('renders the content and profile modules', async () => {
    mount()
    await flush()
    expect(host.textContent).toContain('我的项目')
    expect(host.textContent).toContain('个人信息')
    expect(host.querySelector('.card-content-panel')).not.toBeNull()
    expect(host.querySelector('.card-profile')).not.toBeNull()
  })

  it('disables sharing until a project exists', async () => {
    mount()
    await flush()
    expect(host.querySelector<HTMLButtonElement>('.card-share-btn')?.disabled).toBe(true)
    expect(host.textContent).toContain('请先添加项目')
  })

  it('opens the add project dialog', async () => {
    mount()
    await flush()
    host.querySelector<HTMLButtonElement>('.card-add-btn')?.click()
    await flush()
    expect(host.textContent).toContain('添加已发布项目')
  })

  it('shows remove buttons in the editor project list', async () => {
    const item: CardProjectItem = {
      projectId: 'p1',
      name: '作品一',
      description: '',
      coverUrl: null,
      type: 'html',
      shareUrl: '/p/p1',
      addedAt: '2026-11-08T00:00:00.000Z',
    }
    localStorage.setItem(
      'vibebiz:cards:mine:v1',
      JSON.stringify({ ...emptyCard(), ownerName: 'alice', items: [item] }),
    )
    mount()
    await flush()
    expect(host.querySelectorAll('.card-project-remove')).toHaveLength(1)
  })
})

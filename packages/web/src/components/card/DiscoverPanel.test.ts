import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import DiscoverPanel from './DiscoverPanel.vue'

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ user: { id: 7, username: 'alice', avatarUrl: null } }),
}))

vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))

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
  await new Promise((resolve) => setTimeout(resolve, 350))
  await new Promise((resolve) => setTimeout(resolve, 0))
}

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(DiscoverPanel)
  app.use(createPinia())
  app.mount(host)
}

beforeEach(() => vi.stubGlobal('localStorage', memoryStorage()))
afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('DiscoverPanel', () => {
  it('renders the first page of mock cards', async () => {
    mount()
    await flush()
    expect(host.querySelectorAll('.card-tile')).toHaveLength(6)
    expect(host.textContent).toContain('林一')
  })

  it('loads more cards on click', async () => {
    mount()
    await flush()
    host.querySelector<HTMLButtonElement>('.discover-more')?.click()
    await flush()
    expect(host.querySelectorAll('.card-tile')).toHaveLength(12)
    expect(host.textContent).toContain('已到底')
  })
})

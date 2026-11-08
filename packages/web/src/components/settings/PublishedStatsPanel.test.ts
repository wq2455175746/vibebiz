import { createApp, type App } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'

import type { PublishedStats } from '@/types'
import PublishedStatsPanel from './PublishedStatsPanel.vue'

let app: App | null = null
let host: HTMLElement

function mount(stats: PublishedStats | null, loading = false, error = '') {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(PublishedStatsPanel, { stats, loading, error })
  app.mount(host)
}

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('PublishedStatsPanel', () => {
  it('renders totals and project rows', () => {
    mount({
      exposure: 9,
      clicks: 3,
      dwellSeconds: 0,
      spend: 0,
      projects: [
        {
          projectId: 'p1',
          name: '小游戏',
          exposure: 9,
          clicks: 3,
          dwellSeconds: 0,
          spend: 0,
        },
      ],
    })
    expect(host.textContent).toContain('曝光量')
    expect(host.textContent).toContain('9')
    expect(host.textContent).toContain('小游戏')
    expect(host.textContent).toContain('0 秒')
  })

  it('shows the empty state', () => {
    mount({ exposure: 0, clicks: 0, dwellSeconds: 0, spend: 0, projects: [] })
    expect(host.textContent).toContain('暂无发布数据')
  })

  it('shows loading and error states', () => {
    mount(null, true)
    expect(host.textContent).toContain('加载中')
    app?.unmount()
    host.remove()
    mount(null, false, '加载失败')
    expect(host.textContent).toContain('加载失败')
  })
})

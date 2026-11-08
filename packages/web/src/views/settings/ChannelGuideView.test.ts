import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import ChannelGuideView from './ChannelGuideView.vue'
import { listChannelGuides } from '@/api/channelPackages'
import type { ChannelGuide } from '@/types'

const routeState = vi.hoisted(() => ({ query: {} as Record<string, unknown> }))

vi.mock('vue-router', () => ({ useRoute: () => routeState }))
vi.mock('@/api/channelPackages', () => ({ listChannelGuides: vi.fn() }))

const listMock = vi.mocked(listChannelGuides)

const guides: ChannelGuide[] = [
  {
    channel: 'wechat_minigame',
    title: '微信小游戏',
    summary: '企业主体、软著、类目审核后导入微信开发者工具',
    markdown: '# 微信小游戏\n\n需要企业主体。',
    steps: ['生成渠道包', '注册小游戏 AppID', '导入微信开发者工具', '真机调试', '上传提审'],
    requirements: ['企业主体', '软件著作权', '对应类目资质'],
    updatedAt: '2026-11-07',
  },
  {
    channel: 'douyin_minigame',
    title: '抖音小游戏',
    summary: '企业主体、软著、类目资质后导入抖音开发者工具',
    markdown: '# 抖音小游戏',
    steps: ['生成渠道包', '注册小游戏 AppID', '导入抖音开发者工具', '真机调试', '上传提审'],
    requirements: ['企业主体', '软件著作权'],
    updatedAt: '2026-11-07',
  },
  {
    channel: 'steam_desktop',
    title: 'Steam 桌面',
    summary: 'Steamworks 账号、App 费、商店素材和 SteamPipe 上传',
    markdown: '# Steam 桌面\n\n需要 Steamworks 账号。',
    steps: ['生成渠道包', '注册 Steamworks', '本地构建 Electron 壳', '配置 Depot 和 VDF', 'SteamPipe 上传并提审'],
    requirements: ['Steamworks 账号', '100 美元 App 费', '商店页素材'],
    updatedAt: '2026-11-07',
  },
]

let app: App | null = null
let host: HTMLElement

function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(ChannelGuideView)
  app.mount(host)
}

async function flush() {
  for (let index = 0; index < 4; index += 1) {
    await Promise.resolve()
  }
  await new Promise((resolve) => setTimeout(resolve, 0))
  for (let index = 0; index < 4; index += 1) {
    await Promise.resolve()
  }
}

beforeEach(() => {
  listMock.mockReset()
  routeState.query = {}
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
})

describe('ChannelGuideView', () => {
  it('loads guides and selects the channel from the route query', async () => {
    listMock.mockResolvedValue(guides)
    routeState.query = { channel: 'steam_desktop' }

    mount()
    await flush()

    expect(listMock).toHaveBeenCalledTimes(1)
    expect(host.querySelector('.channel-guide-panel h2')?.textContent).toContain('Steam 桌面')
    expect(host.querySelector('.channel-guide-tab.active')?.textContent).toContain('Steam 桌面')
    expect(host.querySelectorAll('.channel-guide-step')).toHaveLength(5)
    expect(host.querySelectorAll('.channel-guide-requirements li')).toHaveLength(3)
    expect(host.textContent).toContain('100 美元 App 费')
  })

  it('falls back to the first guide when the query channel is unknown', async () => {
    listMock.mockResolvedValue(guides)
    routeState.query = { channel: 'unknown_channel' }

    mount()
    await flush()

    expect(host.querySelector('.channel-guide-tab.active')?.textContent).toContain('微信小游戏')
    expect(host.querySelector('.channel-guide-panel h2')?.textContent).toContain('微信小游戏')
  })

  it('switches the selected guide when a channel tab is clicked', async () => {
    listMock.mockResolvedValue(guides)

    mount()
    await flush()

    const tabs = host.querySelectorAll<HTMLButtonElement>('.channel-guide-tab')
    tabs[2].click()
    await flush()

    expect(host.querySelector('.channel-guide-panel h2')?.textContent).toContain('Steam 桌面')
  })

  it('renders the guide markdown', async () => {
    listMock.mockResolvedValue(guides)

    mount()
    await flush()

    expect(host.querySelector('.channel-guide-markdown')?.textContent).toContain('需要企业主体')
  })

  it('shows an error state when loading fails', async () => {
    listMock.mockRejectedValue(new Error('指引加载失败'))

    mount()
    await flush()

    expect(host.querySelector('.settings-error')?.textContent).toContain('指引加载失败')
  })

  it('shows an empty state when no guides exist', async () => {
    listMock.mockResolvedValue([])

    mount()
    await flush()

    expect(host.querySelector('.channel-guide-empty')).not.toBeNull()
  })
})

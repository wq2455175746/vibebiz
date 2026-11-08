import { describe, expect, it } from 'vitest'

import type { ChannelGuide } from '@/types'
import { channelLabel, channelOptions, levelLabel, packageableChannelOptions } from './channelPackages'

describe('channel package helpers', () => {
  it('exposes three channels in display order', () => {
    expect(channelOptions.map((item) => item.value)).toEqual([
      'wechat_minigame',
      'douyin_minigame',
      'steam_desktop',
    ])
    expect(channelOptions.map((item) => item.label)).toEqual([
      '微信小游戏',
      '抖音小游戏',
      'Steam 桌面',
    ])
  })

  it('maps levels and labels', () => {
    expect(channelLabel('wechat_minigame')).toBe('微信小游戏')
    expect(levelLabel('A')).toContain('直接兼容')
    expect(levelLabel('C')).toContain('需要改造')
  })

  it('returns no level label when the backend omits the level', () => {
    expect(levelLabel('')).toBe('')
  })

  it('filters packageable channels and falls back to builtins', () => {
    const guides: ChannelGuide[] = [
      {
        channel: 'wechat_minigame',
        title: '微信小游戏',
        summary: '',
        markdown: '',
        steps: [],
        requirements: [],
        updatedAt: '2026-11-08',
        enabled: true,
        builtin: true,
        packageable: true,
        sortOrder: 10,
      },
      {
        channel: 'taptap_mobile',
        title: 'TapTap',
        summary: '',
        markdown: '',
        steps: [],
        requirements: [],
        updatedAt: '2026-11-08',
        enabled: true,
        builtin: false,
        packageable: false,
        sortOrder: 40,
      },
    ]

    expect(packageableChannelOptions(guides).map((item) => item.value)).toEqual(['wechat_minigame'])
    expect(packageableChannelOptions([]).map((item) => item.value)).toEqual([
      'wechat_minigame',
      'douyin_minigame',
      'steam_desktop',
    ])
  })
})

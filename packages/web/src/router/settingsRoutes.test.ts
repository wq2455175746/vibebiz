import { describe, expect, it } from 'vitest'

import { settingsMenu, settingsRoute } from './settingsRoutes'

describe('settings routes', () => {
  it('mounts the settings pages under /settings', () => {
    expect(settingsRoute.path).toBe('/settings')
    const children = settingsRoute.children ?? []
    expect(children.map((child) => child.path)).toEqual([
      '',
      'profile',
      'notification-channels',
      'channel-guides',
      'artifact-space',
      'delivery-space',
      'storage',
      'workspace',
      'members',
    ])
    expect(children.map((child) => child.name)).toEqual([
      'settings',
      'settings-profile',
      'settings-notification-channels',
      'settings-channel-guides',
      'settings-artifact-space',
      'settings-delivery-space',
      'settings-storage',
      'settings-workspace',
      'settings-members',
    ])
  })

  it('exposes the left menu entries in display order', () => {
    expect(settingsMenu.map((item) => item.label)).toEqual([
      '个人信息',
      '通知渠道',
      '发布渠道指引',
      '制品空间配置',
      '交付空间配置',
      '磁盘空间',
      '工作区设置',
      '成员',
    ])
    expect(settingsMenu.map((item) => item.to)).toEqual([
      '/settings/profile',
      '/settings/notification-channels',
      '/settings/channel-guides',
      '/settings/artifact-space',
      '/settings/delivery-space',
      '/settings/storage',
      '/settings/workspace',
      '/settings/members',
    ])
  })

  it('places the channel guide entry in the enterprise group', () => {
    const item = settingsMenu.find((entry) => entry.name === 'settings-channel-guides')
    expect(item).toMatchObject({
      label: '发布渠道指引',
      icon: 'pi pi-share-alt',
      to: '/settings/channel-guides',
      group: '企业工程',
    })
  })
})

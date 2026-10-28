import { describe, expect, it } from 'vitest'

import { settingsMenu, settingsRoute } from './settingsRoutes'

describe('settings routes', () => {
  it('mounts the four settings pages under /settings', () => {
    expect(settingsRoute.path).toBe('/settings')
    const children = settingsRoute.children ?? []
    expect(children.map((child) => child.path)).toEqual([
      '',
      'profile',
      'storage',
      'workspace',
      'members',
    ])
    expect(children.map((child) => child.name)).toEqual([
      'settings',
      'settings-profile',
      'settings-storage',
      'settings-workspace',
      'settings-members',
    ])
  })

  it('exposes the left menu entries in display order', () => {
    expect(settingsMenu.map((item) => item.label)).toEqual([
      '个人信息',
      '磁盘空间',
      '工作区设置',
      '成员',
    ])
    expect(settingsMenu.map((item) => item.to)).toEqual([
      '/settings/profile',
      '/settings/storage',
      '/settings/workspace',
      '/settings/members',
    ])
  })
})

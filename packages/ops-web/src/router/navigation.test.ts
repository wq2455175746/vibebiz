import { describe, expect, it } from 'vitest'

import router from './index'
import { navItems } from './navigation'
import { childRoutes } from './routes'

describe('ops navigation', () => {
  it('exposes the admin modules', () => {
    expect(navItems.map((item) => item.path)).toEqual([
      '/',
      '/users',
      '/projects',
      '/cards',
      '/leaderboard',
      '/evolution',
      '/sandboxes',
      '/models',
      '/skills',
      '/roles',
      '/fde-resources',
      '/topics',
      '/templates',
      '/channel-guides',
      '/scheduled-tasks',
      '/quotas',
      '/audit',
      '/access-logs',
      '/notifications',
      '/settings',
    ])
    expect(navItems.map((item) => item.label)).toEqual([
      '概览',
      '用户',
      '项目',
      '名片管理',
      '排行榜',
      '集思进化',
      '沙箱节点',
      '模型管理',
      '技能',
      '角色管理',
      'FDE 资源',
      '类别管理',
      '模板管理',
      '发布渠道',
      '定时任务',
      '配额',
      '审计日志',
      '访问日志',
      '消息通知',
      '系统设置',
    ])
  })

  it('has a route for every navigation item', () => {
    const routeNames = childRoutes.map((route) => route.name)
    expect(routeNames).toEqual(navItems.map((item) => item.name))
  })

  it('resolves every navigation path to a layout child route', () => {
    for (const item of navItems) {
      expect(router.resolve(item.path).matched.length, item.path).toBeGreaterThan(1)
    }
  })

  it('serves the login route outside the ops layout and marks it public', () => {
    const login = router.resolve('/login')
    expect(login.meta.public).toBe(true)
    expect(login.matched).toHaveLength(1)
    expect(router.resolve('/').matched.length).toBeGreaterThan(1)
  })
})

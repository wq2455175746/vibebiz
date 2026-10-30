import { describe, expect, it } from 'vitest'

import router from './index'
import { navItems } from './navigation'
import { childRoutes } from './routes'

describe('ops navigation', () => {
  it('exposes the twelve admin modules', () => {
    expect(navItems.map((item) => item.path)).toEqual([
      '/',
      '/users',
      '/projects',
      '/sandboxes',
      '/models',
      '/roles',
      '/topics',
      '/templates',
      '/quotas',
      '/audit',
      '/notifications',
      '/settings',
    ])
    expect(navItems.map((item) => item.label)).toEqual([
      '概览',
      '用户',
      '项目',
      '沙箱节点',
      '模型管理',
      '角色管理',
      '类别管理',
      '模板管理',
      '配额',
      '审计日志',
      '消息通知',
      '系统设置',
    ])
  })

  it('has a route for every navigation item', () => {
    const routeNames = childRoutes.map((route) => route.name)
    expect(routeNames).toEqual(navItems.map((item) => item.name))
  })

  it('serves the login route outside the ops layout and marks it public', () => {
    const login = router.resolve('/login')
    expect(login.meta.public).toBe(true)
    expect(login.matched).toHaveLength(1)
    expect(router.resolve('/').matched.length).toBeGreaterThan(1)
  })
})

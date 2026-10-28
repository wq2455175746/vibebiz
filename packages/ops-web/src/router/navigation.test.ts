import { describe, expect, it } from 'vitest'

import { navItems } from './navigation'
import { childRoutes } from './routes'

describe('ops navigation', () => {
  it('exposes the seven admin modules', () => {
    expect(navItems.map((item) => item.path)).toEqual([
      '/',
      '/users',
      '/projects',
      '/sandboxes',
      '/quotas',
      '/audit',
      '/settings',
    ])
    expect(navItems.map((item) => item.label)).toEqual([
      '概览',
      '用户',
      '项目',
      '沙箱节点',
      '配额',
      '审计日志',
      '系统设置',
    ])
  })

  it('has a route for every navigation item', () => {
    const routeNames = childRoutes.map((route) => route.name)
    expect(routeNames).toEqual(navItems.map((item) => item.name))
  })
})

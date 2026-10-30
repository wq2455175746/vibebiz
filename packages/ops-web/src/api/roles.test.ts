import { describe, expect, it, vi } from 'vitest'

import { getRoles, updateRole } from './admin'
import { http } from './client'

describe('role admin api', () => {
  it('unwraps the paginated roles envelope', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: {
        code: 0,
        message: 'ok',
        result: { roles: [{ id: 1, roleCode: 'alex' }], total: 1, page: 1, pageSize: 10 },
      },
    })
    await expect(getRoles({ page: 1, pageSize: 10, keyword: 'alex' })).resolves.toEqual({
      items: [{ id: 1, roleCode: 'alex' }],
      total: 1,
      page: 1,
      pageSize: 10,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'get',
      url: '/roles',
      params: { page: 1, pageSize: 10, keyword: 'alex' },
    })
  })

  it('updates a role', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: 1, status: 0 } },
    })
    const input = { description: 'd', systemPrompt: 'p', status: 0 }
    await expect(updateRole(1, input)).resolves.toEqual({ id: 1, status: 0 })
    expect(spy).toHaveBeenCalledWith({ method: 'put', url: '/roles/1', data: input })
  })
})

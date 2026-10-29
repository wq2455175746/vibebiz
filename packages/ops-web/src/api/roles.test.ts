import { describe, expect, it, vi } from 'vitest'

import { getRoles } from './admin'
import { http } from './client'

describe('role admin api', () => {
  it('unwraps the roles envelope', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { roles: [{ id: 1, roleCode: 'alex' }] } },
    })
    await expect(getRoles()).resolves.toEqual([{ id: 1, roleCode: 'alex' }])
  })
})

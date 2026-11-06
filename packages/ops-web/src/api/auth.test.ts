import { describe, expect, it, vi } from 'vitest'

import { changePassword } from './auth'
import { http } from './client'

describe('auth api', () => {
  it('changes the current ops password', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { changed: true } },
    })

    await expect(changePassword('old-password', 'new-password')).resolves.toEqual({
      changed: true,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/auth/password',
      data: { oldPassword: 'old-password', newPassword: 'new-password' },
    })
  })
})

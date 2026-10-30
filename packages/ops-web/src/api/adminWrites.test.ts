import { describe, expect, it, vi } from 'vitest'

import { updateQuota, updateUserStatus } from './admin'
import { http } from './client'

describe('admin writes', () => {
  it('updates a user status', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: '1', status: 0 } },
    })
    await updateUserStatus('1', 0)
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/users/1/status',
      data: { status: 0 },
    })
  })

  it('updates a user quota', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { userId: '1' } },
    })
    const input = { llmTokensQuota: 100, sandboxMinutesQuota: 200, storageQuotaMb: 300 }
    await updateQuota('1', input)
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/quotas/1',
      data: input,
    })
  })
})

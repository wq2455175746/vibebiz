import { describe, expect, it, vi } from 'vitest'

import { getModels } from './admin'
import { http } from './client'

describe('model admin api', () => {
  it('unwraps the models envelope', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { models: [{ id: 1, modelCode: 'x' }] } },
    })
    await expect(getModels()).resolves.toEqual([{ id: 1, modelCode: 'x' }])
  })
})

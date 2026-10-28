import { describe, expect, it, vi } from 'vitest'

import { ApiError, http, onUnauthorized, request, unwrapEnvelope } from './client'

describe('unwrapEnvelope', () => {
  it('returns result when code is 0', () => {
    expect(unwrapEnvelope({ code: 0, message: 'ok', result: { projects: 3 } })).toEqual({
      projects: 3,
    })
  })

  it('throws ApiError when code is not 0', () => {
    expect(() => unwrapEnvelope({ code: 40301, message: 'forbidden', result: null })).toThrow(
      ApiError,
    )
  })

  it('preserves the response code and message', () => {
    try {
      unwrapEnvelope({ code: 50001, message: 'internal error', result: null })
    } catch (error) {
      expect(error).toBeInstanceOf(ApiError)
      expect((error as ApiError).code).toBe(50001)
      expect((error as ApiError).message).toBe('internal error')
    }
  })
})

describe('request', () => {
  it('notifies the unauthorized handler on a 401 response', async () => {
    const handler = vi.fn()
    onUnauthorized(handler)
    const error = Object.assign(new Error('unauthorized'), {
      isAxiosError: true,
      response: {
        status: 401,
        data: { code: 40101, message: 'unauthorized', result: null },
      },
    })
    vi.spyOn(http, 'request').mockRejectedValueOnce(error)

    await expect(request({ method: 'get', url: '/auth/me' })).rejects.toBeInstanceOf(ApiError)
    expect(handler).toHaveBeenCalledTimes(1)

    onUnauthorized(null)
  })
})

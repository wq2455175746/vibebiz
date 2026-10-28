import { describe, expect, it } from 'vitest'

import { ApiError, unwrapEnvelope } from './client'

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

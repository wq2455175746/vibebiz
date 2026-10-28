import axios from 'axios'
import { describe, expect, it, vi } from 'vitest'

import { ApiError, cpHttp, dpHttp, onUnauthorized, request, unwrapEnvelope } from './client'

describe('unwrapEnvelope', () => {
  it('returns result when code is 0', () => {
    expect(unwrapEnvelope({ code: 0, message: 'ok', result: { id: 'p1' } })).toEqual({ id: 'p1' })
  })

  it('throws ApiError when code is not 0', () => {
    expect(() =>
      unwrapEnvelope({ code: 40401, message: 'project not found', result: null }),
    ).toThrow(ApiError)
  })
})

describe('http clients', () => {
  it('uses the CP prefix for control-plane requests', () => {
    expect(cpHttp.defaults.baseURL).toMatch(/\/cp\/api\/v1\/?$/)
  })

  it('uses the DP prefix for data-plane requests', () => {
    expect(dpHttp.defaults.baseURL).toMatch(/\/dp\/api\/v1\/?$/)
  })

  it('sends cookies for CP and DP requests', () => {
    expect(cpHttp.defaults.withCredentials).toBe(true)
    expect(dpHttp.defaults.withCredentials).toBe(true)
  })
})

describe('unauthorized hook', () => {
  it('calls the registered handler on a 401 response', async () => {
    const handler = vi.fn()
    const dispose = onUnauthorized(handler)
    const client = axios.create({
      baseURL: 'http://test.local/cp/api/v1',
      adapter: async (config) => {
        throw Object.assign(new Error('unauthorized'), {
          isAxiosError: true,
          config,
          response: {
            data: { code: 40101, message: 'unauthorized' },
            status: 401,
            statusText: 'Unauthorized',
            headers: {},
            config,
          },
        })
      },
    })

    await expect(request(client, { method: 'get', url: '/auth/me' })).rejects.toBeInstanceOf(
      ApiError,
    )
    expect(handler).toHaveBeenCalledTimes(1)

    dispose()
  })
})

describe('request', () => {
  it('uses the provided client and unwraps the envelope', async () => {
    const client = axios.create({
      baseURL: 'http://test.local/dp/api/v1',
      adapter: async (config) => ({
        data: { code: 0, message: 'ok', result: { id: 'p1' } },
        status: 200,
        statusText: 'OK',
        headers: {},
        config,
      }),
    })

    await expect(request(client, { method: 'get', url: '/projects/p1' })).resolves.toEqual({
      id: 'p1',
    })
  })

  it('maps an axios failure to ApiError with status and code', async () => {
    const client = axios.create({
      baseURL: 'http://test.local/cp/api/v1',
      adapter: async (config) => {
        throw Object.assign(new Error('request failed'), {
          isAxiosError: true,
          config,
          response: {
            data: { code: 40401, message: 'project not found' },
            status: 404,
            statusText: 'Not Found',
            headers: {},
            config,
          },
        })
      },
    })

    await expect(
      request(client, { method: 'get', url: '/projects/missing' }),
    ).rejects.toMatchObject({
      name: 'ApiError',
      message: 'project not found',
      status: 404,
      code: 40401,
    })
  })
})

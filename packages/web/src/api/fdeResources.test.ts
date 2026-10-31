import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { listFdeResources } from './fdeResources'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({ resources: [] })
})

describe('fde resources api', () => {
  it('unwraps the resources envelope', async () => {
    await listFdeResources()
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/fde-resources',
    })
  })
})

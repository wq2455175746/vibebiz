import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { createProject } from './projects'

vi.mock('./client', () => ({
  cpHttp: {},
  dpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({})
})

describe('projects api', () => {
  it('creates a standard project without a mode field', async () => {
    await createProject('small app', [2])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects',
      data: { prompt: 'small app', agentRoleIds: [2] },
    })
  })

  it('creates an enterprise project with an explicit mode', async () => {
    await createProject('enterprise app', [6], 'enterprise')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects',
      data: { prompt: 'enterprise app', agentRoleIds: [6], mode: 'enterprise' },
    })
  })
})

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { publishProject } from './publish'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({ url: '', slug: '', publishedAt: '' })
})

describe('publish api', () => {
  it('publishes a project with card metadata', async () => {
    await publishProject('p1', { title: '小游戏', description: '介绍', tags: ['答题'] })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects/p1/publish',
      data: { title: '小游戏', description: '介绍', tags: ['答题'] },
    })
  })
})

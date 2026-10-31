import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import { deletePublishedProject, listPublishedProjects, publishProject } from './publish'

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
  it('lists published projects for the current user', async () => {
    requestMock.mockResolvedValue({
      projects: [
        {
          projectId: 'p1',
          name: '小游戏',
          description: '',
          tags: ['答题'],
          coverUrl: null,
          publishedAt: '2026-10-31T10:00:00Z',
          url: 'https://play.example.com/p/p1/',
        },
      ],
    })
    await expect(listPublishedProjects()).resolves.toEqual([
      expect.objectContaining({ projectId: 'p1', url: 'https://play.example.com/p/p1/' }),
    ])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/me/published-projects',
    })
  })

  it('deletes a published project without deleting the project', async () => {
    await deletePublishedProject('p1')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'delete',
      url: '/projects/p1/publish',
    })
  })

  it('publishes a project with card metadata', async () => {
    await publishProject('p1', { title: '小游戏', description: '介绍', tags: ['答题'] })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects/p1/publish',
      data: { title: '小游戏', description: '介绍', tags: ['答题'] },
    })
  })
})

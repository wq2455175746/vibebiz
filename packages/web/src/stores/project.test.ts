import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { getProject } from '@/api/projects'
import { useProjectStore } from './project'

vi.mock('@/api/projects', () => ({
  getProject: vi.fn(),
}))

const getProjectMock = vi.mocked(getProject)

beforeEach(() => {
  setActivePinia(createPinia())
  getProjectMock.mockReset()
})

describe('project store cover', () => {
  it('updates the current project after cover upload', async () => {
    getProjectMock.mockResolvedValue({
      id: 'p1',
      name: 'demo',
      prompt: '',
      state: 'ready',
      createdAt: '2026-10-30T00:00:00Z',
      updatedAt: '2026-10-30T00:00:00Z',
      coverUrl: null,
    })
    const project = useProjectStore()
    await project.load('p1')

    project.setCover('https://s3.example/projects/p1/cover.png', 'projects/p1/cover.png')

    expect(project.current?.coverUrl).toBe('https://s3.example/projects/p1/cover.png')
    expect(project.current?.coverS3Path).toBe('projects/p1/cover.png')
  })
})

import { describe, expect, it } from 'vitest'

import { publishedToProject } from './publish'

describe('publishedToProject', () => {
  it('maps published metadata to the project card shape', () => {
    expect(
      publishedToProject({
        projectId: 'p1',
        name: '小游戏',
        description: '介绍',
        tags: ['答题'],
        coverUrl: 'https://cdn.example/p1.png',
        publishedAt: '2026-10-31T10:00:00Z',
        url: 'https://play.example.com/p/p1/',
      }),
    ).toEqual({
      id: 'p1',
      name: '小游戏',
      prompt: '介绍',
      previewPath: '',
      state: 'ready',
      createdAt: '2026-10-31T10:00:00Z',
      updatedAt: '2026-10-31T10:00:00Z',
      coverUrl: 'https://cdn.example/p1.png',
    })
  })
})

import type { Project, PublishedProject } from '@/types'

export function publishedToProject(item: PublishedProject): Project {
  return {
    id: item.projectId,
    name: item.name,
    prompt: item.description,
    previewPath: '',
    state: 'ready',
    createdAt: item.publishedAt,
    updatedAt: item.publishedAt,
    coverUrl: item.coverUrl,
  }
}

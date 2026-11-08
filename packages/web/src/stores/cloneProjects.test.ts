import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import * as api from '@/api/clones'
import type { CloneDraftAsset, CloneProject } from '@/types/clone'
import { useCloneProjectsStore } from './cloneProjects'

vi.mock('@/api/clones', () => ({
  listClones: vi.fn(),
  getClone: vi.fn(),
  createClone: vi.fn(),
  deleteClone: vi.fn(),
  addCloneAsset: vi.fn(),
  deleteCloneAsset: vi.fn(),
  setCloneCover: vi.fn(),
  stageCloneFile: vi.fn(),
}))

function project(overrides: Partial<CloneProject> = {}): CloneProject {
  return {
    id: 'c1',
    name: 'Acme',
    description: 'clone acme',
    sourceUrl: 'https://acme.example.com',
    mode: 'clone',
    state: 'ready',
    workspaceId: 1,
    coverS3Path: '',
    coverUrl: '',
    assets: [],
    createdAt: '2026-10-31T00:00:00Z',
    updatedAt: '2026-10-31T00:00:00Z',
    ...overrides,
  }
}

function image(id: string): CloneDraftAsset {
  return { id, name: `${id}.png`, size: 100, mime: 'image/png', role: 'image', file: new File(['x'], `${id}.png`, { type: 'image/png' }) }
}

describe('useCloneProjectsStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads clones from the API', async () => {
    vi.mocked(api.listClones).mockResolvedValue([project()])
    const store = useCloneProjectsStore()
    await store.load()
    expect(store.projects).toHaveLength(1)
    expect(store.projects[0].id).toBe('c1')
  })

  it('uploads draft assets and creates the clone with cover and gallery roles', async () => {
    vi.mocked(api.stageCloneFile)
      .mockResolvedValueOnce('r1')
      .mockResolvedValueOnce('r2')
      .mockResolvedValueOnce('rdoc')
    vi.mocked(api.createClone).mockResolvedValue(project())
    const store = useCloneProjectsStore()

    const created = await store.create(
      { name: 'Acme', description: 'clone acme', sourceUrl: 'https://acme.example.com' },
      [image('a'), image('b'), { id: 'doc', name: 'spec.pdf', size: 10, mime: 'application/pdf', role: 'docs', file: new File(['x'], 'spec.pdf') }],
      'b',
    )

    expect(created.id).toBe('c1')
    expect(api.createClone).toHaveBeenCalledWith({
      name: 'Acme',
      description: 'clone acme',
      sourceUrl: 'https://acme.example.com',
      assets: { coverResourceId: 'r2', galleryResourceIds: ['r1'], docsResourceId: 'rdoc' },
    })
    expect(store.current?.id).toBe('c1')
  })

  it('adds an image as gallery and promotes it to cover when none exists', async () => {
    vi.mocked(api.getClone).mockResolvedValue(project({ id: 'c1', assets: [] }))
    vi.mocked(api.stageCloneFile).mockResolvedValue('r1')
    vi.mocked(api.addCloneAsset).mockResolvedValue({
      id: 'r1', resourceId: 'r1', name: 'a.png', displayName: 'a.png', path: '.resources/a.png', contentType: 'image/png', size: 100, kind: 'image', role: 'gallery', sortOrder: 0, createdAt: '',
    })
    vi.mocked(api.setCloneCover).mockResolvedValue({
      id: 'r1', resourceId: 'r1', name: 'a.png', displayName: 'a.png', path: '.resources/a.png', contentType: 'image/png', size: 100, kind: 'image', role: 'cover', sortOrder: 0, createdAt: '',
    })
    vi.mocked(api.getClone)
      .mockResolvedValueOnce(project({ id: 'c1', assets: [] }))
      .mockResolvedValueOnce(project({ id: 'c1', coverUrl: 'https://cdn/cover' }))
    const store = useCloneProjectsStore()
    await store.loadOne('c1')

    await store.addAsset('c1', image('a').file!, 'image')

    expect(api.addCloneAsset).toHaveBeenCalledWith('c1', 'r1', 'gallery')
    expect(api.setCloneCover).toHaveBeenCalledWith('c1', 'r1')
    expect(store.current?.coverUrl).toBe('https://cdn/cover')
  })

  it('removes projects through the API', async () => {
    vi.mocked(api.listClones).mockResolvedValue([project()])
    vi.mocked(api.deleteClone).mockResolvedValue(undefined)
    const store = useCloneProjectsStore()
    await store.load()

    await store.remove('c1')

    expect(api.deleteClone).toHaveBeenCalledWith('c1')
    expect(store.projects).toHaveLength(0)
  })
})

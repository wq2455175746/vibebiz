import { describe, expect, it } from 'vitest'

import type { Project } from '@/types'
import { filterStandardProjects, resolveProjectTarget } from './projectRoute'

const storage = { getItem: () => null }

function project(overrides: Partial<Project> = {}): Project {
  return {
    id: 'p1',
    name: '项目',
    prompt: '',
    previewPath: '',
    state: 'ready',
    createdAt: '2026-11-06T00:00:00Z',
    updatedAt: '2026-11-06T00:00:00Z',
    ...overrides,
  }
}

describe('resolveProjectTarget', () => {
  it('routes clone projects to the clones menu', () => {
    expect(resolveProjectTarget(project({ mode: 'clone' }), new Set(), storage)).toEqual({
      name: 'clones',
    })
  })

  it('routes scheduled projects to the scheduled tasks menu', () => {
    expect(resolveProjectTarget(project(), new Set(['p1']), storage)).toEqual({
      name: 'scheduled-tasks',
    })
  })

  it('routes enterprise projects to the enterprise editor', () => {
    expect(resolveProjectTarget(project({ mode: 'enterprise' }), new Set(), storage)).toEqual({
      name: 'enterprise-editor',
      params: { id: 'p1' },
    })
  })

  it('routes standard projects to the editor', () => {
    expect(resolveProjectTarget(project(), new Set(), storage)).toEqual({
      name: 'editor',
      params: { id: 'p1' },
    })
  })
})

describe('filterStandardProjects', () => {
  it('keeps only regular projects outside the scheduled task set', () => {
    const projects = [
      project({ id: 'standard' }),
      project({ id: 'enterprise', mode: 'enterprise' }),
      project({ id: 'clone', mode: 'clone' }),
      project({ id: 'scheduled' }),
    ]
    expect(
      filterStandardProjects(projects, new Set(['scheduled']), storage).map((item) => item.id),
    ).toEqual(['standard'])
  })
})

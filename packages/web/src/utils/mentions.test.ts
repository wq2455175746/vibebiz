import { describe, expect, it } from 'vitest'

import { applyMention, collectReferences, findActiveMention, mergeReferences } from './mentions'
import type { FileEntry, ProjectResource } from '@/types'

const resource: ProjectResource = {
  id: 'r1',
  name: '需求.pdf',
  displayName: '需求.pdf',
  storageName: 'r1.pdf',
  path: '.resources/r1.pdf',
  contentType: 'application/pdf',
  size: 12,
  kind: 'document',
  createdAt: '2026-10-29T00:00:00Z',
}

const files: FileEntry[] = [{ path: 'src/app.ts', type: 'file', size: 1, mtime: 1 }]

describe('mentions', () => {
  it('finds and replaces the active @ token', () => {
    const match = findActiveMention('参考 @需求', 6)
    expect(match).toEqual({ start: 3, end: 6, query: '需求' })
    expect(applyMention('参考 @需求', match!, '需求.pdf')).toEqual({
      value: '参考 @需求.pdf ',
      cursor: 11,
    })
  })

  it('resolves resources and project files', () => {
    const refs = collectReferences('看 @需求.pdf 和 @src/app.ts', [resource], files)
    expect(refs).toEqual([
      {
        kind: 'resource',
        id: 'r1',
        name: '需求.pdf',
        path: '.resources/r1.pdf',
        contentType: 'application/pdf',
        resourceKind: 'document',
      },
      { kind: 'project_file', name: 'app.ts', path: 'src/app.ts' },
    ])
  })

  it('deduplicates mixed references', () => {
    expect(
      mergeReferences([
        { kind: 'resource', id: 'r1' },
        { kind: 'resource', id: 'r1' },
        { kind: 'project_file', path: 'src/app.ts' },
      ]),
    ).toHaveLength(2)
  })
})

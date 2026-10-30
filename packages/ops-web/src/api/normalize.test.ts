import { describe, expect, it } from 'vitest'

import { toEntity, toList, toPage } from './normalize'

describe('toList', () => {
  it('keeps a bare list response', () => {
    expect(toList([{ id: 'u_01' }], 'users')).toEqual([{ id: 'u_01' }])
  })

  it('unwraps a keyed list response', () => {
    expect(toList({ users: [{ id: 'u_02' }] }, 'users')).toEqual([{ id: 'u_02' }])
  })

  it('returns an empty list for an unknown shape', () => {
    expect(toList({}, 'users')).toEqual([])
  })
})

describe('toPage', () => {
  it('unwraps a paginated response', () => {
    expect(toPage({ users: [{ id: 'u_01' }], total: 21, page: 2, pageSize: 10 }, 'users')).toEqual({
      items: [{ id: 'u_01' }],
      total: 21,
      page: 2,
      pageSize: 10,
    })
  })

  it('keeps a bare list response', () => {
    expect(toPage([{ id: 'u_02' }], 'users')).toEqual({
      items: [{ id: 'u_02' }],
      total: 1,
      page: 1,
      pageSize: 1,
    })
  })
})

describe('toEntity', () => {
  it('keeps a bare entity response', () => {
    expect(toEntity({ projects: 3, users: 2 }, 'overview')).toEqual({ projects: 3, users: 2 })
  })

  it('unwraps a keyed entity response', () => {
    expect(toEntity({ overview: { projects: 4, users: 2 } }, 'overview')).toEqual({
      projects: 4,
      users: 2,
    })
  })
})

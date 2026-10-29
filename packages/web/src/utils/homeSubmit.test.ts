import { describe, expect, it } from 'vitest'

import { canAppendToProject, resolveHomeSubmit } from './homeSubmit'

describe('homeSubmit', () => {
  it('creates when no target project is selected', () => {
    expect(resolveHomeSubmit(null)).toEqual({ mode: 'create' })
  })

  it('appends when a target project is selected', () => {
    expect(resolveHomeSubmit('p1')).toEqual({ mode: 'append', projectId: 'p1' })
  })

  it('rejects appending while the target is running', () => {
    expect(canAppendToProject(true)).toBe(false)
    expect(canAppendToProject(false)).toBe(true)
  })
})

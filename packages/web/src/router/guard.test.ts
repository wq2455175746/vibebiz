import { describe, expect, it } from 'vitest'

import { sanitizeRedirect } from './redirect'

describe('sanitizeRedirect', () => {
  it('keeps internal absolute paths', () => {
    expect(sanitizeRedirect('/editor/42')).toBe('/editor/42')
  })

  it('falls back when the redirect is missing', () => {
    expect(sanitizeRedirect(undefined)).toBe('/')
  })

  it('rejects external URLs', () => {
    expect(sanitizeRedirect('https://evil.example/steal')).toBe('/')
    expect(sanitizeRedirect('//evil.example/steal')).toBe('/')
    expect(sanitizeRedirect('/\\evil.example/steal')).toBe('/')
  })

  it('ignores repeated query values', () => {
    expect(sanitizeRedirect(['/editor/1', '/editor/2'])).toBe('/')
  })
})

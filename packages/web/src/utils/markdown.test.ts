import { describe, expect, it } from 'vitest'

import { renderMarkdown } from './markdown'

describe('renderMarkdown', () => {
  it('removes script tags', () => {
    const html = renderMarkdown('<script>alert(1)</script>')

    expect(html).not.toContain('<script')
    expect(html).not.toContain('alert(1)')
  })

  it('removes event handler attributes from images', () => {
    const html = renderMarkdown('<img src="x" onerror="alert(1)">')

    expect(html).not.toContain('onerror')
  })

  it('removes javascript links', () => {
    const html = renderMarkdown('[x](javascript:alert(1))')

    expect(html).not.toContain('javascript:')
  })
})

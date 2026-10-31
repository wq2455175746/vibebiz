import { describe, expect, it } from 'vitest'

import { PREVIEW_SANDBOX, buildPreviewUrl, previewKind } from './preview'

describe('buildPreviewUrl', () => {
  it('uses the DP prefix and encodes the project id', () => {
    expect(buildPreviewUrl('p 1')).toBe('/dp/api/v1/projects/p%201/preview/')
  })

  it('keeps nested preview paths and encodes each segment', () => {
    expect(buildPreviewUrl('p1', 'dist/my app.html')).toBe(
      '/dp/api/v1/projects/p1/preview/dist/my%20app.html',
    )
  })

  it('keeps the preview same-origin so authenticated assets load', () => {
    expect(PREVIEW_SANDBOX.split(' ')).toContain('allow-same-origin')
    expect(PREVIEW_SANDBOX.split(' ')).toContain('allow-scripts')
  })
})

describe('previewKind', () => {
  it('detects previewable document types', () => {
    expect(previewKind('index.html')).toBe('html')
    expect(previewKind('docs/a.pdf')).toBe('pdf')
    expect(previewKind('README.md')).toBe('markdown')
    expect(previewKind('assets/cover.JPG')).toBe('image')
    expect(previewKind('slides/demo.pptx')).toBe('ppt')
    expect(previewKind('data.json')).toBe('other')
  })
})

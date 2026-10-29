import { describe, expect, it } from 'vitest'

import { PREVIEW_SANDBOX, buildPreviewUrl } from './preview'

describe('buildPreviewUrl', () => {
  it('uses the DP prefix and encodes the project id', () => {
    expect(buildPreviewUrl('p 1')).toBe('/dp/api/v1/projects/p%201/preview/')
  })

  it('keeps the preview same-origin so authenticated assets load', () => {
    expect(PREVIEW_SANDBOX.split(' ')).toContain('allow-same-origin')
    expect(PREVIEW_SANDBOX.split(' ')).toContain('allow-scripts')
  })
})

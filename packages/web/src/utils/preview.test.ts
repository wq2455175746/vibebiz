import { describe, expect, it } from 'vitest'

import { buildPreviewUrl } from './preview'

describe('buildPreviewUrl', () => {
  it('uses the DP prefix and encodes the project id', () => {
    expect(buildPreviewUrl('p 1')).toBe('/dp/api/v1/projects/p%201/preview/')
  })
})

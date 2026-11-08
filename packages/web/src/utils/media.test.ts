import { describe, expect, it } from 'vitest'

import { dataUrlToBlob } from './media'

describe('dataUrlToBlob', () => {
  it('decodes a base64 data url with its mime type', () => {
    const blob = dataUrlToBlob('data:application/pdf;base64,JVBERi0xLjQK')
    expect(blob.type).toBe('application/pdf')
    expect(blob.size).toBe(9)
  })

  it('decodes a percent-encoded data url', () => {
    const blob = dataUrlToBlob('data:text/plain,%E4%BD%A0%E5%A5%BD')
    expect(blob.type).toBe('text/plain')
    expect(blob.size).toBe(6)
  })

  it('rejects invalid data urls', () => {
    expect(() => dataUrlToBlob('not-a-data-url')).toThrow('invalid data url')
  })
})

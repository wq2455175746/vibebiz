import { describe, expect, it } from 'vitest'

import { detectLanguage } from './language'

describe('detectLanguage', () => {
  it('maps known extensions', () => {
    expect(detectLanguage('src/index.html')).toBe('html')
    expect(detectLanguage('app.js')).toBe('javascript')
  })

  it('falls back to plaintext', () => {
    expect(detectLanguage('LICENSE')).toBe('plaintext')
  })
})

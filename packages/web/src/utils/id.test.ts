import { afterEach, describe, expect, it, vi } from 'vitest'

import { randomId } from './id'

const UUID_V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('randomId', () => {
  it('uses crypto.randomUUID when available', () => {
    vi.stubGlobal('crypto', { randomUUID: () => 'fixed-id' })

    expect(randomId()).toBe('fixed-id')
  })

  it('falls back to getRandomValues when randomUUID is missing', () => {
    vi.stubGlobal('crypto', {
      getRandomValues: (bytes: Uint8Array) => {
        bytes.fill(0)
        return bytes
      },
    })

    expect(randomId()).toBe('00000000-0000-4000-8000-000000000000')
  })

  it('falls back to Math.random when crypto is unavailable', () => {
    vi.stubGlobal('crypto', undefined)

    expect(randomId()).toMatch(UUID_V4)
  })
})

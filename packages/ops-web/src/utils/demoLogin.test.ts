import { afterEach, describe, expect, it, vi } from 'vitest'

import { loginDefaults } from './demoLogin'

afterEach(() => {
  vi.unstubAllEnvs()
})

describe('loginDefaults', () => {
  it('prefills the account from environment variables when enabled', () => {
    vi.stubEnv('VITE_DEMO_IDENTIFIER', 'demo-admin')
    vi.stubEnv('VITE_DEMO_PASSWORD', 'demo-pass')
    expect(loginDefaults(true)).toEqual({ identifier: 'demo-admin', password: 'demo-pass' })
  })

  it('does not hardcode credentials when environment variables are unset', () => {
    vi.stubEnv('VITE_DEMO_IDENTIFIER', '')
    vi.stubEnv('VITE_DEMO_PASSWORD', '')
    expect(loginDefaults(true)).toEqual({ identifier: '', password: '' })
  })

  it('leaves the form empty when disabled', () => {
    expect(loginDefaults(false)).toEqual({ identifier: '', password: '' })
  })
})

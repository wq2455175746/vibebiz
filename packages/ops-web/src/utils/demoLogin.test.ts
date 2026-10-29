import { describe, expect, it } from 'vitest'

import { loginDefaults } from './demoLogin'

describe('loginDefaults', () => {
  it('prefills the local ops account when enabled', () => {
    expect(loginDefaults(true)).toEqual({ identifier: 'admin', password: '88888888' })
  })

  it('leaves the form empty when disabled', () => {
    expect(loginDefaults(false)).toEqual({ identifier: '', password: '' })
  })
})

import { describe, expect, it } from 'vitest'

import { loginDefaults } from './demoLogin'

describe('loginDefaults', () => {
  it('prefills the local test account when enabled', () => {
    expect(loginDefaults(true)).toEqual({ identifier: 'wanghg11', password: '88888888' })
  })

  it('leaves the form empty when disabled', () => {
    expect(loginDefaults(false)).toEqual({ identifier: '', password: '' })
  })
})

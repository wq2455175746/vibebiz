import { describe, expect, it } from 'vitest'

import { MAX_AVATAR_BYTES, validateAvatarFile } from './avatar'

describe('validateAvatarFile', () => {
  it('accepts jpeg, png and webp within the size limit', () => {
    expect(validateAvatarFile({ type: 'image/jpeg', size: 1024 })).toBeNull()
    expect(validateAvatarFile({ type: 'image/png', size: MAX_AVATAR_BYTES })).toBeNull()
    expect(validateAvatarFile({ type: 'image/webp', size: 2048 })).toBeNull()
  })

  it('rejects unsupported image types', () => {
    expect(validateAvatarFile({ type: 'image/gif', size: 1024 })).toBe(
      '仅支持 JPG、PNG 或 WebP 图片',
    )
  })

  it('rejects files larger than 5MB', () => {
    expect(validateAvatarFile({ type: 'image/png', size: MAX_AVATAR_BYTES + 1 })).toBe(
      '图片大小不能超过 5MB',
    )
  })
})

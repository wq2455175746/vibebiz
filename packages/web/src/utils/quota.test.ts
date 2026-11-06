import { describe, expect, it } from 'vitest'

import { quotaDescription, quotaUsage } from './quota'

describe('quota copy', () => {
  it('describes each quota type', () => {
    expect(quotaDescription({ message: '', code: 42901, quotaType: 'project' })).toContain('项目')
    expect(quotaDescription({ message: '', code: 42902, quotaType: 'token' })).toContain('Token')
    expect(quotaDescription({ message: '', code: 42903, quotaType: 'upload_count' })).toContain('文件数')
    expect(quotaDescription({ message: '', code: 42904, quotaType: 'upload_bytes' })).toContain('大小')
    expect(quotaDescription(null)).toContain('额度')
  })

  it('formats used and limit when present', () => {
    expect(quotaUsage({ message: '', code: 42901, quotaType: 'project', used: 3, limit: 3 })).toBe(
      '当前：3 / 3',
    )
    expect(quotaUsage(null)).toBe('')
  })
})

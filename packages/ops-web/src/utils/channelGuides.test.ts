import { describe, expect, it } from 'vitest'

import { isMarkdownFile, splitLines } from './channelGuides'

describe('channel guide helpers', () => {
  it('splits non-empty trimmed lines', () => {
    expect(splitLines(' a \n\n b\r\nc ')).toEqual(['a', 'b', 'c'])
  })

  it('accepts markdown extensions only', () => {
    expect(isMarkdownFile('guide.md')).toBe(true)
    expect(isMarkdownFile('guide.MARKDOWN')).toBe(true)
    expect(isMarkdownFile('guide.txt')).toBe(false)
  })
})

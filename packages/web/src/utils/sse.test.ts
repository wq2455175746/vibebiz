import { describe, expect, it } from 'vitest'

import { splitSSE } from './sse'

describe('splitSSE', () => {
  it('parses complete frames and keeps the tail', () => {
    const result = splitSSE('data: {"type":"agent"}\n\ndata: [DONE]\n\ndata: {"type":"par')

    expect(result.frames).toEqual([{ data: '{"type":"agent"}' }, { data: '[DONE]' }])
    expect(result.rest).toBe('data: {"type":"par')
  })

  it('joins multi-line data fields', () => {
    const result = splitSSE('data: line-1\ndata: line-2\n\n')

    expect(result.frames).toEqual([{ data: 'line-1\nline-2' }])
    expect(result.rest).toBe('')
  })

  it('normalizes CRLF', () => {
    const result = splitSSE('data: hello\r\n\r\n')

    expect(result.frames).toEqual([{ data: 'hello' }])
  })
})

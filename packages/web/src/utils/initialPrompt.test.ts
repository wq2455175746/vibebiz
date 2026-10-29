import { describe, expect, it } from 'vitest'

import { shouldSendInitialPrompt, takeInitialAppend } from './initialPrompt'

function memoryStorage(initial: Record<string, string> = {}) {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => data.get(key) ?? null,
    setItem: (key: string, value: string) => data.set(key, value),
    removeItem: (key: string) => data.delete(key),
  }
}

describe('initialPrompt', () => {
  it('sends a new project prompt only without history', () => {
    expect(shouldSendInitialPrompt('hi', false, false)).toBe(true)
    expect(shouldSendInitialPrompt('hi', true, false)).toBe(false)
  })

  it('sends append prompts even with history', () => {
    expect(shouldSendInitialPrompt('hi', true, true)).toBe(true)
  })

  it('consumes the append flag once', () => {
    const storage = memoryStorage({ 'vibebiz:initial-append:p1': '1' })

    expect(takeInitialAppend('p1', storage)).toBe(true)
    expect(takeInitialAppend('p1', storage)).toBe(false)
  })
})

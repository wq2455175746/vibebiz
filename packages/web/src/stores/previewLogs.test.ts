import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import { usePreviewLogStore } from './previewLogs'

beforeEach(() => {
  setActivePinia(createPinia())
})

describe('preview log store', () => {
  it('keeps preview logs separate and newest last', () => {
    const logs = usePreviewLogStore()

    logs.append('log', 'hello')
    logs.append('error', 'boom')

    expect(logs.entries.map((entry) => [entry.level, entry.message])).toEqual([
      ['info', 'hello'],
      ['error', 'boom'],
    ])
    expect(logs.entries.every((entry) => entry.source === 'preview')).toBe(true)
  })

  it('clears all preview logs', () => {
    const logs = usePreviewLogStore()
    logs.append('log', 'hello')

    logs.clear()

    expect(logs.entries).toEqual([])
  })
})

import { describe, expect, it } from 'vitest'

import { summarizeTools } from './toolCalls'

describe('summarizeTools', () => {
  it('keeps small tool lists expanded', () => {
    const summary = summarizeTools([
      { id: '1', name: 'Read', status: 'done' },
      { id: '2', name: 'Edit', status: 'running' },
      { id: '3', name: 'Bash', status: 'done' },
    ])

    expect(summary.collapsible).toBe(false)
    expect(summary.running).toBe(1)
  })

  it('summarizes repeated tools in first-use order', () => {
    const summary = summarizeTools([
      { id: '1', name: 'Bash', status: 'done' },
      { id: '2', name: 'Edit', status: 'done' },
      { id: '3', name: 'Bash', status: 'done' },
      { id: '4', name: 'Read', status: 'done' },
      { id: '5', name: 'Bash', status: 'done' },
    ])

    expect(summary.collapsible).toBe(true)
    expect(summary.total).toBe(5)
    expect(summary.running).toBe(0)
    expect(summary.detail).toBe('Bash ×3 · Edit ×1 · Read ×1')
  })
})

import { describe, expect, it } from 'vitest'

import { buildMockChatSteps, buildMockLogLines } from './editorMock'

describe('editor mock', () => {
  it('streams text, tools and logs for a prompt', () => {
    const steps = buildMockChatSteps('做一个贪吃蛇')
    expect(steps.some((step) => step.text?.includes('第一版已经完成'))).toBe(true)
    expect(steps.some((step) => step.tool?.name === 'Read')).toBe(true)
    expect(steps.some((step) => step.toolDone === 'mock-tool-read')).toBe(true)
    expect(steps.filter((step) => step.log).length).toBeGreaterThan(3)
  })

  it('returns only log steps in order', () => {
    const logs = buildMockLogLines('落地页')
    expect(logs[0].message).toContain('落地页')
    expect(logs.at(-1)?.message).toContain('预览已刷新')
  })
})

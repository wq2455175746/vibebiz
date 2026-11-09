import { describe, expect, it } from 'vitest'

import {
  actionStatusLabel,
  agentActionLabel,
  formatDuration,
  scoreBand,
  scorePercent,
  sourceKindLabel,
  superStepLabel,
} from './superTeam'

describe('ops super team utils', () => {
  it('maps source kinds', () => {
    expect(sourceKindLabel('feishu')).toBe('飞书')
    expect(sourceKindLabel('webhook')).toBe('Webhook')
    expect(sourceKindLabel('queue')).toBe('消息队列')
    expect(sourceKindLabel('unknown')).toBe('unknown')
  })

  it('maps steps and actions', () => {
    expect(superStepLabel('requirements')).toBe('需求挖掘')
    expect(superStepLabel('operations')).toBe('观测运维')
    expect(agentActionLabel('noise_filter')).toBe('噪音过滤')
    expect(agentActionLabel('semantic_eval')).toBe('语义评估')
    expect(agentActionLabel('artifact')).toBe('产物生成')
    expect(actionStatusLabel('running')).toBe('执行中')
  })

  it('formats scores and durations', () => {
    expect(scorePercent(82.6)).toBe('83%')
    expect(scorePercent(-5)).toBe('0%')
    expect(scorePercent(120)).toBe('100%')
    expect(scoreBand(20)).toBe('low')
    expect(scoreBand(50)).toBe('medium')
    expect(scoreBand(90)).toBe('high')
    expect(formatDuration(320)).toBe('320ms')
    expect(formatDuration(1840)).toBe('1.8s')
  })
})

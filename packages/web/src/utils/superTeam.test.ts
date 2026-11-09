import { describe, expect, it } from 'vitest'

import {
  SUPER_TEAM_STORAGE_KEY,
  advanceStep,
  defaultSources,
  defaultSteps,
  initialState,
  isStepUnlocked,
  loadState,
  noiseBand,
  semanticBand,
  serializeState,
  sourceMeta,
  sourceStatusLabel,
  stepStatusLabel,
  stepTitle,
} from './superTeam'

describe('super team utils', () => {
  it('starts with requirements active and later steps locked', () => {
    const steps = defaultSteps()
    expect(steps.map((step) => step.key)).toEqual(['requirements', 'development', 'operations'])
    expect(steps[0].status).toBe('active')
    expect(steps[1].status).toBe('locked')
    expect(steps[2].status).toBe('locked')
    expect(isStepUnlocked(steps, 'requirements')).toBe(true)
    expect(isStepUnlocked(steps, 'development')).toBe(false)
  })

  it('advances requirements and unlocks development only', () => {
    const steps = advanceStep(defaultSteps(), 'requirements')
    expect(steps[0].status).toBe('done')
    expect(steps[0].progress).toBe(100)
    expect(steps[1].status).toBe('ready')
    expect(steps[2].status).toBe('locked')
  })

  it('does not advance a locked step', () => {
    const steps = defaultSteps()
    expect(advanceStep(steps, 'operations')).toEqual(steps)
  })

  it('advances development and unlocks operations', () => {
    const steps = advanceStep(defaultSteps(), 'requirements')
    const next = advanceStep(steps, 'development')
    expect(next[2].status).toBe('ready')
  })

  it('provides four event sources with queue disabled', () => {
    const sources = defaultSources()
    expect(sources.map((source) => source.kind)).toEqual(['feishu', 'webhook', 'email', 'queue'])
    const queue = sources.find((source) => source.kind === 'queue')
    expect(queue?.enabled).toBe(false)
    expect(queue?.status).toBe('coming_soon')
  })

  it('labels sources and steps', () => {
    expect(sourceMeta('feishu').label).toBe('飞书')
    expect(sourceStatusLabel('connected')).toBe('已连接')
    expect(stepTitle('development')).toBe('系统开发')
    expect(stepStatusLabel('locked')).toBe('锁定')
  })

  it('round-trips state and merges missing fields', () => {
    const state = initialState()
    const raw = serializeState(state)
    expect(loadState(raw).sources).toHaveLength(4)
    const merged = loadState(
      JSON.stringify({
        sources: [{ kind: 'email', enabled: true, fields: { host: 'imap.test' } }],
        steps: [{ key: 'requirements', status: 'done', progress: 100 }],
      }),
    )
    const email = merged.sources.find((source) => source.kind === 'email')
    expect(email?.enabled).toBe(true)
    expect(email?.fields.host).toBe('imap.test')
    expect(merged.steps.find((step) => step.key === 'requirements')?.status).toBe('done')
    expect(merged.steps.find((step) => step.key === 'development')?.status).toBe('locked')
  })

  it('falls back to defaults for invalid storage', () => {
    expect(loadState('not-json').sources).toHaveLength(4)
    expect(loadState(null).steps).toHaveLength(3)
  })

  it('bands scores consistently', () => {
    expect(noiseBand(20)).toBe('low')
    expect(noiseBand(55)).toBe('medium')
    expect(noiseBand(85)).toBe('high')
    expect(semanticBand(75)).toBe('high')
  })

  it('exposes a stable storage key', () => {
    expect(SUPER_TEAM_STORAGE_KEY).toBe('vibebiz:super-team')
  })
})

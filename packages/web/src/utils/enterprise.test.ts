import { describe, expect, it } from 'vitest'

import type { Project } from '@/types'
import {
  buildStagePrompt,
  completeStage,
  createInitialStageState,
  currentStageKey,
  ENTERPRISE_STAGES,
  isEnterpriseProject,
  loadEnterpriseStageState,
  markEnterpriseProject,
  resolveEditorPath,
  saveEnterpriseStageState,
} from './enterprise'

function memoryStorage(initial: Record<string, string> = {}) {
  const values = new Map(Object.entries(initial))
  return {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => void values.set(key, value),
    removeItem: (key: string) => void values.delete(key),
  }
}

const project: Project = {
  id: 'p1',
  name: 'demo',
  prompt: '',
  state: 'ready',
  createdAt: '2026-10-30T00:00:00Z',
  updatedAt: '2026-10-30T00:00:00Z',
}

describe('enterprise stages', () => {
  it('starts with requirements active and open future stages locked', () => {
    const stages = createInitialStageState()
    expect(currentStageKey(stages)).toBe('requirements')
    expect(stages.requirements.status).toBe('active')
    expect(stages.architecture.status).toBe('locked')
    expect(stages.artifacts.status).toBe('placeholder')
    expect(stages.delivery.status).toBe('placeholder')
  })

  it('completes only the active stage and opens the next stage', () => {
    const next = completeStage(createInitialStageState(), 'requirements', '2026-10-30T10:00:00Z')
    expect(next).not.toBeNull()
    expect(next?.states.requirements.status).toBe('done')
    expect(next?.states.architecture.status).toBe('active')
    expect(next?.nextKey).toBe('architecture')
    expect(currentStageKey(next!.states)).toBe('architecture')
  })

  it('rejects completing a locked or placeholder stage', () => {
    const initial = createInitialStageState()
    expect(completeStage(initial, 'architecture')).toBeNull()
    expect(completeStage(initial, 'artifacts')).toBeNull()
  })

  it('does not activate placeholder stages after development', () => {
    let stages = createInitialStageState()
    for (const key of ['requirements', 'architecture', 'detailed_design', 'test_plan'] as const) {
      const result = completeStage(stages, key)
      expect(result).not.toBeNull()
      stages = result!.states
    }
    const result = completeStage(stages, 'development')
    expect(result?.states.development.status).toBe('done')
    expect(result?.states.artifacts.status).toBe('placeholder')
    expect(result?.states.delivery.status).toBe('placeholder')
    expect(result?.nextKey).toBeNull()
  })

  it('builds requirement and later stage prompts with role and artifact', () => {
    const requirements = ENTERPRISE_STAGES[0]
    const architecture = ENTERPRISE_STAGES[1]
    expect(buildStagePrompt(requirements, '做一个 CRM')).toContain('用户需求：\n做一个 CRM')
    expect(buildStagePrompt(requirements, '')).toContain('docs/requirements.md')
    expect(buildStagePrompt(architecture, '')).toContain('Bob')
    expect(buildStagePrompt(architecture, '')).toContain('docs/architecture.md')
  })
})

describe('enterprise project routing', () => {
  it('resolves enterprise projects to the enterprise editor', () => {
    const storage = memoryStorage()
    markEnterpriseProject('p1', storage)
    expect(isEnterpriseProject(project, storage)).toBe(true)
    expect(resolveEditorPath(project, storage)).toBe('/enterprise/editor/p1')
  })

  it('keeps standard projects on the existing editor', () => {
    const storage = memoryStorage()
    expect(isEnterpriseProject(project, storage)).toBe(false)
    expect(resolveEditorPath(project, storage)).toBe('/editor/p1')
  })

  it('uses the backend mode when present', () => {
    const storage = memoryStorage()
    expect(isEnterpriseProject({ ...project, mode: 'enterprise' }, storage)).toBe(true)
    expect(resolveEditorPath({ ...project, mode: 'enterprise' }, storage)).toBe(
      '/enterprise/editor/p1',
    )
  })
})

describe('enterprise stage persistence', () => {
  it('round-trips stage state per project', () => {
    const storage = memoryStorage()
    const stages = createInitialStageState()
    const next = completeStage(stages, 'requirements')!.states
    saveEnterpriseStageState('p1', next, storage)
    expect(loadEnterpriseStageState('p1', storage)).toEqual(next)
    expect(currentStageKey(loadEnterpriseStageState('p1', storage))).toBe('architecture')
  })

  it('falls back to a fresh state for malformed storage', () => {
    const storage = memoryStorage({ 'vibebiz:enterprise:stages:p1': '{bad json' })
    expect(loadEnterpriseStageState('p1', storage)).toEqual(createInitialStageState())
  })
})

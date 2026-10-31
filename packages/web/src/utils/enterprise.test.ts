import { describe, expect, it } from 'vitest'

import type { Project } from '@/types'
import {
  buildStagePrompt,
  createInitialStageState,
  currentStageKey,
  ENTERPRISE_STAGES,
  isEnterpriseProject,
  stageStatesFromProject,
  markEnterpriseProject,
  resolveEditorPath,
  stageArtifactReady,
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

  it('maps API stage records into local state', () => {
    const states = stageStatesFromProject([
      {
        stageKey: 'requirements',
        seq: 1,
        title: '需求梳理',
        roleCode: 'emma',
        roleName: 'Emma',
        artifactPath: 'docs/requirements.md',
        workspace: 'document',
        status: 'done',
        startedAt: '2026-10-30T09:00:00Z',
        completedAt: '2026-10-30T09:30:00Z',
      },
      {
        stageKey: 'architecture',
        seq: 2,
        title: '概要设计',
        roleCode: 'bob',
        roleName: 'Bob',
        artifactPath: 'docs/architecture.md',
        workspace: 'document',
        status: 'active',
      },
    ])
    expect(states.requirements.status).toBe('done')
    expect(states.architecture.status).toBe('active')
    expect(states.test_plan.status).toBe('locked')
  })

  it('only accepts the loaded artifact of the active stage', () => {
    const development = ENTERPRISE_STAGES[4]
    expect(stageArtifactReady(development, 'docs/test-plan.md', 'old content')).toBe(false)
    expect(stageArtifactReady(development, 'docs/development.md', '')).toBe(false)
    expect(stageArtifactReady(development, 'docs/development.md', 'done')).toBe(true)
    expect(stageArtifactReady(null, 'docs/development.md', 'done')).toBe(false)
  })

  it('marks artifact and delivery as dedicated workspaces', () => {
    expect(ENTERPRISE_STAGES.find((item) => item.key === 'artifacts')?.workspace).toBe('artifact')
    expect(ENTERPRISE_STAGES.find((item) => item.key === 'delivery')?.workspace).toBe('delivery')
    expect(buildStagePrompt(ENTERPRISE_STAGES[5])).toBe('')
    expect(buildStagePrompt(ENTERPRISE_STAGES[6])).toBe('')
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

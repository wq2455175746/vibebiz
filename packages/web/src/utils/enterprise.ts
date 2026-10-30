import type { Project } from '@/types'

export type EnterpriseStageKey =
  | 'requirements'
  | 'architecture'
  | 'detailed_design'
  | 'test_plan'
  | 'development'
  | 'artifacts'
  | 'delivery'

export type EnterpriseStageStatus = 'locked' | 'active' | 'done' | 'placeholder'
export type EnterpriseStageWorkspace = 'document' | 'development' | 'placeholder'

export interface EnterpriseStageDefinition {
  key: EnterpriseStageKey
  seq: number
  title: string
  roleCode: string
  roleLabel: string
  artifactPath: string
  workspace: EnterpriseStageWorkspace
}

export interface EnterpriseStageState {
  status: EnterpriseStageStatus
  startedAt: string | null
  completedAt: string | null
}

export type EnterpriseStageStateMap = Record<EnterpriseStageKey, EnterpriseStageState>

export interface CompleteStageResult {
  states: EnterpriseStageStateMap
  nextKey: EnterpriseStageKey | null
}

export const DEFAULT_ENTERPRISE_PROMPT =
  '我要进行企业级工程开发。请使用 #团队角色 协作，先完成需求梳理，再依次进行概要设计、详细设计、测试用例、开发和交付。必须按阶段逐步完成，每阶段产出后等待我确认，未确认不得进入下一阶段。'

export const ENTERPRISE_STAGES: EnterpriseStageDefinition[] = [
  {
    key: 'requirements',
    seq: 1,
    title: '需求梳理',
    roleCode: 'emma',
    roleLabel: 'Emma · 产品经理',
    artifactPath: 'docs/requirements.md',
    workspace: 'document',
  },
  {
    key: 'architecture',
    seq: 2,
    title: '概要设计',
    roleCode: 'bob',
    roleLabel: 'Bob · 系统架构师',
    artifactPath: 'docs/architecture.md',
    workspace: 'document',
  },
  {
    key: 'detailed_design',
    seq: 3,
    title: '详细设计',
    roleCode: 'senior',
    roleLabel: '高级工程师',
    artifactPath: 'docs/detailed-design.md',
    workspace: 'document',
  },
  {
    key: 'test_plan',
    seq: 4,
    title: '测试用例',
    roleCode: 'qa',
    roleLabel: '测试开发工程师',
    artifactPath: 'docs/test-plan.md',
    workspace: 'document',
  },
  {
    key: 'development',
    seq: 5,
    title: '全栈开发',
    roleCode: 'alex',
    roleLabel: 'Alex · 全栈工程师',
    artifactPath: 'docs/development.md',
    workspace: 'development',
  },
  {
    key: 'artifacts',
    seq: 6,
    title: '制品空间',
    roleCode: '',
    roleLabel: '暂未开放',
    artifactPath: '',
    workspace: 'placeholder',
  },
  {
    key: 'delivery',
    seq: 7,
    title: '交付空间',
    roleCode: '',
    roleLabel: '暂未开放',
    artifactPath: '',
    workspace: 'placeholder',
  },
]

const ENTERPRISE_PROJECT_PREFIX = 'vibebiz:enterprise:project:'
const ENTERPRISE_STAGE_PREFIX = 'vibebiz:enterprise:stages:'

type ReadStorage = Pick<Storage, 'getItem'>
type WriteStorage = Pick<Storage, 'setItem'>

function stageKey(projectId: string) {
  return `${ENTERPRISE_STAGE_PREFIX}${projectId}`
}

function projectKey(projectId: string) {
  return `${ENTERPRISE_PROJECT_PREFIX}${projectId}`
}

export function stageDefinition(key: EnterpriseStageKey): EnterpriseStageDefinition {
  const stage = ENTERPRISE_STAGES.find((item) => item.key === key)
  if (!stage) {
    throw new Error(`unknown enterprise stage: ${key}`)
  }
  return stage
}

export function createInitialStageState(): EnterpriseStageStateMap {
  return ENTERPRISE_STAGES.reduce((result, stage) => {
    result[stage.key] = {
      status:
        stage.key === 'requirements'
          ? 'active'
          : stage.workspace === 'placeholder'
            ? 'placeholder'
            : 'locked',
      startedAt: null,
      completedAt: null,
    }
    return result
  }, {} as EnterpriseStageStateMap)
}

export function currentStageKey(states: EnterpriseStageStateMap): EnterpriseStageKey | null {
  return ENTERPRISE_STAGES.find((stage) => states[stage.key].status === 'active')?.key ?? null
}

export function completeStage(
  states: EnterpriseStageStateMap,
  key: EnterpriseStageKey,
  now = new Date().toISOString(),
): CompleteStageResult | null {
  const index = ENTERPRISE_STAGES.findIndex((stage) => stage.key === key)
  if (index < 0 || states[key].status !== 'active') {
    return null
  }
  const nextStage = ENTERPRISE_STAGES.slice(index + 1).find(
    (stage) => stage.workspace !== 'placeholder',
  )
  const next = { ...states, [key]: { ...states[key], status: 'done' as const, completedAt: now } }
  if (nextStage) {
    next[nextStage.key] = {
      ...next[nextStage.key],
      status: 'active',
      startedAt: next[nextStage.key].startedAt ?? now,
    }
  }
  return { states: next, nextKey: nextStage?.key ?? null }
}

export function buildStagePrompt(stage: EnterpriseStageDefinition, requirement = ''): string {
  if (stage.workspace === 'placeholder') {
    return ''
  }
  const lines = [
    `当前阶段：${stage.seq}. ${stage.title}`,
    `负责角色：${stage.roleLabel}`,
    `阶段产物：${stage.artifactPath}`,
    '只完成当前阶段，不要提前进入后续阶段。',
  ]
  if (stage.key === 'requirements') {
    lines.push('', '请梳理用户需求，写入固定产物文件，包含目标、用户故事、验收标准和非目标。')
    lines.push('', '用户需求：', requirement.trim() || '（未提供，请先向用户确认）')
  } else {
    lines.push('', '请先读取上一阶段产物，再完成本阶段，并写入固定产物文件。')
  }
  lines.push('', '完成后等待用户确认，不要自行进入下一阶段。')
  return lines.join('\n')
}

function isStageStateMap(value: unknown): value is EnterpriseStageStateMap {
  if (!value || typeof value !== 'object') {
    return false
  }
  const record = value as Record<string, unknown>
  return ENTERPRISE_STAGES.every((stage) => {
    const item = record[stage.key]
    if (!item || typeof item !== 'object') {
      return false
    }
    const entry = item as Record<string, unknown>
    return (
      entry.status === 'locked' ||
      entry.status === 'active' ||
      entry.status === 'done' ||
      entry.status === 'placeholder'
    )
  })
}

export function loadEnterpriseStageState(
  projectId: string,
  storage: ReadStorage = localStorage,
): EnterpriseStageStateMap {
  const raw = storage.getItem(stageKey(projectId))
  if (!raw) {
    return createInitialStageState()
  }
  try {
    const parsed: unknown = JSON.parse(raw)
    return isStageStateMap(parsed) ? parsed : createInitialStageState()
  } catch {
    return createInitialStageState()
  }
}

export function saveEnterpriseStageState(
  projectId: string,
  states: EnterpriseStageStateMap,
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(stageKey(projectId), JSON.stringify(states))
}

export function markEnterpriseProject(
  projectId: string,
  storage: WriteStorage = localStorage,
): void {
  storage.setItem(projectKey(projectId), '1')
}

export function isEnterpriseProject(
  project: Project,
  storage: ReadStorage = localStorage,
): boolean {
  return project.mode === 'enterprise' || storage.getItem(projectKey(project.id)) === '1'
}

export function resolveEditorPath(project: Project, storage: ReadStorage = localStorage): string {
  return isEnterpriseProject(project, storage)
    ? `/enterprise/editor/${project.id}`
    : `/editor/${project.id}`
}

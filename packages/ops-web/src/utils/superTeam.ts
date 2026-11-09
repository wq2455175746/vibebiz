const SOURCE_LABELS: Record<string, string> = {
  feishu: '飞书',
  webhook: 'Webhook',
  email: 'Email',
  queue: '消息队列',
}

const STEP_LABELS: Record<string, string> = {
  requirements: '需求挖掘',
  development: '系统开发',
  operations: '观测运维',
}

const ACTION_LABELS: Record<string, string> = {
  noise_filter: '噪音过滤',
  semantic_eval: '语义评估',
  artifact: '产物生成',
  step_advance: '阶段推进',
}

const STATUS_LABELS: Record<string, string> = {
  success: '成功',
  failed: '失败',
  running: '执行中',
}

export type ScoreBand = 'low' | 'medium' | 'high'

export function sourceKindLabel(kind: string): string {
  return SOURCE_LABELS[kind] ?? kind
}

export function superStepLabel(key: string): string {
  return STEP_LABELS[key] ?? key
}

export function agentActionLabel(action: string): string {
  return ACTION_LABELS[action] ?? action
}

export function actionStatusLabel(status: string): string {
  return STATUS_LABELS[status] ?? status
}

export function scorePercent(score: number): string {
  const normalized = Math.max(0, Math.min(100, Math.round(score)))
  return `${normalized}%`
}

export function scoreBand(score: number): ScoreBand {
  if (score >= 70) {
    return 'high'
  }
  return score >= 40 ? 'medium' : 'low'
}

export function formatDuration(ms: number): string {
  if (ms < 1000) {
    return `${ms}ms`
  }
  return `${(ms / 1000).toFixed(1)}s`
}

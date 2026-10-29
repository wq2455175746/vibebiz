export type HomeSubmitPlan =
  | { mode: 'create' }
  | { mode: 'append'; projectId: string }

export function resolveHomeSubmit(targetProjectId: string | null): HomeSubmitPlan {
  return targetProjectId ? { mode: 'append', projectId: targetProjectId } : { mode: 'create' }
}

export function canAppendToProject(active: boolean): boolean {
  return !active
}

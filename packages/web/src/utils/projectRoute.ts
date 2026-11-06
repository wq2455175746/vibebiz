import type { Project } from '@/types'
import { isEnterpriseProject } from './enterprise'

export type ProjectTarget =
  | { name: 'clones' }
  | { name: 'scheduled-tasks' }
  | { name: 'enterprise-editor'; params: { id: string } }
  | { name: 'editor'; params: { id: string } }

export function isScheduledProject(
  project: Project,
  scheduledProjectIds: ReadonlySet<string>,
): boolean {
  return scheduledProjectIds.has(project.id)
}

export function isStandardProject(
  project: Project,
  scheduledProjectIds: ReadonlySet<string>,
  storage?: Pick<Storage, 'getItem'>,
): boolean {
  return (
    (project.mode ?? 'standard') === 'standard' &&
    !isScheduledProject(project, scheduledProjectIds) &&
    !isEnterpriseProject(project, storage)
  )
}

export function filterStandardProjects(
  projects: Project[],
  scheduledProjectIds: ReadonlySet<string>,
  storage?: Pick<Storage, 'getItem'>,
): Project[] {
  return projects.filter((project) => isStandardProject(project, scheduledProjectIds, storage))
}

export function resolveProjectTarget(
  project: Project,
  scheduledProjectIds: ReadonlySet<string>,
  storage?: Pick<Storage, 'getItem'>,
): ProjectTarget {
  if (project.mode === 'clone') {
    return { name: 'clones' }
  }
  if (isScheduledProject(project, scheduledProjectIds)) {
    return { name: 'scheduled-tasks' }
  }
  if (isEnterpriseProject(project, storage)) {
    return { name: 'enterprise-editor', params: { id: project.id } }
  }
  return { name: 'editor', params: { id: project.id } }
}

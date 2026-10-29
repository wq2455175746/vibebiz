import type { ChatReferenceInput } from '@/api/chat'
import type { FileEntry, ProjectResource } from '@/types'

export interface MentionMatch {
  start: number
  end: number
  query: string
}

export function findActiveMention(value: string, cursor: number): MentionMatch | null {
  const before = value.slice(0, cursor)
  const start = before.lastIndexOf('@')
  if (start < 0) {
    return null
  }
  const query = before.slice(start + 1)
  if (/\s/.test(query)) {
    return null
  }
  return { start, end: cursor, query }
}

export function mentionQuery(value: string, start: number, end: number): string {
  return value.slice(start + 1, end).toLowerCase()
}

export function applyMention(
  value: string,
  match: MentionMatch,
  replacement: string,
): { value: string; cursor: number } {
  const next = `${value.slice(0, match.start)}@${replacement} ${value.slice(match.end)}`
  return { value: next, cursor: match.start + replacement.length + 2 }
}

export function collectReferences(
  value: string,
  resources: ProjectResource[],
  files: FileEntry[],
): ChatReferenceInput[] {
  const result: ChatReferenceInput[] = []
  const seen = new Set<string>()
  for (const match of value.matchAll(/@([^\s@]+)/g)) {
    const token = match[1]
    const resource = resources.find(
      (item) => item.displayName === token || item.name === token || item.path === token,
    )
    if (resource) {
      const key = `resource:${resource.id}`
      if (!seen.has(key)) {
        seen.add(key)
        result.push({
          kind: 'resource',
          id: resource.id,
          name: resource.displayName,
          path: resource.path,
          contentType: resource.contentType,
          resourceKind: resource.kind,
        })
      }
      continue
    }
    const file = files.find(
      (item) => item.type === 'file' && (item.path === token || item.path.endsWith(`/${token}`)),
    )
    if (file) {
      const key = `file:${file.path}`
      if (!seen.has(key)) {
        seen.add(key)
        result.push({ kind: 'project_file', name: file.path.split('/').pop() ?? file.path, path: file.path })
      }
    }
  }
  return result
}

export function mergeReferences(items: ChatReferenceInput[]): ChatReferenceInput[] {
  const seen = new Set<string>()
  const result: ChatReferenceInput[] = []
  for (const item of items) {
    const key = item.kind === 'resource' ? `resource:${item.id}` : `file:${item.path}`
    if (seen.has(key)) {
      continue
    }
    seen.add(key)
    result.push(item)
  }
  return result
}

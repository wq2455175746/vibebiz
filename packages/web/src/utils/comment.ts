import type { CardComment } from '@/types'

export function findComment(nodes: CardComment[], id: string): CardComment | null {
  for (const node of nodes) {
    if (node.id === id) return node
    const found = findComment(node.replies ?? [], id)
    if (found) return found
  }
  return null
}

export function patchComment(
  nodes: CardComment[],
  id: string,
  patch: Partial<CardComment>,
): CardComment[] {
  return nodes.map((node) => {
    if (node.id === id) return { ...node, ...patch }
    if (!node.replies?.length) return node
    return { ...node, replies: patchComment(node.replies, id, patch) }
  })
}

export function removeComment(nodes: CardComment[], id: string): CardComment[] {
  return nodes
    .filter((node) => node.id !== id)
    .map((node) =>
      node.replies?.length ? { ...node, replies: removeComment(node.replies, id) } : node,
    )
}

export function mergeCommentPage(
  current: CardComment[],
  page: CardComment[],
  append: boolean,
): CardComment[] {
  if (!append) return page
  const seen = new Set(current.map((item) => item.id))
  return [...current, ...page.filter((item) => !seen.has(item.id))]
}

export function countComments(nodes: CardComment[]): number {
  return nodes.reduce((total, node) => total + 1 + countComments(node.replies ?? []), 0)
}

import { describe, expect, it } from 'vitest'

import type { CardComment } from '@/types'

import {
  countComments,
  findComment,
  mergeCommentPage,
  patchComment,
  removeComment,
} from './comment'

function baseComment(id: string, overrides: Partial<CardComment> = {}): CardComment {
  return {
    id,
    cardId: 'card-1',
    depth: 0,
    content: id,
    author: { userId: 1, name: 'alice', avatarUrl: null },
    replyCount: 0,
    status: 'normal',
    editable: true,
    deletable: true,
    editedAt: null,
    createdAt: '2026-11-16T00:00:00.000Z',
    replies: [],
    ...overrides,
  }
}

describe('findComment', () => {
  it('finds a root comment', () => {
    const tree = [baseComment('root')]
    expect(findComment(tree, 'root')?.id).toBe('root')
  })

  it('finds a nested reply', () => {
    const tree = [baseComment('root', { replies: [baseComment('reply')] })]
    expect(findComment(tree, 'reply')?.id).toBe('reply')
  })

  it('returns null when missing', () => {
    expect(findComment([baseComment('root')], 'nope')).toBeNull()
  })
})

describe('patchComment', () => {
  it('patches a nested reply without touching siblings', () => {
    const tree: CardComment[] = [
      {
        ...baseComment('root'),
        replies: [baseComment('sibling'), { ...baseComment('reply'), replies: [] }],
      },
    ]
    const next = patchComment(tree, 'reply', { content: '改过' })
    expect(next[0].replies?.[1].content).toBe('改过')
    expect(next[0].replies?.[0].content).toBe('sibling')
    expect(next[0].content).toBe('root')
  })

  it('does not mutate the input tree', () => {
    const tree = [baseComment('root')]
    const next = patchComment(tree, 'root', { content: 'next' })
    expect(tree[0].content).toBe('root')
    expect(next[0].content).toBe('next')
  })

  it('returns an equivalent tree when the id is missing', () => {
    const tree = [baseComment('root', { replies: [baseComment('reply')] })]
    const next = patchComment(tree, 'missing', { content: 'x' })
    expect(next.map((item) => item.id)).toEqual(['root'])
    expect(next[0].replies?.[0].content).toBe('reply')
  })
})

describe('removeComment', () => {
  it('removes a leaf reply and keeps the root', () => {
    const tree: CardComment[] = [
      { ...baseComment('root'), replies: [baseComment('reply')] },
    ]
    const next = removeComment(tree, 'reply')
    expect(next).toHaveLength(1)
    expect(next[0].replies).toHaveLength(0)
  })

  it('removes a root together with its replies', () => {
    const tree = [baseComment('root', { replies: [baseComment('reply')] })]
    expect(removeComment(tree, 'root')).toHaveLength(0)
  })

  it('removes a deeply nested reply', () => {
    const tree = [
      baseComment('root', {
        replies: [
          baseComment('reply', {
            replies: [baseComment('deep')],
          }),
        ],
      }),
    ]
    const next = removeComment(tree, 'deep')
    expect(next[0].replies?.[0].replies).toHaveLength(0)
  })
})

describe('mergeCommentPage', () => {
  it('replaces when not appending', () => {
    const current = [baseComment('old')]
    const next = mergeCommentPage(current, [baseComment('new')], false)
    expect(next.map((item) => item.id)).toEqual(['new'])
  })

  it('appends new items and drops duplicates', () => {
    const current = [baseComment('c1')]
    const next = mergeCommentPage(
      current,
      [baseComment('c1'), baseComment('c2')],
      true,
    )
    expect(next.map((item) => item.id)).toEqual(['c1', 'c2'])
  })
})

describe('countComments', () => {
  it('counts nested replies', () => {
    const tree = [
      baseComment('root', {
        replies: [baseComment('reply', { replies: [baseComment('deep')] })],
      }),
      baseComment('root-2'),
    ]
    expect(countComments(tree)).toBe(4)
  })

  it('is zero for an empty tree', () => {
    expect(countComments([])).toBe(0)
  })
})

import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as api from '@/api/cards'
import type { CardComment } from '@/types'

import { useCommentStore } from './comment'

vi.mock('@/api/cards', () => ({
  getCardComments: vi.fn(),
  getCardCommentReplies: vi.fn(),
  createCardComment: vi.fn(),
  updateCardComment: vi.fn(),
  deleteCardComment: vi.fn(),
}))

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

beforeEach(() => {
  vi.clearAllMocks()
  setActivePinia(createPinia())
})

describe('comment store loading', () => {
  it('loads first page and appends next page', async () => {
    vi.mocked(api.getCardComments)
      .mockResolvedValueOnce({ comments: [baseComment('c1')], nextCursor: 'c1', hasMore: true })
      .mockResolvedValueOnce({ comments: [baseComment('c2')], nextCursor: '', hasMore: false })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadMore('card-1')

    expect(store.comments.map((item) => item.id)).toEqual(['c1', 'c2'])
    expect(api.getCardComments).toHaveBeenLastCalledWith('card-1', { cursor: 'c1', limit: 20 })
    expect(store.hasMore).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('replaces roots on a fresh load', async () => {
    vi.mocked(api.getCardComments)
      .mockResolvedValueOnce({ comments: [baseComment('old')], nextCursor: '', hasMore: false })
      .mockResolvedValueOnce({ comments: [baseComment('new')], nextCursor: '', hasMore: false })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadRoots('card-1')

    expect(store.comments.map((item) => item.id)).toEqual(['new'])
  })

  it('does not load more when there is nothing more', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('c1')],
      nextCursor: '',
      hasMore: false,
    })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadMore('card-1')

    expect(api.getCardComments).toHaveBeenCalledTimes(1)
  })

  it('dedupes appended pages', async () => {
    vi.mocked(api.getCardComments)
      .mockResolvedValueOnce({ comments: [baseComment('c1')], nextCursor: 'c1', hasMore: true })
      .mockResolvedValueOnce({
        comments: [baseComment('c1'), baseComment('c2')],
        nextCursor: '',
        hasMore: false,
      })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadMore('card-1')

    expect(store.comments.map((item) => item.id)).toEqual(['c1', 'c2'])
  })
})

describe('comment store replies', () => {
  it('loads the first reply page and appends later pages', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root', { replyCount: 2 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.getCardCommentReplies)
      .mockResolvedValueOnce({ comments: [baseComment('r1')], nextCursor: 'r1', hasMore: true })
      .mockResolvedValueOnce({ comments: [baseComment('r2')], nextCursor: '', hasMore: false })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadReplies('card-1', 'root')
    await store.loadReplies('card-1', 'root', 'r1')

    expect(api.getCardCommentReplies).toHaveBeenNthCalledWith(1, 'card-1', 'root', {
      cursor: '',
      limit: 10,
    })
    expect(api.getCardCommentReplies).toHaveBeenNthCalledWith(2, 'card-1', 'root', {
      cursor: 'r1',
      limit: 10,
    })
    expect(store.comments[0].replies?.map((item) => item.id)).toEqual(['r1', 'r2'])
  })

  it('ignores replies for an unknown root', async () => {
    vi.mocked(api.getCardCommentReplies).mockResolvedValue({
      comments: [baseComment('r1')],
      nextCursor: '',
      hasMore: false,
    })

    const store = useCommentStore()
    await store.loadReplies('card-1', 'missing')

    expect(store.comments).toEqual([])
  })

  it('continues from the stored cursor when no cursor is passed', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root', { replyCount: 2 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.getCardCommentReplies)
      .mockResolvedValueOnce({ comments: [baseComment('r1')], nextCursor: 'r1', hasMore: true })
      .mockResolvedValueOnce({ comments: [baseComment('r2')], nextCursor: '', hasMore: false })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadReplies('card-1', 'root')
    await store.loadReplies('card-1', 'root')

    expect(api.getCardCommentReplies).toHaveBeenNthCalledWith(2, 'card-1', 'root', {
      cursor: 'r1',
      limit: 10,
    })
    expect(store.comments[0].replies?.map((item) => item.id)).toEqual(['r1', 'r2'])
    expect(store.replyHasMore.root).toBe(false)
  })

  it('records an error and rethrows when replies fail to load', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root', { replyCount: 2 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.getCardCommentReplies).mockRejectedValue('boom')

    const store = useCommentStore()
    await store.loadRoots('card-1')

    await expect(store.loadReplies('card-1', 'root')).rejects.toBe('boom')
    expect(store.error).toBe('回复加载失败')
  })
})

describe('comment store create', () => {
  it('prepends a new root comment', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('c1')],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.createCardComment).mockResolvedValue(baseComment('c2'))

    const store = useCommentStore()
    await store.loadRoots('card-1')
    const created = await store.create('card-1', 'hello')

    expect(created.id).toBe('c2')
    expect(store.comments.map((item) => item.id)).toEqual(['c2', 'c1'])
    expect(store.submitting).toBe(false)
  })

  it('inserts a reply under its root and increments its reply count', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root')],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.createCardComment).mockResolvedValue(
      baseComment('reply', { parentId: 'root', rootId: 'root', depth: 1 }),
    )

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.create('card-1', 'hi', 'root')

    expect(store.comments[0].replies?.map((item) => item.id)).toEqual(['reply'])
    expect(store.comments[0].replyCount).toBe(1)
  })

  it('inserts a nested reply under its parent and increments the root count', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [
        baseComment('root', {
          replies: [baseComment('reply', { parentId: 'root', rootId: 'root', depth: 1 })],
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.createCardComment).mockResolvedValue(
      baseComment('deep', { parentId: 'reply', rootId: 'root', depth: 2 }),
    )

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.create('card-1', 'deep', 'reply')

    expect(store.comments[0].replies?.[0].replies?.map((item) => item.id)).toEqual(['deep'])
    expect(store.comments[0].replyCount).toBe(1)
  })

  it('records an error message and rethrows when creation fails', async () => {
    vi.mocked(api.createCardComment).mockRejectedValue(new Error('nope'))

    const store = useCommentStore()
    await expect(store.create('card-1', 'bad')).rejects.toThrow('nope')
    expect(store.error).toBe('nope')
    expect(store.submitting).toBe(false)
  })
})

describe('comment store update and delete', () => {
  it('patches a comment after update', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root', { replies: [baseComment('reply')] })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.updateCardComment).mockResolvedValue(
      baseComment('reply', { content: '改过', editedAt: '2026-11-16T01:00:00.000Z' }),
    )

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.update('card-1', 'reply', '改过')

    expect(api.updateCardComment).toHaveBeenCalledWith('card-1', 'reply', '改过')
    expect(store.comments[0].replies?.[0].content).toBe('改过')
    expect(store.comments[0].replies?.[0].editedAt).toBe('2026-11-16T01:00:00.000Z')
  })

  it('tombstones a comment that keeps replies', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root', { content: 'hi', replyCount: 1 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.deleteCardComment).mockResolvedValue({ tombstoned: true, commentCount: 1 })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    const result = await store.remove('card-1', 'root')

    expect(result.tombstoned).toBe(true)
    expect(store.comments).toHaveLength(1)
    expect(store.comments[0]).toMatchObject({
      status: 'deleted',
      content: '',
      editable: false,
      deletable: false,
    })
  })

  it('removes a leaf comment', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [
        baseComment('root', {
          replies: [baseComment('reply', { parentId: 'root', rootId: 'root', depth: 1 })],
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.deleteCardComment).mockResolvedValue({ tombstoned: false, commentCount: 0 })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.remove('card-1', 'reply')

    expect(store.comments[0].replies).toHaveLength(0)
  })

  it('records an error and rethrows when update fails', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root')],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.updateCardComment).mockRejectedValue(new Error('update boom'))

    const store = useCommentStore()
    await store.loadRoots('card-1')

    await expect(store.update('card-1', 'root', 'new')).rejects.toThrow('update boom')
    expect(store.error).toBe('update boom')
  })

  it('records an error and rethrows when delete fails', async () => {
    vi.mocked(api.getCardComments).mockResolvedValue({
      comments: [baseComment('root')],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(api.deleteCardComment).mockRejectedValue('delete boom')

    const store = useCommentStore()
    await store.loadRoots('card-1')

    await expect(store.remove('card-1', 'root')).rejects.toBe('delete boom')
    expect(store.error).toBe('评论删除失败')
  })
})

describe('comment store errors', () => {
  it('records a message when loading fails', async () => {
    vi.mocked(api.getCardComments).mockRejectedValue(new Error('boom'))

    const store = useCommentStore()
    await store.loadRoots('card-1')

    expect(store.error).toBe('boom')
    expect(store.loading).toBe(false)
  })

  it('falls back to a default message for non-Error failures', async () => {
    vi.mocked(api.getCardComments).mockRejectedValue('boom')

    const store = useCommentStore()
    await store.loadRoots('card-1')

    expect(store.error).toBe('评论加载失败')
  })

  it('clears the error after a successful load', async () => {
    vi.mocked(api.getCardComments)
      .mockRejectedValueOnce(new Error('boom'))
      .mockResolvedValueOnce({ comments: [baseComment('c1')], nextCursor: '', hasMore: false })

    const store = useCommentStore()
    await store.loadRoots('card-1')
    await store.loadRoots('card-1')

    expect(store.error).toBe('')
  })
})

import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CardCommentTree from './CardCommentTree.vue'
import * as cardsApi from '@/api/cards'
import type { CardComment } from '@/types'

vi.mock('@/api/cards', () => ({
  getCardComments: vi.fn(),
  getCardCommentReplies: vi.fn(),
  createCardComment: vi.fn(),
  updateCardComment: vi.fn(),
  deleteCardComment: vi.fn(),
}))

function makeComment(id: string, overrides: Partial<CardComment> = {}): CardComment {
  return {
    id,
    cardId: 'card-1',
    depth: 1,
    content: id,
    author: { userId: 7, name: 'alice', avatarUrl: null },
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

let app: App | null = null
let host: HTMLElement

async function flush() {
  for (let index = 0; index < 3; index += 1) {
    await new Promise((resolve) => setTimeout(resolve, 0))
  }
}

async function mount(props: Record<string, unknown> = {}) {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardCommentTree, {
    cardId: 'card-1',
    currentUserId: 7,
    commentCount: 1,
    ...props,
  })
  app.use(createPinia())
  app.mount(host)
  await flush()
}

function buttonByText(text: string, scope: ParentNode = host): HTMLButtonElement | undefined {
  return Array.from(scope.querySelectorAll<HTMLButtonElement>('button')).find((button) =>
    button.textContent?.includes(text),
  )
}

function typeInto(textarea: HTMLTextAreaElement, value: string) {
  textarea.value = value
  textarea.dispatchEvent(new Event('input'))
}

beforeEach(() => {
  vi.clearAllMocks()
  vi.mocked(cardsApi.getCardComments).mockResolvedValue({
    comments: [],
    nextCursor: '',
    hasMore: false,
  })
  vi.mocked(cardsApi.getCardCommentReplies).mockResolvedValue({
    comments: [],
    nextCursor: '',
    hasMore: false,
  })
  vi.mocked(cardsApi.createCardComment).mockResolvedValue(makeComment('new'))
  vi.mocked(cardsApi.updateCardComment).mockResolvedValue(makeComment('updated'))
  vi.mocked(cardsApi.deleteCardComment).mockResolvedValue({ tombstoned: false, commentCount: 0 })
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  vi.restoreAllMocks()
})

describe('CardCommentTree', () => {
  it('renders root comments and only shows delete for deletable comments', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [
        makeComment('c1', { content: '很不错的作品', deletable: true }),
        makeComment('c2', {
          content: '别人的评论',
          deletable: false,
          editable: false,
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })

    await mount()

    expect(host.textContent).toContain('很不错的作品')
    expect(host.textContent).toContain('别人的评论')
    expect(
      Array.from(host.querySelectorAll('button')).filter(
        (item) => item.textContent?.trim() === '删除',
      ),
    ).toHaveLength(1)
  })

  it('renders nested replies and flattens depth 3 with the reply target name', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [
        makeComment('root', {
          replies: [
            makeComment('r1', {
              depth: 2,
              parentId: 'root',
              rootId: 'root',
              replies: [
                makeComment('r2', {
                  depth: 3,
                  parentId: 'r1',
                  rootId: 'root',
                  replyToName: 'bob',
                  content: '同意',
                }),
              ],
            }),
          ],
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })

    await mount()

    expect(host.textContent).toContain('回复 @bob')
    expect(host.textContent).toContain('同意')
  })

  it('shows a tombstone for deleted comments and keeps their replies', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [
        makeComment('root', {
          status: 'deleted',
          content: '',
          editable: false,
          deletable: false,
          replyCount: 1,
          replies: [
            makeComment('r1', {
              depth: 2,
              parentId: 'root',
              rootId: 'root',
              content: '回复还在',
            }),
          ],
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })

    await mount()

    const nodes = host.querySelectorAll<HTMLElement>('.card-comment-node')
    expect(nodes[0].textContent).toContain('该评论已删除')
    expect(nodes[0].querySelector(':scope > .card-comment-body > .card-comment-actions')).toBeNull()
    expect(host.textContent).toContain('回复还在')
  })

  it('loads replies when clicking the reply count', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('root', { replyCount: 2 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.getCardCommentReplies).mockResolvedValue({
      comments: [
        makeComment('r1', { depth: 2, parentId: 'root', rootId: 'root', content: '第一条回复' }),
      ],
      nextCursor: '',
      hasMore: false,
    })

    await mount()
    buttonByText('查看 2 条回复')?.click()
    await flush()

    expect(cardsApi.getCardCommentReplies).toHaveBeenCalledWith('card-1', 'root', {
      cursor: '',
      limit: 10,
    })
    expect(host.textContent).toContain('第一条回复')
  })

  it('loads more when the backend embeds only reply previews', async () => {
    const previews = [1, 2].map((index) =>
      makeComment(`r${index}`, {
        depth: 2,
        parentId: 'root',
        rootId: 'root',
        content: `预览 ${index}`,
      }),
    )
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('root', { replyCount: 5, replies: previews })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.getCardCommentReplies).mockResolvedValue({
      comments: [1, 2, 3, 4, 5].map((index) =>
        makeComment(`r${index}`, {
          depth: 2,
          parentId: 'root',
          rootId: 'root',
          content: `回复 ${index}`,
        }),
      ),
      nextCursor: '',
      hasMore: false,
    })

    await mount()
    expect(host.textContent).toContain('预览 1')
    const affordance = buttonByText('加载更多回复')
    expect(affordance).toBeDefined()

    affordance?.click()
    await flush()

    expect(cardsApi.getCardCommentReplies).toHaveBeenCalledWith('card-1', 'root', {
      cursor: '',
      limit: 10,
    })
    expect(host.textContent).toContain('回复 5')
    expect(buttonByText('加载更多回复')).toBeUndefined()
  })

  it('hides the reply affordance when previews already cover every reply', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [
        makeComment('root', {
          replyCount: 2,
          replies: [
            makeComment('r1', { depth: 2, parentId: 'root', rootId: 'root' }),
            makeComment('r2', { depth: 2, parentId: 'root', rootId: 'root' }),
          ],
        }),
      ],
      nextCursor: '',
      hasMore: false,
    })

    await mount()

    expect(buttonByText('查看 2 条回复')).toBeUndefined()
    expect(buttonByText('加载更多回复')).toBeUndefined()
  })

  it('loads subsequent reply pages and offers a load more replies button', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('root', { replyCount: 12 })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.getCardCommentReplies)
      .mockResolvedValueOnce({
        comments: Array.from({ length: 10 }, (_, index) =>
          makeComment(`r${index + 1}`, {
            depth: 2,
            parentId: 'root',
            rootId: 'root',
            content: `回复 ${index + 1}`,
          }),
        ),
        nextCursor: 'r10',
        hasMore: true,
      })
      .mockResolvedValueOnce({
        comments: Array.from({ length: 2 }, (_, index) =>
          makeComment(`r${index + 11}`, {
            depth: 2,
            parentId: 'root',
            rootId: 'root',
            content: `回复 ${index + 11}`,
          }),
        ),
        nextCursor: '',
        hasMore: false,
      })

    await mount()
    buttonByText('查看 12 条回复')?.click()
    await flush()

    expect(host.textContent).toContain('回复 1')
    expect(buttonByText('加载更多回复')).toBeDefined()

    buttonByText('加载更多回复')?.click()
    await flush()

    expect(cardsApi.getCardCommentReplies).toHaveBeenNthCalledWith(2, 'card-1', 'root', {
      cursor: 'r10',
      limit: 10,
    })
    expect(host.textContent).toContain('回复 11')
    expect(buttonByText('加载更多回复')).toBeUndefined()
  })

  it('loads more root comments from the next cursor', async () => {
    vi.mocked(cardsApi.getCardComments)
      .mockResolvedValueOnce({
        comments: [makeComment('c1', { content: '第一页' })],
        nextCursor: 'c1',
        hasMore: true,
      })
      .mockResolvedValueOnce({
        comments: [makeComment('c2', { content: '第二页' })],
        nextCursor: '',
        hasMore: false,
      })

    await mount()
    expect(host.textContent).toContain('第一页')
    buttonByText('加载更多评论')?.click()
    await flush()

    expect(cardsApi.getCardComments).toHaveBeenLastCalledWith('card-1', {
      cursor: 'c1',
      limit: 20,
    })
    expect(host.textContent).toContain('第二页')
    expect(buttonByText('加载更多评论')).toBeUndefined()
  })

  it('shows a performance hint once 200 nodes are loaded', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: Array.from({ length: 200 }, (_, index) =>
        makeComment(`c${index}`, { content: `评论 ${index}` }),
      ),
      nextCursor: 'c199',
      hasMore: true,
    })

    await mount()

    expect(host.textContent).toContain('已加载 200 条评论，继续加载可能影响页面性能。')
  })

  it('creates a root comment and emits the new count', async () => {
    const onCountChange = vi.fn()
    vi.mocked(cardsApi.createCardComment).mockResolvedValue(
      makeComment('c1', { content: '新评论' }),
    )

    await mount({ commentCount: 3, onCountChange })
    typeInto(host.querySelector<HTMLTextAreaElement>('.card-comment-form textarea')!, '新评论')
    host.querySelector<HTMLFormElement>('.card-comment-form')!.dispatchEvent(new Event('submit'))
    await flush()

    expect(cardsApi.createCardComment).toHaveBeenCalledWith(
      'card-1',
      expect.objectContaining({ content: '新评论' }),
    )
    expect(onCountChange).toHaveBeenCalledWith(4)
    expect(host.textContent).toContain('新评论')
  })

  it('creates a reply under the selected comment and emits the new count', async () => {
    const onCountChange = vi.fn()
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('root', { content: '根评论' })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.createCardComment).mockResolvedValue(
      makeComment('reply', {
        depth: 2,
        parentId: 'root',
        rootId: 'root',
        content: '回复内容',
      }),
    )

    await mount({ onCountChange })
    buttonByText('回复')?.click()
    typeInto(host.querySelector<HTMLTextAreaElement>('.card-comment-form textarea')!, '回复内容')
    host.querySelector<HTMLFormElement>('.card-comment-form')!.dispatchEvent(new Event('submit'))
    await flush()

    expect(cardsApi.createCardComment).toHaveBeenCalledWith(
      'card-1',
      expect.objectContaining({ content: '回复内容', parentId: 'root' }),
    )
    expect(onCountChange).toHaveBeenCalledWith(2)
    expect(host.textContent).toContain('回复内容')
  })

  it('edits an own comment in place', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('c1', { content: '旧内容' })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.updateCardComment).mockResolvedValue(
      makeComment('c1', { content: '改过', editedAt: '2026-11-16T01:00:00.000Z' }),
    )

    await mount()
    buttonByText('编辑')?.click()
    await flush()
    typeInto(host.querySelector<HTMLTextAreaElement>('.card-comment-body textarea')!, '改过')
    buttonByText('保存')?.click()
    await flush()

    expect(cardsApi.updateCardComment).toHaveBeenCalledWith('card-1', 'c1', '改过')
    expect(host.textContent).toContain('改过')
    expect(host.textContent).toContain('已编辑')
  })

  it('keeps the editor open and preserves the draft when an update fails', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('c1', { content: '旧内容' })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.updateCardComment).mockRejectedValue(new Error('更新失败'))

    await mount()
    buttonByText('编辑')?.click()
    await flush()
    typeInto(host.querySelector<HTMLTextAreaElement>('.card-comment-body textarea')!, '改过')
    buttonByText('保存')?.click()
    await flush()

    expect(host.textContent).toContain('更新失败')
    const textarea = host.querySelector<HTMLTextAreaElement>('.card-comment-body textarea')
    expect(textarea).not.toBeNull()
    expect(textarea?.value).toBe('改过')
  })

  it('deletes an own comment and emits the server count', async () => {
    const onCountChange = vi.fn()
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('c1', { content: '要删除' })],
      nextCursor: '',
      hasMore: false,
    })
    vi.mocked(cardsApi.deleteCardComment).mockResolvedValue({ tombstoned: true, commentCount: 0 })

    await mount({ onCountChange })
    buttonByText('删除')?.click()
    await flush()
    expect(cardsApi.deleteCardComment).not.toHaveBeenCalled()

    buttonByText('确认删除')?.click()
    await flush()

    expect(cardsApi.deleteCardComment).toHaveBeenCalledWith('card-1', 'c1')
    expect(onCountChange).toHaveBeenCalledWith(0)
    expect(host.textContent).toContain('该评论已删除')
  })

  it('cancels a pending comment delete', async () => {
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('c1', { content: '不要删了' })],
      nextCursor: '',
      hasMore: false,
    })

    await mount()
    buttonByText('删除')?.click()
    await flush()
    expect(buttonByText('确认删除')).toBeDefined()

    buttonByText('取消')?.click()
    await flush()

    expect(cardsApi.deleteCardComment).not.toHaveBeenCalled()
    expect(buttonByText('确认删除')).toBeUndefined()
    expect(buttonByText('删除')).toBeDefined()
  })

  it('collapses long comments and toggles expand and collapse', async () => {
    const longContent = '长'.repeat(300)
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('c1', { content: longContent })],
      nextCursor: '',
      hasMore: false,
    })

    await mount()
    const content = host.querySelector<HTMLElement>('.card-comment-content')
    expect(content?.classList.contains('is-collapsed')).toBe(true)
    expect(buttonByText('展开')).toBeDefined()

    buttonByText('展开')?.click()
    await flush()

    expect(content?.classList.contains('is-collapsed')).toBe(false)
    expect(buttonByText('收起')).toBeDefined()

    buttonByText('收起')?.click()
    await flush()

    expect(content?.classList.contains('is-collapsed')).toBe(true)
  })

  it('prompts login instead of submitting an anonymous comment', async () => {
    const onLoginRequired = vi.fn()
    await mount({ currentUserId: null, onLoginRequired })
    typeInto(host.querySelector<HTMLTextAreaElement>('.card-comment-form textarea')!, '匿名评论')
    host.querySelector<HTMLFormElement>('.card-comment-form')!.dispatchEvent(new Event('submit'))
    await flush()

    expect(cardsApi.createCardComment).not.toHaveBeenCalled()
    expect(onLoginRequired).toHaveBeenCalledTimes(1)
  })

  it('prompts login instead of selecting a reply target when anonymous', async () => {
    const onLoginRequired = vi.fn()
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment('root', { content: '根评论' })],
      nextCursor: '',
      hasMore: false,
    })

    await mount({ currentUserId: null, onLoginRequired })
    buttonByText('回复')?.click()
    await flush()

    expect(onLoginRequired).toHaveBeenCalledTimes(1)
    expect(host.querySelector('.card-comment-reply-hint')).toBeNull()
  })

  it('shows the empty state when there are no comments', async () => {
    await mount()
    expect(host.textContent).toContain('还没有评论。')
  })
})

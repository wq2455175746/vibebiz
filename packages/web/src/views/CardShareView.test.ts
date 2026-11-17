import { createPinia } from 'pinia'
import { createApp, type App } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import CardShareView from './CardShareView.vue'
import * as cardsApi from '@/api/cards'
import type { BusinessCard, CardComment } from '@/types'

const routeState = vi.hoisted(() => ({
  id: 'demo-1',
  fullPath: '/share/card/demo-1',
}))
const routerState = vi.hoisted(() => ({ push: vi.fn() }))
const authState = vi.hoisted(() => ({
  user: null as { id: number } | null,
  ensure: vi.fn(async () => {}),
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: routeState.id }, fullPath: routeState.fullPath }),
  useRouter: () => ({ push: routerState.push }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => authState,
}))

vi.mock('@/api/cards', () => ({
  getPublicCard: vi.fn(),
  getCardComments: vi.fn(),
  getCardCommentReplies: vi.fn(),
  createCardComment: vi.fn(),
  updateCardComment: vi.fn(),
  deleteCardComment: vi.fn(),
  likeCard: vi.fn(),
  unlikeCard: vi.fn(),
}))

const flushPromises = () => new Promise((resolve) => setTimeout(resolve, 0))

function makeCard(overrides: Partial<BusinessCard> = {}): BusinessCard {
  return {
    id: 'demo-1',
    ownerUserId: 7,
    ownerName: '林一',
    avatarUrl: null,
    headline: '前端工程师',
    tags: ['Vue', '游戏'],
    items: [
      {
        projectId: 'demo-1-p1',
        name: '作品一',
        description: '',
        coverUrl: null,
        type: 'html',
        shareUrl: '/share/project/demo-1-p1',
        addedAt: '2026-11-08T00:00:00.000Z',
      },
    ],
    featured: { kind: 'project', projectId: 'demo-1-p1' },
    published: true,
    commentCount: 3,
    likeCount: 0,
    liked: false,
    createdAt: '2026-11-08T00:00:00.000Z',
    updatedAt: '2026-11-08T00:00:00.000Z',
    ...overrides,
  }
}

function makeComment(overrides: Partial<CardComment> = {}): CardComment {
  return {
    id: 'c1',
    cardId: 'demo-1',
    depth: 1,
    content: '很不错的作品',
    author: { userId: 8, name: 'bob', avatarUrl: null },
    replyCount: 0,
    status: 'normal',
    editable: false,
    deletable: false,
    editedAt: null,
    createdAt: '2026-11-16T00:00:00.000Z',
    replies: [],
    ...overrides,
  }
}

let app: App | null = null
let host: HTMLElement
const scrollIntoViewDescriptor = Object.getOwnPropertyDescriptor(
  Element.prototype,
  'scrollIntoView',
)

async function mount() {
  host = document.createElement('div')
  document.body.appendChild(host)
  app = createApp(CardShareView)
  app.use(createPinia())
  app.mount(host)
  await flushPromises()
}

beforeEach(() => {
  vi.mocked(cardsApi.getPublicCard).mockReset()
  vi.mocked(cardsApi.getPublicCard).mockResolvedValue(makeCard())
  vi.mocked(cardsApi.getCardComments).mockReset()
  vi.mocked(cardsApi.getCardComments).mockResolvedValue({
    comments: [],
    nextCursor: '',
    hasMore: false,
  })
  vi.mocked(cardsApi.createCardComment).mockReset()
  authState.user = null
  authState.ensure.mockClear()
  vi.mocked(cardsApi.likeCard).mockReset()
  vi.mocked(cardsApi.likeCard).mockResolvedValue({ liked: true, likeCount: 1 })
  vi.mocked(cardsApi.unlikeCard).mockReset()
  vi.mocked(cardsApi.unlikeCard).mockResolvedValue({ liked: false, likeCount: 0 })
})

afterEach(() => {
  app?.unmount()
  app = null
  host?.remove()
  if (scrollIntoViewDescriptor) {
    Object.defineProperty(Element.prototype, 'scrollIntoView', scrollIntoViewDescriptor)
  } else {
    Reflect.deleteProperty(Element.prototype, 'scrollIntoView')
  }
  routeState.id = 'demo-1'
  routeState.fullPath = '/share/card/demo-1'
  routerState.push.mockReset()
  vi.restoreAllMocks()
})

describe('CardShareView', () => {
  it('renders the card profile and projects from the public API', async () => {
    await mount()
    expect(cardsApi.getPublicCard).toHaveBeenCalledWith('demo-1')
    expect(host.textContent).toContain('林一')
    expect(host.textContent).toContain('前端工程师')
    expect(host.textContent).toContain('Vue')
    expect(host.textContent).toContain('作品一')
  })

  it('enables the comment entry and keeps the message entry disabled', async () => {
    await mount()
    expect(host.querySelector<HTMLButtonElement>('.card-social-comment')?.disabled).toBe(false)
    expect(host.querySelector<HTMLButtonElement>('.card-social-message')?.disabled).toBe(true)
  })

  it('scrolls to the comments section when clicking the comment entry', async () => {
    const scrollIntoView = vi.fn()
    Object.defineProperty(Element.prototype, 'scrollIntoView', {
      value: scrollIntoView,
      writable: true,
      configurable: true,
    })
    await mount()
    host.querySelector<HTMLElement>('.card-social-comment')?.click()
    expect(scrollIntoView).toHaveBeenCalledWith({ behavior: 'smooth', block: 'start' })
    expect(host.querySelector('#comments')).not.toBeNull()
  })

  it('loads the comment section from the public comments API', async () => {
    await mount()
    expect(cardsApi.getCardComments).toHaveBeenCalledWith('demo-1')
    expect(host.textContent).toContain('还没有评论。')
  })

  it('opens the featured project from the main preview', async () => {
    await mount()
    host.querySelector<HTMLElement>('.share-featured')?.click()
    expect(routerState.push).toHaveBeenCalledWith({
      name: 'project-share',
      params: { projectId: 'demo-1-p1' },
    })
  })

  it('opens the featured file in a preview tab instead of downloading it', async () => {
    const open = vi.spyOn(window, 'open').mockImplementation(() => null)
    vi.mocked(cardsApi.getPublicCard).mockResolvedValue(
      makeCard({
        featured: {
          kind: 'file',
          fileType: 'image',
          name: 'cover.png',
          size: 1024,
          url: 'https://example.com/cover.png',
        },
      }),
    )
    await mount()
    host.querySelector<HTMLElement>('.share-featured')?.click()
    expect(open).toHaveBeenCalledWith('/share/card/demo-1/file', '_blank', 'noopener,noreferrer')
  })

  it('does not render remove buttons on the public share page', async () => {
    await mount()
    expect(host.querySelectorAll('.card-project-remove')).toHaveLength(0)
  })

  it('shows an empty state when the card cannot be loaded', async () => {
    vi.mocked(cardsApi.getPublicCard).mockRejectedValue(new Error('名片不存在'))
    await mount()
    expect(host.querySelector('.share-empty')).not.toBeNull()
    expect(host.textContent).toContain('名片不存在')
  })

  it('shows an edit entry only for the own card', async () => {
    authState.user = null
    await mount()
    expect(host.textContent).not.toContain('编辑名片')
    app?.unmount()
    host.remove()
    authState.user = { id: 7 }
    await mount()
    const edit = Array.from(host.querySelectorAll<HTMLAnchorElement>('a')).find(
      (link) => link.textContent === '编辑名片',
    )
    expect(edit?.getAttribute('href')).toBe('/my-card')
  })

  it('redirects anonymous visitors to login when liking', async () => {
    authState.user = null
    await mount()
    host.querySelector<HTMLButtonElement>('.card-like-button')?.click()
    await flushPromises()
    expect(routerState.push).toHaveBeenCalledWith({
      name: 'login',
      query: { redirect: '/share/card/demo-1' },
    })
    expect(cardsApi.likeCard).not.toHaveBeenCalled()
  })

  it('redirects anonymous visitors to login when commenting', async () => {
    authState.user = null
    await mount()
    const textarea = host.querySelector<HTMLTextAreaElement>('.card-comment-form textarea')!
    textarea.value = '匿名评论'
    textarea.dispatchEvent(new Event('input'))
    host.querySelector<HTMLFormElement>('.card-comment-form')!.dispatchEvent(new Event('submit'))
    await flushPromises()

    expect(routerState.push).toHaveBeenCalledWith({
      name: 'login',
      query: { redirect: '/share/card/demo-1' },
    })
    expect(cardsApi.createCardComment).not.toHaveBeenCalled()
  })

  it('redirects anonymous visitors to login when replying', async () => {
    authState.user = null
    vi.mocked(cardsApi.getCardComments).mockResolvedValue({
      comments: [makeComment()],
      nextCursor: '',
      hasMore: false,
    })
    await mount()
    const reply = Array.from(host.querySelectorAll<HTMLButtonElement>('button')).find(
      (button) => button.textContent?.trim() === '回复',
    )
    reply?.click()
    await flushPromises()

    expect(routerState.push).toHaveBeenCalledWith({
      name: 'login',
      query: { redirect: '/share/card/demo-1' },
    })
  })

  it('likes optimistically and reconciles with the API state', async () => {
    authState.user = { id: 7 }
    vi.mocked(cardsApi.likeCard).mockResolvedValue({ liked: true, likeCount: 4 })
    await mount()
    const button = host.querySelector<HTMLButtonElement>('.card-like-button')
    button?.click()
    await flushPromises()
    expect(cardsApi.likeCard).toHaveBeenCalledWith('demo-1')
    expect(host.querySelector('.card-like-button.is-liked')).not.toBeNull()
    expect(button?.textContent).toContain('4')
  })

  it('rolls back the optimistic like when the API fails', async () => {
    authState.user = { id: 7 }
    vi.mocked(cardsApi.likeCard).mockRejectedValue(new Error('boom'))
    await mount()
    const button = host.querySelector<HTMLButtonElement>('.card-like-button')
    button?.click()
    await flushPromises()
    expect(cardsApi.likeCard).toHaveBeenCalledWith('demo-1')
    expect(host.querySelector('.card-like-button.is-liked')).toBeNull()
    expect(button?.textContent).toContain('0')
    expect(host.textContent).toContain('点赞失败，请稍后重试')
  })
})

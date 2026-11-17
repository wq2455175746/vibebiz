import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import {
  addCardProject,
  clearCardFeatured,
  createCardComment,
  deleteCardComment,
  getCardCommentReplies,
  getCardComments,
  getDiscoverCards,
  getMyCard,
  getPublicCard,
  getPublicCardProject,
  likeCard,
  publishMyCard,
  removeCardProject,
  setCardFeaturedProject,
  unlikeCard,
  updateCardComment,
  updateMyCard,
  uploadCardFeatured,
} from './cards'

vi.mock('./client', () => ({ cpHttp: {}, request: vi.fn() }))

const requestMock = vi.mocked(request)

beforeEach(() => {
  requestMock.mockReset()
  requestMock.mockResolvedValue({ id: 'card-1' })
})

describe('card api', () => {
  it('loads and updates my card', async () => {
    await getMyCard()
    expect(requestMock).toHaveBeenNthCalledWith(1, cpHttp, { method: 'get', url: '/me/card' })
    await updateMyCard({ headline: 'hi', tags: ['Vue'] })
    expect(requestMock).toHaveBeenNthCalledWith(2, cpHttp, {
      method: 'put',
      url: '/me/card',
      data: { headline: 'hi', tags: ['Vue'] },
    })
  })

  it('adds and removes projects', async () => {
    await addCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/items',
      data: { projectId: 'p1' },
    })
    await removeCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/me/card/items/p1',
    })
  })

  it('sets and clears featured project', async () => {
    await setCardFeaturedProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'put',
      url: '/me/card/featured',
      data: { kind: 'project', projectId: 'p1' },
    })
    await clearCardFeatured()
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/me/card/featured',
    })
  })

  it('uploads featured file with content type and encoded name', async () => {
    const file = new File(['%PDF-1.4'], '简历.pdf', { type: 'application/pdf' })
    await uploadCardFeatured(file)
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/featured/upload',
      data: file,
      headers: {
        'Content-Type': 'application/pdf',
        'X-File-Name': encodeURIComponent('简历.pdf'),
      },
      timeout: 30_000,
    })
  })

  it('publishes and loads discovery/public data', async () => {
    await publishMyCard()
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/me/card/publish',
    })
    await getDiscoverCards({ page: 2, pageSize: 12 })
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/cards/discover',
      params: { page: 2, pageSize: 12 },
    })
    await getPublicCard('card-1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/public/cards/card-1',
    })
    await getPublicCardProject('p1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/public/card-projects/p1',
    })
  })

  it('loads and mutates card comments', async () => {
    await getCardComments('card-1', { cursor: 'c1', limit: 20 })
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: '/public/cards/card-1/comments',
      params: { cursor: 'c1', limit: 20 },
    })
    await createCardComment('card-1', { content: 'hi', parentId: 'c-0' })
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'post',
      url: '/public/cards/card-1/comments',
      data: { content: 'hi', parentId: 'c-0' },
    })
  })

  it('defaults comment query params and encodes ids', async () => {
    await getCardComments('card 1/x')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: `/public/cards/${encodeURIComponent('card 1/x')}/comments`,
      params: {},
    })
    await getCardCommentReplies('card-1', 'c/1', { limit: 5 })
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'get',
      url: `/public/cards/card-1/comments/${encodeURIComponent('c/1')}/replies`,
      params: { limit: 5 },
    })
  })

  it('updates and deletes a comment', async () => {
    await updateCardComment('card-1', 'c-1', 'edited')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'put',
      url: '/public/cards/card-1/comments/c-1',
      data: { content: 'edited' },
    })
    await deleteCardComment('card-1', 'c-1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/public/cards/card-1/comments/c-1',
    })
  })

  it('likes and unlikes a card', async () => {
    await likeCard('card-1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'put',
      url: '/public/cards/card-1/like',
    })
    await unlikeCard('card-1')
    expect(requestMock).toHaveBeenLastCalledWith(cpHttp, {
      method: 'delete',
      url: '/public/cards/card-1/like',
    })
  })
})

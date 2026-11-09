import { beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import {
  addCardProject,
  clearCardFeatured,
  getDiscoverCards,
  getMyCard,
  getPublicCard,
  getPublicCardProject,
  publishMyCard,
  removeCardProject,
  setCardFeaturedProject,
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
})

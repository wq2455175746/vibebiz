import { beforeEach, describe, expect, it, vi } from 'vitest'

import { request } from './client'
import { deleteCard, listCards, updateCardHidden } from './cards'

vi.mock('./client', () => ({ request: vi.fn() }))
const requestMock = vi.mocked(request)

beforeEach(() => requestMock.mockReset())

describe('cards api', () => {
  it('lists cards with query params', async () => {
    requestMock.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 10 })
    await listCards({ page: 2, pageSize: 10, keyword: 'alice' })
    expect(requestMock).toHaveBeenCalledWith({
      method: 'get',
      url: '/cards',
      params: { page: 2, pageSize: 10, keyword: 'alice' },
    })
  })

  it('hides a card', async () => {
    requestMock.mockResolvedValue({})
    await updateCardHidden('c1', true)
    expect(requestMock).toHaveBeenCalledWith({
      method: 'put',
      url: '/cards/c1/hidden',
      data: { hidden: true },
    })
  })

  it('deletes a card', async () => {
    requestMock.mockResolvedValue({})
    await deleteCard('c1')
    expect(requestMock).toHaveBeenCalledWith({ method: 'delete', url: '/cards/c1' })
  })
})

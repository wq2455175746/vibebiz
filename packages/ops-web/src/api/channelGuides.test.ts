import { describe, expect, it, vi } from 'vitest'

import {
  createChannelGuide,
  deleteChannelGuide,
  getChannelGuide,
  getChannelGuides,
  updateChannelGuide,
} from './admin'
import { http } from './client'

describe('channel guide admin api', () => {
  it('lists channel guides with filters', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { guides: [] } },
    })
    await getChannelGuides({ keyword: '微信', status: 'enabled' })
    expect(spy).toHaveBeenCalledWith({
      method: 'get',
      url: '/channel-guides',
      params: { keyword: '微信', status: 'enabled' },
    })
  })

  it('loads a detail', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'wechat_minigame', markdown: '# x' } },
    })
    await expect(getChannelGuide('wechat_minigame')).resolves.toMatchObject({
      channel: 'wechat_minigame',
    })
  })

  it('creates and updates a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'taptap_mobile' } },
    })
    await createChannelGuide({
      channel: 'taptap_mobile',
      title: 'TapTap',
      summary: '',
      markdown: '# TapTap',
      steps: [],
      requirements: [],
      enabled: true,
      sortOrder: 40,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'post',
      url: '/channel-guides',
      data: expect.objectContaining({ channel: 'taptap_mobile' }),
    })
  })

  it('deletes a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { deleted: true } },
    })
    await deleteChannelGuide('taptap_mobile')
    expect(spy).toHaveBeenCalledWith({
      method: 'delete',
      url: '/channel-guides/taptap_mobile',
    })
  })

  it('updates a channel guide', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { channel: 'wechat_minigame' } },
    })
    await updateChannelGuide('wechat_minigame', {
      title: '微信',
      summary: '',
      markdown: '# x',
      steps: [],
      requirements: [],
      enabled: false,
      sortOrder: 10,
    })
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/channel-guides/wechat_minigame',
      data: expect.objectContaining({ enabled: false }),
    })
  })
})

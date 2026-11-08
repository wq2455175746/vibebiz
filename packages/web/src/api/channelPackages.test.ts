import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { cpHttp, request } from './client'
import {
  deleteChannelPackage,
  downloadChannelPackage,
  generateChannelPackage,
  listChannelGuides,
  listChannelPackages,
} from './channelPackages'
import type { ChannelPackage } from '@/types'

vi.mock('./client', () => ({
  cpHttp: {},
  request: vi.fn(),
}))

const requestMock = vi.mocked(request)

const samplePackage: ChannelPackage = {
  id: 'pkg-1',
  channel: 'wechat_minigame',
  sourceType: 'html5',
  status: 'ready',
  version: 'v1',
  fileSizeBytes: 1024,
  level: 'B',
  warnings: [],
  errorMessage: '',
  createdAt: '2026-11-07T00:00:00Z',
  downloadUrl: '/cp/api/v1/projects/p1/channel-packages/pkg-1/download',
  guideUrl: '/settings/channel-guides?channel=wechat_minigame',
}

beforeEach(() => {
  requestMock.mockReset()
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('channelPackages api', () => {
  it('unwraps generate envelope', async () => {
    requestMock.mockResolvedValue({ package: samplePackage })
    const result = await generateChannelPackage('p1', {
      channel: 'wechat_minigame',
      sourceType: 'html5',
    })
    expect(result.channel).toBe('wechat_minigame')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects/p1/channel-packages',
      data: { channel: 'wechat_minigame', sourceType: 'html5' },
    })
  })

  it('passes resource id when generating from an engine artifact', async () => {
    requestMock.mockResolvedValue({ package: samplePackage })
    await generateChannelPackage('p1', {
      channel: 'steam_desktop',
      sourceType: 'engine_artifact',
      resourceId: 'r1',
    })
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'post',
      url: '/projects/p1/channel-packages',
      data: { channel: 'steam_desktop', sourceType: 'engine_artifact', resourceId: 'r1' },
    })
  })

  it('unwraps listed packages', async () => {
    requestMock.mockResolvedValue({ packages: [samplePackage] })
    await expect(listChannelPackages('p1')).resolves.toEqual([samplePackage])
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/projects/p1/channel-packages',
    })
  })

  it('deletes a channel package', async () => {
    requestMock.mockResolvedValue({ deleted: true })
    await deleteChannelPackage('p1', 'pkg-1')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'delete',
      url: '/projects/p1/channel-packages/pkg-1',
    })
  })

  it('unwraps channel guides', async () => {
    requestMock.mockResolvedValue({
      guides: [
        {
          channel: 'wechat_minigame',
          title: '微信小游戏',
          summary: '',
          markdown: '# 微信',
          steps: ['生成渠道包'],
          requirements: ['企业主体'],
          updatedAt: '2026-11-07',
          enabled: true,
          builtin: true,
          packageable: true,
          sortOrder: 10,
        },
      ],
    })
    const guides = await listChannelGuides()
    expect(guides).toHaveLength(1)
    expect(guides[0]?.channel).toBe('wechat_minigame')
    expect(requestMock).toHaveBeenCalledWith(cpHttp, {
      method: 'get',
      url: '/channel-guides',
    })
  })

  it('parses filename from content-disposition', async () => {
    const blob = new Blob(['zip'])
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      blob: async () => blob,
      headers: new Headers({
        'content-disposition': 'attachment; filename="game-wechat.zip"',
      }),
    })
    vi.stubGlobal('fetch', fetchMock)
    const result = await downloadChannelPackage('p1', 'pkg-1')
    expect(result.filename).toBe('game-wechat.zip')
    expect(result.blob).toBe(blob)
    expect(fetchMock).toHaveBeenCalledWith(
      '/cp/api/v1/projects/p1/channel-packages/pkg-1/download',
      { credentials: 'include' },
    )
  })

  it('falls back to a generated filename without content-disposition', async () => {
    const blob = new Blob(['zip'])
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: true,
        blob: async () => blob,
        headers: new Headers(),
      }),
    )
    const result = await downloadChannelPackage('p1', 'pkg-1')
    expect(result.filename).toBe('p1-channel.zip')
  })

  it('throws when the download response is not ok', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({
        ok: false,
        status: 500,
        blob: async () => new Blob([]),
        headers: new Headers(),
      }),
    )
    await expect(downloadChannelPackage('p1', 'pkg-1')).rejects.toThrow('下载失败（500）')
  })
})

import { describe, expect, it, vi } from 'vitest'

import {
  createTemplate,
  createTopic,
  deleteTemplate,
  getAdminTopics,
  getProjectFiles,
  reviewProject,
  updateQuota,
  updateTemplate,
  updateUserStatus,
} from './admin'
import { http } from './client'

describe('admin writes', () => {
  it('updates a user status', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: '1', status: 0 } },
    })
    await updateUserStatus('1', 0)
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/users/1/status',
      data: { status: 0 },
    })
  })

  it('loads all topics for management', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { topics: [] } },
    })
    await getAdminTopics()
    expect(spy).toHaveBeenCalledWith({ method: 'get', url: '/topics', params: { all: 1 } })
  })

  it('creates a topic', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: 1 } },
    })
    const input = {
      parentId: 1,
      code: 'game_action',
      name: '动作游戏',
      description: '',
      promptTemplate: '',
      sortOrder: 0,
      status: 1,
    }
    await createTopic(input)
    expect(spy).toHaveBeenCalledWith({ method: 'post', url: '/topics', data: input })
  })

  it('updates and deletes templates', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: 2 } },
    })
    const input = {
      templateName: 'Demo',
      description: '',
      topicId: 3,
      recommendScore: 80,
      status: 1,
    }
    await updateTemplate(2, input)
    expect(spy).toHaveBeenCalledWith({ method: 'put', url: '/templates/2', data: input })

    const deleteSpy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { deleted: true } },
    })
    await deleteTemplate(2)
    expect(deleteSpy).toHaveBeenCalledWith({ method: 'delete', url: '/templates/2' })
  })

  it('loads project files', async () => {
    vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { files: [{ path: 'index.html' }] } },
    })
    await expect(getProjectFiles('p1')).resolves.toEqual([{ path: 'index.html' }])
  })

  it('reviews a project', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: 'p1' } },
    })
    await reviewProject('p1', 'rejected', '违规')
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/projects/p1/review',
      data: { status: 'rejected', note: '违规' },
    })
  })

  it('creates a template', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { id: 3 } },
    })
    const input = { templateName: 'Demo', description: 'd', topicId: 11, recommendScore: 88 }
    await createTemplate('p1', input)
    expect(spy).toHaveBeenCalledWith({
      method: 'post',
      url: '/projects/p1/templates',
      data: input,
    })
  })

  it('updates a user quota', async () => {
    const spy = vi.spyOn(http, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { userId: '1' } },
    })
    const input = { llmTokensQuota: 100, sandboxMinutesQuota: 200, storageQuotaMb: 300 }
    await updateQuota('1', input)
    expect(spy).toHaveBeenCalledWith({
      method: 'put',
      url: '/quotas/1',
      data: input,
    })
  })
})

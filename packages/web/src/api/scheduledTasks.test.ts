import { describe, expect, it, vi } from 'vitest'

import { cpHttp } from './client'
import { deleteScheduledTask, parseScheduledTask } from './scheduledTasks'

describe('scheduledTasks api', () => {
  it('unwraps parse draft', async () => {
    vi.spyOn(cpHttp, 'request').mockResolvedValue({
      data: { code: 0, message: 'ok', result: { name: '检查', scheduleType: 'daily' } },
    } as never)
    await expect(parseScheduledTask('每天检查')).resolves.toMatchObject({ name: '检查' })
  })

  it('passes deleteProject flag', async () => {
    const spy = vi.spyOn(cpHttp, 'request').mockResolvedValue({
      data: { code: 0, message: 'ok', result: { id: 1 } },
    } as never)
    await deleteScheduledTask(1, true)
    expect(spy).toHaveBeenCalledWith(
      expect.objectContaining({
        method: 'delete',
        url: '/scheduled-tasks/1',
        params: { deleteProject: true },
      }),
    )
  })
})

import { describe, expect, it, vi } from 'vitest'

import { dpHttp } from './client'
import { listSkills } from './skills'

describe('listSkills', () => {
  it('unwraps the skills envelope', async () => {
    vi.spyOn(dpHttp, 'request').mockResolvedValueOnce({
      data: {
        code: 0,
        message: 'ok',
        result: { skills: [{ name: 'demo', description: 'demo', source: 'project' }] },
      },
    })

    await expect(listSkills('p1')).resolves.toEqual([
      { name: 'demo', description: 'demo', source: 'project' },
    ])
  })
})

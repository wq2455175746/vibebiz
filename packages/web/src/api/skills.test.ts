import { describe, expect, it, vi } from 'vitest'

import { cpHttp, dpHttp } from './client'
import { deleteMySkill, listMySkills, listSkills, uploadMySkill } from './skills'

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

describe('my skills', () => {
  it('lists public and user skills', async () => {
    const spy = vi.spyOn(cpHttp, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { skills: [{ name: 'demo', source: 'user' }] } },
    })
    await expect(listMySkills()).resolves.toEqual([{ name: 'demo', source: 'user' }])
    expect(spy).toHaveBeenCalledWith({ method: 'get', url: '/me/skills' })
  })

  it('uploads a zip to the user scope', async () => {
    const spy = vi.spyOn(cpHttp, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { name: 'demo', source: 'user' } },
    })
    const file = new File(['zip'], 'demo.zip', { type: 'application/zip' })
    await uploadMySkill(file, true)
    const config = spy.mock.calls[0][0] as { method: string; url: string; data: FormData }
    expect(config.method).toBe('post')
    expect(config.url).toBe('/me/skills')
    expect(config.data.get('overwrite')).toBe('true')
    expect(config.data.get('file')).toBe(file)
  })

  it('deletes my skill by encoded name', async () => {
    const spy = vi.spyOn(cpHttp, 'request').mockResolvedValueOnce({
      data: { code: 0, message: 'ok', result: { deleted: true } },
    })
    await deleteMySkill('demo skill')
    expect(spy).toHaveBeenCalledWith({ method: 'delete', url: '/me/skills/demo%20skill' })
  })
})

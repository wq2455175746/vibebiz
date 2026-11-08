import { describe, expect, it } from 'vitest'

import router from './index'

describe('card routes', () => {
  it('registers my card and discover routes behind auth', () => {
    expect(router.resolve('/my-card').name).toBe('my-card')
    expect(router.resolve('/discover').name).toBe('discover')
    expect(router.resolve('/my-card').meta.public).toBeFalsy()
    expect(router.resolve('/discover').meta.public).toBeFalsy()
  })

  it('marks share pages public', () => {
    const card = router.resolve('/share/card/me')
    const project = router.resolve('/share/project/demo-1-p1')
    expect(card.name).toBe('card-share')
    expect(card.meta.public).toBe(true)
    expect(project.name).toBe('project-share')
    expect(project.meta.public).toBe(true)
  })
})

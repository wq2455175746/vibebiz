import { describe, expect, it } from 'vitest'

import { draftToInput, emptyChannelDraft, validateChannelDraft } from './notificationChannels'

describe('notification channels', () => {
  it('validates name and URLs', () => {
    const draft = { ...emptyChannelDraft(), name: '研发群', url: 'https://example.com/hook' }
    expect(validateChannelDraft(draft)).toBeNull()
    expect(validateChannelDraft({ ...draft, name: '  ' })).toBe('请输入渠道名称')
    expect(validateChannelDraft({ ...draft, url: 'http://example.com' })).toBe(
      'Webhook 地址必须使用 HTTPS',
    )
  })

  it('validates email channels and strips empty secrets', () => {
    expect(
      validateChannelDraft({ ...emptyChannelDraft('email'), name: '邮箱', to: 'a@example.com' }),
    ).toBeNull()
    expect(validateChannelDraft({ ...emptyChannelDraft('email'), name: '邮箱', to: 'bad' })).toBe(
      '请输入有效邮箱',
    )

    const input = draftToInput({
      ...emptyChannelDraft('webhook'),
      name: ' 群 ',
      url: ' https://example.com ',
      secret: '',
    })
    expect(input).toMatchObject({ name: '群', config: { url: 'https://example.com' } })
    expect(input.config.secret).toBeUndefined()
  })
})

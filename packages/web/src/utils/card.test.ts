import { describe, expect, it } from 'vitest'

import {
  CARD_MAX_ITEMS,
  CARD_MAX_TAG_LENGTH,
  addCardTag,
  canAddCardItem,
  cardContentType,
  decodeCard,
  decodeCards,
  emptyCard,
  normalizeCard,
  pageCards,
  sortDiscoverCards,
  validateCardUpload,
} from './card'
import type { BusinessCard, CardProjectItem } from '@/types'

function item(projectId: string): CardProjectItem {
  return {
    projectId,
    name: projectId,
    description: '',
    coverUrl: null,
    type: 'html',
    shareUrl: `/p/${projectId}`,
    addedAt: '2026-11-08T00:00:00.000Z',
  }
}

function card(id: string, overrides: Partial<BusinessCard> = {}): BusinessCard {
  return { ...emptyCard(), id, ...overrides }
}

describe('cardContentType', () => {
  it('maps preview paths to card content types', () => {
    expect(cardContentType('index.html')).toBe('html')
    expect(cardContentType('app.htm')).toBe('html')
    expect(cardContentType('docs/a.pdf')).toBe('pdf')
    expect(cardContentType('assets/cover.JPG')).toBe('image')
    expect(cardContentType('media/demo.mp4')).toBe('video')
    expect(cardContentType('README.md')).toBe('document')
    expect(cardContentType('unknown.bin')).toBe('html')
  })

  it('ignores query and hash suffixes', () => {
    expect(cardContentType('demo.pdf?v=2#page=1')).toBe('pdf')
  })
})

describe('addCardTag', () => {
  it('adds a trimmed tag', () => {
    expect(addCardTag([], '  Vue  ')).toEqual({ tags: ['Vue'], error: '' })
  })

  it('rejects duplicates and overlong tags', () => {
    expect(addCardTag(['Vue'], 'Vue').error).toBe('标签已存在')
    expect(addCardTag([], 'a'.repeat(CARD_MAX_TAG_LENGTH + 1)).error).toContain('不能超过')
  })

  it('rejects a sixth tag', () => {
    expect(addCardTag(['a', 'b', 'c', 'd', 'e'], 'f').error).toContain('最多')
  })
})

describe('canAddCardItem', () => {
  it('rejects duplicates and more than five items', () => {
    const items = [item('p1')]
    expect(canAddCardItem(items, 'p2')).toBe(true)
    expect(canAddCardItem(items, 'p1')).toBe(false)
    const full = Array.from({ length: CARD_MAX_ITEMS }, (_, index) => item(`p${index}`))
    expect(canAddCardItem(full, 'p9')).toBe(false)
  })
})

describe('pageCards', () => {
  it('returns the requested slice', () => {
    expect(pageCards([1, 2, 3, 4, 5, 6, 7], 2, 3)).toEqual([4, 5, 6])
    expect(pageCards([1, 2], 0, 3)).toEqual([1, 2])
  })
})

describe('normalizeCard', () => {
  it('keeps a featured project only when it exists in items', () => {
    expect(
      normalizeCard(
        card('demo-1', {
          items: [item('p1')],
          featured: { kind: 'project', projectId: 'p2' },
        }),
      ).featured,
    ).toBeNull()
    expect(
      normalizeCard(
        card('demo-1', {
          items: [item('p1')],
          featured: { kind: 'project', projectId: 'p1' },
        }),
      ).featured,
    ).toEqual({ kind: 'project', projectId: 'p1' })
  })

  it('trims and deduplicates tags', () => {
    expect(normalizeCard(card('demo-1', { tags: [' Vue ', 'Vue', '', 'Go'] })).tags).toEqual([
      'Vue',
      'Go',
    ])
  })
})

describe('decode', () => {
  it('round-trips a card and rejects broken json', () => {
    const source = card('demo-1', { headline: 'hello' })
    expect(decodeCard(JSON.stringify(source))?.headline).toBe('hello')
    expect(decodeCard('{oops')).toBeNull()
    expect(decodeCard(null)).toBeNull()
  })

  it('decodes a card list and skips invalid entries', () => {
    const raw = JSON.stringify([card('demo-1'), card('demo-2'), null, 'x'])
    expect(decodeCards(raw).map((entry) => entry.id)).toEqual(['demo-1', 'demo-2'])
    expect(decodeCards('[]')).toEqual([])
  })
})

describe('sortDiscoverCards', () => {
  it('puts the own card first', () => {
    const cards = [card('demo-1'), card('me'), card('demo-2')]
    expect(sortDiscoverCards(cards).map((entry) => entry.id)).toEqual(['me', 'demo-2', 'demo-1'])
  })
})

describe('validateCardUpload', () => {
  it('accepts pdf and images under 2MB', () => {
    expect(validateCardUpload({ name: 'a.pdf', size: 10, type: 'application/pdf' })).toBe('')
    expect(validateCardUpload({ name: 'a.png', size: 10, type: 'image/png' })).toBe('')
  })

  it('rejects other types and oversized files', () => {
    expect(validateCardUpload({ name: 'a.zip', size: 10, type: 'application/zip' })).toContain(
      'PDF',
    )
    expect(
      validateCardUpload({ name: 'a.png', size: 3 * 1024 * 1024, type: 'image/png' }),
    ).toContain('2MB')
  })
})

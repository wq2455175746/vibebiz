import { describe, expect, it } from 'vitest'

import type { EvolutionVersion } from '@/types/evolution'
import {
  OFFICIAL_USER_ID,
  OFFICIAL_VERSION_ID,
  buildAgentDecision,
  buildEvolutionPreviewUrl,
  classifyProposal,
  groupVersionsByUser,
  latestVersionsByUser,
  statusLabel,
  withQuery,
} from './evolution'

function version(
  id: string,
  userId: string,
  updatedAt: string,
  overrides: Partial<EvolutionVersion> = {},
): EvolutionVersion {
  return {
    id,
    userId,
    userName: userId,
    label: id,
    createdAt: updatedAt,
    updatedAt,
    status: 'preview',
    scenario: 'baseline',
    summary: '',
    proposalIds: [],
    ...overrides,
  }
}

describe('classifyProposal', () => {
  it('classifies defects as bug', () => {
    expect(classifyProposal('删除按钮点击没有反应')).toBe('bug')
    expect(classifyProposal('页面报错，打不开')).toBe('bug')
  })

  it('classifies page adjustments as visual', () => {
    expect(classifyProposal('首页卡片太挤了，希望更紧凑')).toBe('visual')
    expect(classifyProposal('输入框的布局和间距需要优化')).toBe('visual')
  })

  it('classifies capability changes as feature', () => {
    expect(classifyProposal('希望支持多人协作')).toBe('feature')
    expect(classifyProposal('接入一个新的数据库')).toBe('feature')
  })

  it('defaults unknown ideas to feature instead of accepting blindly', () => {
    expect(classifyProposal('我想到了一个全新的玩法')).toBe('feature')
  })
})

describe('buildAgentDecision', () => {
  it('patches bug reports directly', () => {
    const decision = buildAgentDecision('按钮失效了')
    expect(decision.kind).toBe('bug')
    expect(decision.action).toBe('patch')
    expect(decision.scenario).toBe('bugfix')
  })

  it('previews visual changes and asks for confirmation', () => {
    const decision = buildAgentDecision('首页导航布局更紧凑一些')
    expect(decision.kind).toBe('visual')
    expect(decision.action).toBe('preview')
    expect(decision.scenario).toBe('compact-sidebar')
  })

  it('pushes back on feature requests', () => {
    const decision = buildAgentDecision('支持多人同时编辑')
    expect(decision.kind).toBe('feature')
    expect(decision.action).toBe('pushback')
  })
})

describe('groupVersionsByUser', () => {
  const versions = [
    version('current-old', '7', '2026-11-01T00:00:00.000Z'),
    version('current-new', '7', '2026-11-07T00:00:00.000Z'),
    version('other', '9', '2026-11-08T00:00:00.000Z'),
    version(OFFICIAL_VERSION_ID, OFFICIAL_USER_ID, '2026-11-09T00:00:00.000Z'),
  ]

  it('pins the current user first and the official baseline last', () => {
    const groups = groupVersionsByUser(versions, '7')
    expect(groups.map((group) => group.userId)).toEqual(['7', '9', OFFICIAL_USER_ID])
    expect(groups[0].latest.id).toBe('current-new')
    expect(groups[0].versions.map((item) => item.id)).toEqual(['current-new', 'current-old'])
  })

  it('returns the latest version per user', () => {
    const latest = latestVersionsByUser(versions, '7')
    expect(latest.map((item) => item.id)).toEqual(['current-new', 'other', OFFICIAL_VERSION_ID])
  })
})

describe('preview URL', () => {
  it('appends query parameters without losing the path', () => {
    expect(withQuery('/a/b.html', { scenario: 'soft-cards', user: '林 岚' })).toBe(
      '/a/b.html?scenario=soft-cards&user=%E6%9E%97+%E5%B2%9A',
    )
  })

  it('builds an absolute preview URL', () => {
    const url = buildEvolutionPreviewUrl(
      'http://localhost:5173/evolution/origin_version/index.html',
      version('v1', '7', '2026-11-08T00:00:00.000Z', {
        label: 'v7.2',
        userName: '林岚',
        scenario: 'compact-sidebar',
      }),
      '王海广',
    )
    expect(url).toContain('scenario=compact-sidebar')
    expect(url).toContain('version=v7.2')
    expect(url).toContain('viewer=')
  })
})

describe('statusLabel', () => {
  it('maps known statuses', () => {
    expect(statusLabel('preview')).toBe('待确认')
    expect(statusLabel('queued')).toBe('待处理')
    expect(statusLabel('unknown')).toBe('unknown')
  })
})

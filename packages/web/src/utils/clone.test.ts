import { describe, expect, it } from 'vitest'

import type { ProjectStage } from '@/types'
import type { CloneDraft, CloneDraftAsset } from '@/types/clone'
import {
  buildCloneStagePrompt,
  CLONE_STAGES,
  cloneStageDefinition,
  cloneStagesFromProject,
  createInitialCloneStages,
  isCloneUrl,
  nextCoverAfterRemoval,
  removeCloneAsset,
  validateCloneAsset,
  validateCloneDraft,
} from './clone'

const draft: CloneDraft = {
  name: '克隆示例',
  description: '复刻一个示例站点',
  sourceUrl: 'https://example.com',
}

function asset(id: string, role: CloneDraftAsset['role'], name = `${id}.png`): CloneDraftAsset {
  return { id, name, size: 1024, mime: role === 'image' ? 'image/png' : 'application/zip', role }
}

function stage(stageKey: string, status: ProjectStage['status']): ProjectStage {
  return {
    stageKey,
    seq: 1,
    title: stageKey,
    roleCode: 'bob',
    roleName: 'Bob',
    artifactPath: `docs/clone/${stageKey}.md`,
    workspace: 'document',
    status,
  }
}

describe('validateCloneDraft', () => {
  it('requires name and description', () => {
    expect(validateCloneDraft({ name: ' ', description: ' ', sourceUrl: '' })).toEqual({
      name: '请填写名称',
      description: '请填写描述',
    })
  })

  it('limits name and description length', () => {
    const errors = validateCloneDraft({
      name: 'a'.repeat(81),
      description: 'b'.repeat(2001),
      sourceUrl: '',
    })
    expect(errors.name).toContain('80')
    expect(errors.description).toContain('2000')
  })

  it('accepts only http and https urls', () => {
    expect(isCloneUrl('')).toBe(true)
    expect(isCloneUrl('https://example.com')).toBe(true)
    expect(isCloneUrl('http://example.com/a?b=1')).toBe(true)
    expect(isCloneUrl('ftp://example.com')).toBe(false)
    expect(isCloneUrl('not-a-url')).toBe(false)
    expect(validateCloneDraft({ ...draft, sourceUrl: 'ftp://x' }).sourceUrl).toBeTruthy()
  })
})

describe('validateCloneAsset', () => {
  it('limits images to nine', () => {
    const images = Array.from({ length: 9 }, (_, index) => asset(`img-${index}`, 'image'))
    const file = new File(['x'], 'ten.png', { type: 'image/png' })
    expect(validateCloneAsset(file, images, 'image')).toBe('图片最多 9 张')
  })

  it('rejects unsupported image types and oversized images', () => {
    const gif = new File(['x'], 'a.gif', { type: 'image/gif' })
    const svg = new File(['x'], 'a.svg', { type: 'image/svg+xml' })
    const big = new File([new Uint8Array(10 * 1024 * 1024 + 1)], 'a.png', { type: 'image/png' })
    expect(validateCloneAsset(gif, [], 'image')).toBe('')
    expect(validateCloneAsset(svg, [], 'image')).toContain('JPG')
    expect(validateCloneAsset(big, [], 'image')).toContain('10MB')
  })

  it('allows only one zip unit test file', () => {
    const zip = new File(['x'], 'tests.zip', { type: 'application/zip' })
    expect(validateCloneAsset(zip, [], 'unit_test')).toBe('')
    expect(validateCloneAsset(zip, [asset('t', 'unit_test', 'a.zip')], 'unit_test')).toContain(
      '1 个',
    )
    expect(validateCloneAsset(new File(['x'], 'a.txt'), [], 'unit_test')).toContain('zip')
  })

  it('limits docs to 100MB', () => {
    const big = new File([new Uint8Array(100 * 1024 * 1024 + 1)], 'a.pdf', {
      type: 'application/pdf',
    })
    expect(validateCloneAsset(big, [], 'docs')).toContain('100MB')
  })
})

describe('clone assets', () => {
  it('falls back to the first remaining image after removing the cover', () => {
    const assets = [asset('img-1', 'image'), asset('img-2', 'image')]
    expect(nextCoverAfterRemoval(assets, 'img-1', 'img-1')).toBe('img-2')
    expect(nextCoverAfterRemoval(assets, 'img-2', 'img-1')).toBe('img-2')
    expect(nextCoverAfterRemoval(assets, 'img-1', 'img-2')).toBe('img-1')
    expect(nextCoverAfterRemoval([asset('img-1', 'image')], 'img-1', 'img-1')).toBeNull()
  })

  it('removes assets by id', () => {
    const assets = [asset('img-1', 'image'), asset('doc-1', 'docs')]
    expect(removeCloneAsset(assets, 'img-1').map((item) => item.id)).toEqual(['doc-1'])
  })
})

describe('clone stages', () => {
  it('starts with only base design active', () => {
    const stages = createInitialCloneStages()
    expect(stages.base_design.status).toBe('active')
    expect(stages.architecture.status).toBe('locked')
    expect(stages.artifacts.status).toBe('locked')
  })

  it('maps backend stages into the five clone stages', () => {
    const stages = cloneStagesFromProject([
      stage('base_design', 'done'),
      stage('architecture', 'active'),
      stage('test', 'locked'),
    ])
    expect(stages.base_design.status).toBe('done')
    expect(stages.architecture.status).toBe('active')
    expect(stages.test.status).toBe('locked')
    expect(stages.artifacts.status).toBe('locked')
  })

  it('defines five stages without delivery', () => {
    expect(CLONE_STAGES.map((item) => item.key)).toEqual([
      'base_design',
      'architecture',
      'test',
      'development',
      'artifacts',
    ])
    expect(cloneStageDefinition('test').workspace).toBe('test')
  })

  it('builds stage prompts with target context', () => {
    const prompt = buildCloneStagePrompt(cloneStageDefinition('base_design'), {
      name: 'Acme',
      description: 'clone acme',
      sourceUrl: 'https://acme.example.com',
    })
    expect(prompt).toContain('基础设计')
    expect(prompt).toContain('docs/clone/base-design.md')
    expect(prompt).toContain('https://acme.example.com')
    expect(prompt).toContain('技术栈推测')
  })
})

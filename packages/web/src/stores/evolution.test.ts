import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it } from 'vitest'

import type { EvolutionViewer } from '@/types/evolution'
import type { EvolutionStorage } from '@/utils/evolutionState'

import { useEvolutionStore } from './evolution'

const viewer: EvolutionViewer = { id: '42', name: '测试用户' }
let storage: EvolutionStorage

function memoryStorage(): EvolutionStorage {
  const data = new Map<string, string>()
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => {
      data.set(key, value)
    },
  }
}

function setup() {
  const store = useEvolutionStore()
  store.init(viewer, storage)
  return store
}

beforeEach(() => {
  setActivePinia(createPinia())
  storage = memoryStorage()
})

describe('evolution store', () => {
  it('seeds the official baseline and a copy for the viewer', () => {
    const store = setup()

    expect(store.versions.some((version) => version.userId === 'official')).toBe(true)
    expect(store.versions.some((version) => version.userId === '42')).toBe(true)
    expect(store.selectedVersion?.userId).toBe('42')
    expect(store.versionGroups[0].userId).toBe('42')
  })

  it('creates a preview version for a bug proposal', () => {
    const store = setup()
    const before = store.versions.filter((version) => version.userId === '42').length

    const proposal = store.submitProposal('删除按钮点击没有反应')

    expect(proposal?.kind).toBe('bug')
    expect(store.versions.filter((version) => version.userId === '42')).toHaveLength(before + 1)
    expect(store.selectedVersion?.status).toBe('preview')
    const messages = store.activeChat
    expect(messages[messages.length - 1]?.role).toBe('agent')
  })

  it('pushes back on feature proposals without creating a version', () => {
    const store = setup()
    const before = store.versions.filter((version) => version.userId === '42').length

    const proposal = store.submitProposal('希望支持多人同时编辑同一个项目')

    expect(proposal?.kind).toBe('feature')
    expect(proposal?.status).toBe('reviewing')
    expect(store.versions.filter((version) => version.userId === '42')).toHaveLength(before)
  })

  it('appends chat messages without generating a version', () => {
    const store = setup()
    const versionsBefore = store.versions.length
    const messagesBefore = store.activeChat.length

    expect(store.submitChat('这个页面的颜色还可以再克制一点')).toBe(true)

    expect(store.versions).toHaveLength(versionsBefore)
    expect(store.activeChat.length).toBe(messagesBefore + 2)
  })

  it('confirms a preview version and records the acceptance reply', () => {
    const store = setup()
    store.submitProposal('首页卡片布局再紧凑一点')

    const preview = store.latestPreviewVersion
    expect(preview).not.toBeNull()

    expect(store.confirmVersion(preview?.id ?? '')).toBe(true)
    expect(store.selectedVersion?.status).toBe('accepted')
    const messages = store.activeChat
    expect(messages[messages.length - 1]?.content).toContain('已收录你的意见版本')
  })
})

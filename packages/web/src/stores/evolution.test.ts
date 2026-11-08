import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as evolutionApi from '@/api/evolution'
import type { EvolutionProposal, EvolutionVersion } from '@/types/evolution'

import { useEvolutionStore } from './evolution'

vi.mock('@/api/evolution', () => ({
  listVersions: vi.fn(),
  ensureDraft: vi.fn(),
  getConversation: vi.fn(),
  submitProposal: vi.fn(),
  submitChat: vi.fn(),
  confirmVersion: vi.fn(),
}))

const listVersionsMock = vi.mocked(evolutionApi.listVersions)
const ensureDraftMock = vi.mocked(evolutionApi.ensureDraft)
const getConversationMock = vi.mocked(evolutionApi.getConversation)
const submitProposalMock = vi.mocked(evolutionApi.submitProposal)
const submitChatMock = vi.mocked(evolutionApi.submitChat)
const confirmVersionMock = vi.mocked(evolutionApi.confirmVersion)

function version(
  id: string,
  userId: string,
  status: EvolutionVersion['status'] = 'preview',
  scenario = 'baseline',
): EvolutionVersion {
  return {
    id,
    userId,
    userName: userId === '7' ? '王海广' : '林岚',
    label: id,
    createdAt: '2026-11-08T10:00:00.000Z',
    updatedAt: '2026-11-08T10:00:00.000Z',
    status,
    scenario,
    summary: '摘要',
    originVersion: 'v1.2026.11',
    proposalIds: [],
  }
}

const ownVersion = version('v-7-1', '7')
const otherVersion = version('v-1001-2', '1001', 'preview', 'compact-sidebar')
const officialVersion = version('v-official-1', 'official', 'released')

function seedVersions(versions: EvolutionVersion[] = [ownVersion, otherVersion, officialVersion]) {
  listVersionsMock.mockResolvedValue({
    projectId: 'vibebiz-evolve',
    originVersion: 'v1.2026.11',
    versions,
  })
}

beforeEach(() => {
  setActivePinia(createPinia())
  listVersionsMock.mockReset()
  ensureDraftMock.mockReset()
  getConversationMock.mockReset()
  submitProposalMock.mockReset()
  submitChatMock.mockReset()
  confirmVersionMock.mockReset()
  getConversationMock.mockImplementation((userId: string) =>
    Promise.resolve({
      messages: [
        {
          id: `c-${userId}-1`,
          createdAt: '2026-11-08T10:00:00.000Z',
          role: 'agent',
          userName: '集思进化助手',
          content: '你好',
        },
      ],
      proposals: [],
    }),
  )
})

describe('evolution store', () => {
  it('loads versions and selects the viewer copy', async () => {
    seedVersions()
    const store = useEvolutionStore()

    await store.init({ id: '7', name: '王海广' })

    expect(store.selectedVersion?.id).toBe('v-7-1')
    expect(store.versionGroups[0].userId).toBe('7')
    expect(store.activeChat).toHaveLength(1)
    expect(getConversationMock).toHaveBeenCalledWith('7')
  })

  it('creates a draft when the viewer has no version yet', async () => {
    const draft = version('v-7-1', '7', 'draft')
    listVersionsMock
      .mockResolvedValueOnce({
        projectId: 'vibebiz-evolve',
        originVersion: 'v1.2026.11',
        versions: [otherVersion, officialVersion],
      })
      .mockResolvedValue({
        projectId: 'vibebiz-evolve',
        originVersion: 'v1.2026.11',
        versions: [draft, otherVersion, officialVersion],
      })
    ensureDraftMock.mockResolvedValue({ version: draft })
    const store = useEvolutionStore()

    await store.init({ id: '7', name: '王海广' })

    expect(ensureDraftMock).toHaveBeenCalledTimes(1)
    expect(store.selectedVersion?.id).toBe('v-7-1')
    expect(store.isOwnVersion).toBe(true)
  })

  it('creates a proposal and selects the new version', async () => {
    const next = version('v-7-2', '7', 'preview', 'compact-sidebar')
    const proposal: EvolutionProposal = {
      id: 'p-1',
      createdAt: '2026-11-08T10:00:00.000Z',
      userId: '7',
      userName: '王海广',
      kind: 'visual',
      content: '首页卡片再紧凑一点',
      status: 'queued',
      agentReply: '已生成候选版本',
      versionId: 'v-7-2',
    }
    seedVersions()
    submitProposalMock.mockResolvedValue({ proposal, version: next })
    listVersionsMock.mockResolvedValue({
      projectId: 'vibebiz-evolve',
      originVersion: 'v1.2026.11',
      versions: [next, ownVersion, otherVersion, officialVersion],
    })
    const store = useEvolutionStore()
    await store.init({ id: '7', name: '王海广' })

    const created = await store.submitProposal('首页卡片再紧凑一点')

    expect(created.kind).toBe('visual')
    expect(store.selectedVersion?.id).toBe('v-7-2')
    expect(store.selectedVersion?.scenario).toBe('compact-sidebar')
  })

  it('keeps the viewer version when a feature proposal is pushed back', async () => {
    const proposal: EvolutionProposal = {
      id: 'p-2',
      createdAt: '2026-11-08T10:00:00.000Z',
      userId: '7',
      userName: '王海广',
      kind: 'feature',
      content: '支持多人协作',
      status: 'reviewing',
      agentReply: '暂不生成',
      versionId: null,
    }
    seedVersions()
    submitProposalMock.mockResolvedValue({ proposal, version: null })
    const store = useEvolutionStore()
    await store.init({ id: '7', name: '王海广' })

    await store.submitProposal('支持多人协作')

    expect(store.selectedVersion?.id).toBe('v-7-1')
  })

  it('reloads the conversation after chat', async () => {
    seedVersions()
    submitChatMock.mockResolvedValue({ messages: [] })
    const store = useEvolutionStore()
    await store.init({ id: '7', name: '王海广' })
    getConversationMock.mockClear()

    await store.submitChat('这个颜色还可以再克制一点')

    expect(submitChatMock).toHaveBeenCalledWith('这个颜色还可以再克制一点')
    expect(getConversationMock).toHaveBeenCalledWith('7')
  })

  it('loads another user conversation when selecting their version', async () => {
    seedVersions()
    const store = useEvolutionStore()
    await store.init({ id: '7', name: '王海广' })

    await store.selectVersion('v-1001-2')

    expect(store.activeUserId).toBe('1001')
    expect(store.isOwnVersion).toBe(false)
    expect(getConversationMock).toHaveBeenCalledWith('1001')
  })

  it('refreshes the selected version after confirmation', async () => {
    const accepted = version('v-7-1', '7', 'accepted')
    seedVersions()
    confirmVersionMock.mockResolvedValue({ version: accepted })
    const store = useEvolutionStore()
    await store.init({ id: '7', name: '王海广' })
    listVersionsMock.mockResolvedValue({
      projectId: 'vibebiz-evolve',
      originVersion: 'v1.2026.11',
      versions: [accepted, otherVersion, officialVersion],
    })

    await store.confirmVersion('v-7-1')

    expect(store.selectedVersion?.status).toBe('accepted')
  })
})

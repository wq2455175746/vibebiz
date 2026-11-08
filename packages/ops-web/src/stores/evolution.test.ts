import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import * as evolutionApi from '@/api/evolution'
import type {
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionTask,
  EvolutionVersion,
} from '@/types/evolution'

import { useEvolutionAdminStore } from './evolution'

vi.mock('@/api/evolution', () => ({
  getSnapshot: vi.fn(),
  getConversation: vi.fn(),
  reply: vi.fn(),
  submitTask: vi.fn(),
  updateTask: vi.fn(),
}))

const getSnapshotMock = vi.mocked(evolutionApi.getSnapshot)
const getConversationMock = vi.mocked(evolutionApi.getConversation)
const replyMock = vi.mocked(evolutionApi.reply)
const submitTaskMock = vi.mocked(evolutionApi.submitTask)
const updateTaskMock = vi.mocked(evolutionApi.updateTask)

function version(
  id: string,
  userId: string,
  userName: string,
  updatedAt: string,
  status: EvolutionVersion['status'] = 'preview',
): EvolutionVersion {
  return {
    id,
    userId,
    userName,
    label: id,
    createdAt: updatedAt,
    updatedAt,
    status,
    scenario: 'baseline',
    summary: `摘要 ${id}`,
    originVersion: 'v1.2026.11',
    proposalIds: [],
  }
}

const versionA = version('v-1001-2', '1001', '林岚', '2026-11-08T10:00:00.000Z')
const versionB = version('v-1002-1', '1002', '陈墨', '2026-11-08T09:00:00.000Z')
const official = version(
  'v-official-1',
  'official',
  'Vibebiz 官方',
  '2026-11-01T00:00:00.000Z',
  'released',
)

const proposalA: EvolutionProposal = {
  id: 'p-1001-2',
  createdAt: '2026-11-08T10:00:00.000Z',
  userId: '1001',
  userName: '林岚',
  kind: 'visual',
  content: '首页卡片更紧凑',
  status: 'accepted',
  agentReply: '已生成候选版本',
  versionId: 'v-1001-2',
}

const proposalB: EvolutionProposal = {
  id: 'p-1002-1',
  createdAt: '2026-11-08T09:00:00.000Z',
  userId: '1002',
  userName: '陈墨',
  kind: 'visual',
  content: '首屏更聚焦',
  status: 'queued',
  agentReply: '已生成候选版本',
  versionId: 'v-1002-1',
}

const task: EvolutionTask = {
  id: 't-1',
  versionId: 'v-1002-1',
  userId: '1002',
  userName: '陈墨',
  versionLabel: 'v-1002-1',
  originVersion: 'v1.2026.11',
  chatSummary: '对话摘要',
  status: 'ci',
  createdAt: '2026-11-08T09:10:00.000Z',
  updatedAt: '2026-11-08T09:10:00.000Z',
}

const message: EvolutionChatMessage = {
  id: 'c-1',
  createdAt: '2026-11-08T10:00:00.000Z',
  role: 'agent',
  userName: '集思进化助手',
  content: '你好',
}

function snapshot(overrides: Partial<evolutionApi.EvolutionSnapshot> = {}) {
  return {
    projectId: 'vibebiz-evolve',
    originVersion: 'v1.2026.11',
    versions: [versionA, versionB, official],
    proposals: [proposalA, proposalB],
    tasks: [task],
    ...overrides,
  }
}

beforeEach(() => {
  setActivePinia(createPinia())
  getSnapshotMock.mockReset()
  getConversationMock.mockReset()
  replyMock.mockReset()
  submitTaskMock.mockReset()
  updateTaskMock.mockReset()
  getConversationMock.mockResolvedValue({ messages: [message], proposals: [proposalA] })
})

describe('ops evolution store', () => {
  it('builds table rows without the official baseline', async () => {
    getSnapshotMock.mockResolvedValue(snapshot())
    const store = useEvolutionAdminStore()

    await store.init()

    expect(store.rows.map((row) => row.version.id)).toEqual(['v-1001-2', 'v-1002-1'])
    expect(store.rows[0].proposalCount).toBe(1)
    expect(store.rows[1].task?.id).toBe('t-1')
    expect(store.stats).toEqual({
      users: 2,
      versions: 2,
      pendingProposals: 1,
      tasks: 1,
    })
  })

  it('loads the selected user conversation', async () => {
    getSnapshotMock.mockResolvedValue(snapshot())
    const store = useEvolutionAdminStore()
    await store.init()

    await store.selectVersion('v-1001-2')

    expect(store.selectedUserId).toBe('1001')
    expect(store.selectedChat).toHaveLength(1)
    expect(getConversationMock).toHaveBeenCalledWith('1001')
  })

  it('appends an admin reply and reloads the conversation', async () => {
    getSnapshotMock.mockResolvedValue(snapshot())
    replyMock.mockResolvedValue({ ...message, role: 'admin', userName: '运营管理员' })
    const store = useEvolutionAdminStore()
    await store.init()

    await store.reply('v-1001-2', '这个方向可以，补充验收标准。')

    expect(replyMock).toHaveBeenCalledWith('v-1001-2', '这个方向可以，补充验收标准。')
    expect(getConversationMock).toHaveBeenCalledWith('1001')
  })

  it('submits a version to the coding plan and refreshes', async () => {
    getSnapshotMock.mockResolvedValueOnce(snapshot()).mockResolvedValue(
      snapshot({
        tasks: [
          { ...task, id: 't-2', versionId: 'v-1001-2', versionLabel: 'v-1001-2', status: 'queued' },
          task,
        ],
        versions: [{ ...versionA, status: 'submitted' }, versionB, official],
      }),
    )
    submitTaskMock.mockResolvedValue({
      ...task,
      id: 't-2',
      versionId: 'v-1001-2',
      versionLabel: 'v-1001-2',
      status: 'queued',
    })
    const store = useEvolutionAdminStore()
    await store.init()

    await store.submitTask('v-1001-2')

    expect(store.taskByVersion.get('v-1001-2')?.id).toBe('t-2')
    expect(store.rows.find((row) => row.version.id === 'v-1001-2')?.version.status).toBe(
      'submitted',
    )
  })

  it('advances a task status and refreshes', async () => {
    getSnapshotMock.mockResolvedValueOnce(snapshot()).mockResolvedValue(
      snapshot({
        tasks: [{ ...task, status: 'released' }],
      }),
    )
    updateTaskMock.mockResolvedValue({ ...task, status: 'released' })
    const store = useEvolutionAdminStore()
    await store.init()

    await store.advanceTask('t-1', 'released')

    expect(updateTaskMock).toHaveBeenCalledWith('t-1', 'released')
    expect(store.tasks[0].status).toBe('released')
  })
})

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import * as evolutionApi from '@/api/evolution'
import type {
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionRow,
  EvolutionTask,
  EvolutionVersion,
  EvolutionVersionGroup,
  TaskStatus,
} from '@/types/evolution'
import { OFFICIAL_USER_ID, groupVersionsByUser } from '@/utils/evolution'

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败'
}

export const useEvolutionAdminStore = defineStore('evolution-admin', () => {
  const versions = ref<EvolutionVersion[]>([])
  const proposals = ref<EvolutionProposal[]>([])
  const tasks = ref<EvolutionTask[]>([])
  const originVersion = ref('')
  const projectId = ref('vibebiz-evolve')
  const messagesByUser = ref<Record<string, EvolutionChatMessage[]>>({})
  const proposalsByUser = ref<Record<string, EvolutionProposal[]>>({})
  const selectedVersionId = ref<string | null>(null)
  const selectedUserId = ref<string | null>(null)
  const loading = ref(false)
  const conversationLoading = ref(false)
  const error = ref('')

  const versionGroups = computed<EvolutionVersionGroup[]>(() =>
    groupVersionsByUser(versions.value, ''),
  )
  const selectedVersion = computed<EvolutionVersion | null>(
    () => versions.value.find((version) => version.id === selectedVersionId.value) ?? null,
  )
  const selectedChat = computed<EvolutionChatMessage[]>(() =>
    selectedUserId.value ? (messagesByUser.value[selectedUserId.value] ?? []) : [],
  )
  const selectedProposals = computed<EvolutionProposal[]>(() =>
    selectedUserId.value ? (proposalsByUser.value[selectedUserId.value] ?? []) : [],
  )
  const sortedTasks = computed<EvolutionTask[]>(() =>
    [...tasks.value].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)),
  )
  const taskByVersion = computed(() => {
    const map = new Map<string, EvolutionTask>()
    for (const task of tasks.value) {
      map.set(task.versionId, task)
    }
    return map
  })
  const rows = computed<EvolutionRow[]>(() =>
    versions.value
      .filter((version) => version.userId !== OFFICIAL_USER_ID && version.status !== 'draft')
      .map((version) => {
        const related = proposals.value.filter((proposal) => proposal.versionId === version.id)
        return {
          version,
          proposalCount: related.length,
          latestProposal: related[0] ?? null,
          task: taskByVersion.value.get(version.id) ?? null,
        }
      }),
  )
  const stats = computed(() => ({
    users: new Set(rows.value.map((row) => row.version.userId)).size,
    versions: rows.value.length,
    pendingProposals: proposals.value.filter(
      (proposal) => proposal.status === 'queued' || proposal.status === 'reviewing',
    ).length,
    tasks: tasks.value.length,
  }))

  async function refresh(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const snapshot = await evolutionApi.getSnapshot()
      versions.value = snapshot.versions ?? []
      proposals.value = snapshot.proposals ?? []
      tasks.value = snapshot.tasks ?? []
      originVersion.value = snapshot.originVersion || ''
      projectId.value = snapshot.projectId || 'vibebiz-evolve'
      if (
        selectedVersionId.value &&
        !versions.value.some((version) => version.id === selectedVersionId.value)
      ) {
        selectedVersionId.value = null
        selectedUserId.value = null
      }
    } catch (err) {
      error.value = errorMessage(err)
      throw err
    } finally {
      loading.value = false
    }
  }

  async function loadConversation(userId: string, force = false): Promise<void> {
    const id = userId.trim()
    if (!id) {
      return
    }
    if (!force && messagesByUser.value[id] && proposalsByUser.value[id]) {
      return
    }
    conversationLoading.value = true
    try {
      const result = await evolutionApi.getConversation(id)
      messagesByUser.value = { ...messagesByUser.value, [id]: result.messages ?? [] }
      proposalsByUser.value = { ...proposalsByUser.value, [id]: result.proposals ?? [] }
    } catch (err) {
      error.value = errorMessage(err)
      throw err
    } finally {
      conversationLoading.value = false
    }
  }

  async function init(): Promise<void> {
    await refresh()
  }

  async function selectVersion(versionId: string): Promise<void> {
    const version = versions.value.find((item) => item.id === versionId)
    if (!version) {
      return
    }
    selectedVersionId.value = version.id
    selectedUserId.value = version.userId
    await loadConversation(version.userId)
  }

  async function reply(versionId: string, content: string): Promise<EvolutionChatMessage> {
    const version = versions.value.find((item) => item.id === versionId)
    const message = await evolutionApi.reply(versionId, content)
    if (version) {
      selectedVersionId.value = version.id
      selectedUserId.value = version.userId
      await loadConversation(version.userId, true)
    }
    return message
  }

  async function submitTask(versionId: string): Promise<EvolutionTask> {
    const task = await evolutionApi.submitTask(versionId)
    await refresh()
    return task
  }

  async function advanceTask(taskId: string, status: TaskStatus): Promise<EvolutionTask> {
    const task = await evolutionApi.updateTask(taskId, status)
    await refresh()
    return task
  }

  return {
    versions,
    proposals,
    tasks,
    originVersion,
    projectId,
    messagesByUser,
    proposalsByUser,
    selectedVersionId,
    selectedUserId,
    loading,
    conversationLoading,
    error,
    versionGroups,
    selectedVersion,
    selectedChat,
    selectedProposals,
    sortedTasks,
    taskByVersion,
    rows,
    stats,
    init,
    refresh,
    loadConversation,
    selectVersion,
    reply,
    submitTask,
    advanceTask,
  }
})

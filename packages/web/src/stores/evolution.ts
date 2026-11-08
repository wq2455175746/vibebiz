import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import * as evolutionApi from '@/api/evolution'
import type {
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionVersion,
  EvolutionVersionGroup,
} from '@/types/evolution'
import { OFFICIAL_USER_ID, groupVersionsByUser } from '@/utils/evolution'

interface EvolutionViewer {
  id: string
  name: string
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : '操作失败'
}

export const useEvolutionStore = defineStore('evolution', () => {
  const versions = ref<EvolutionVersion[]>([])
  const originVersion = ref('')
  const projectId = ref('vibebiz-evolve')
  const messagesByUser = ref<Record<string, EvolutionChatMessage[]>>({})
  const proposalsByUser = ref<Record<string, EvolutionProposal[]>>({})
  const selectedVersionId = ref<string | null>(null)
  const selectedUserId = ref<string | null>(null)
  const viewer = ref<EvolutionViewer>({ id: '', name: '' })
  const loading = ref(false)
  const conversationLoading = ref(false)
  const error = ref('')

  const versionGroups = computed<EvolutionVersionGroup[]>(() =>
    groupVersionsByUser(versions.value, viewer.value.id),
  )
  const selectedVersion = computed<EvolutionVersion | null>(() => {
    const id = selectedVersionId.value
    if (id) {
      const found = versions.value.find((version) => version.id === id)
      if (found) {
        return found
      }
    }
    return versions.value.find((version) => version.userId === viewer.value.id) ?? null
  })
  const activeUserId = computed(
    () => selectedVersion.value?.userId ?? selectedUserId.value ?? viewer.value.id,
  )
  const activeUserName = computed(
    () =>
      selectedVersion.value?.userName ??
      (activeUserId.value === viewer.value.id ? viewer.value.name : ''),
  )
  const activeChat = computed<EvolutionChatMessage[]>(
    () => messagesByUser.value[activeUserId.value] ?? [],
  )
  const activeProposals = computed<EvolutionProposal[]>(
    () => proposalsByUser.value[activeUserId.value] ?? [],
  )
  const pendingProposalCount = computed(
    () =>
      activeProposals.value.filter(
        (proposal) => proposal.status === 'queued' || proposal.status === 'reviewing',
      ).length,
  )
  const isOwnVersion = computed(() => activeUserId.value === viewer.value.id)
  const latestPreviewVersion = computed<EvolutionVersion | null>(() => {
    if (!isOwnVersion.value || !activeUserId.value) {
      return null
    }
    return (
      versions.value
        .filter(
          (version) =>
            version.userId === activeUserId.value &&
            version.status === 'preview' &&
            version.userId !== OFFICIAL_USER_ID,
        )
        .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0] ?? null
    )
  })

  async function refresh(): Promise<void> {
    loading.value = true
    error.value = ''
    try {
      const result = await evolutionApi.listVersions()
      versions.value = result.versions ?? []
      originVersion.value = result.originVersion || ''
      projectId.value = result.projectId || 'vibebiz-evolve'
      if (
        selectedVersionId.value &&
        !versions.value.some((version) => version.id === selectedVersionId.value)
      ) {
        selectedVersionId.value = null
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

  async function init(current: EvolutionViewer): Promise<void> {
    viewer.value = { id: current.id || '', name: current.name || current.id || '' }
    error.value = ''
    await refresh()
    let own = versions.value.find((version) => version.userId === viewer.value.id) ?? null
    if (!own && viewer.value.id) {
      try {
        const result = await evolutionApi.ensureDraft()
        await refresh()
        own = versions.value.find((version) => version.id === result.version.id) ?? result.version
      } catch (err) {
        error.value = errorMessage(err)
      }
    }
    const preferred = own ?? versions.value[0]
    if (preferred) {
      selectedVersionId.value = preferred.id
      selectedUserId.value = preferred.userId
      await loadConversation(preferred.userId)
      return
    }
    selectedUserId.value = viewer.value.id
    await loadConversation(viewer.value.id)
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

  async function submitProposal(content: string): Promise<EvolutionProposal> {
    const result = await evolutionApi.submitProposal(content)
    await refresh()
    if (result.version) {
      selectedVersionId.value = result.version.id
      selectedUserId.value = result.version.userId
      await loadConversation(result.version.userId, true)
    } else {
      const own = versions.value.find((version) => version.userId === viewer.value.id)
      if (own) {
        selectedVersionId.value = own.id
        selectedUserId.value = own.userId
      }
      await loadConversation(viewer.value.id, true)
    }
    return result.proposal
  }

  async function submitChat(content: string): Promise<void> {
    await evolutionApi.submitChat(content)
    await loadConversation(viewer.value.id, true)
  }

  async function confirmVersion(versionId: string): Promise<EvolutionVersion> {
    const result = await evolutionApi.confirmVersion(versionId)
    await refresh()
    const version = versions.value.find((item) => item.id === result.version.id) ?? result.version
    selectedVersionId.value = version.id
    selectedUserId.value = version.userId
    await loadConversation(version.userId, true)
    return version
  }

  return {
    versions,
    originVersion,
    projectId,
    messagesByUser,
    proposalsByUser,
    selectedVersionId,
    selectedUserId,
    viewer,
    loading,
    conversationLoading,
    error,
    versionGroups,
    selectedVersion,
    activeUserId,
    activeUserName,
    activeChat,
    activeProposals,
    pendingProposalCount,
    isOwnVersion,
    latestPreviewVersion,
    init,
    refresh,
    loadConversation,
    selectVersion,
    submitProposal,
    submitChat,
    confirmVersion,
  }
})

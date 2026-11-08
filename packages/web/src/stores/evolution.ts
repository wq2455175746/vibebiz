import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionState,
  EvolutionVersion,
  EvolutionVersionGroup,
  EvolutionViewer,
} from '@/types/evolution'
import { OFFICIAL_VERSION_ID, groupVersionsByUser, latestVersionsByUser } from '@/utils/evolution'
import {
  browserStorage,
  confirmVersionState,
  ensureViewer,
  loadEvolutionState,
  saveEvolutionState,
  submitChatState,
  submitProposalState,
  type EvolutionStorage,
} from '@/utils/evolutionState'

const emptyState: EvolutionState = {
  proposals: [],
  versions: [],
  chats: {},
  selectedVersionId: null,
  selectedUserId: null,
}

export const useEvolutionStore = defineStore('evolution', () => {
  const state = ref<EvolutionState>(emptyState)
  const viewer = ref<EvolutionViewer>({ id: 'guest', name: '访客' })
  const initialized = ref(false)
  const storage = ref<EvolutionStorage | null>(browserStorage())

  const versions = computed<EvolutionVersion[]>(() => state.value.versions)
  const proposals = computed<EvolutionProposal[]>(() => state.value.proposals)
  const chats = computed<Record<string, EvolutionChatMessage[]>>(() => state.value.chats)
  const selectedVersionId = computed(() => state.value.selectedVersionId)
  const selectedUserId = computed(() => state.value.selectedUserId)

  const versionGroups = computed<EvolutionVersionGroup[]>(() =>
    groupVersionsByUser(state.value.versions, viewer.value.id),
  )
  const latestVersions = computed<EvolutionVersion[]>(() =>
    latestVersionsByUser(state.value.versions, viewer.value.id),
  )
  const selectedVersion = computed<EvolutionVersion | null>(() => {
    const id = state.value.selectedVersionId
    if (id) {
      const found = state.value.versions.find((version) => version.id === id)
      if (found) {
        return found
      }
    }
    return latestVersions.value[0] ?? null
  })
  const activeUserId = computed(() => selectedVersion.value?.userId ?? state.value.selectedUserId)
  const activeUserName = computed(
    () =>
      selectedVersion.value?.userName ??
      (activeUserId.value === viewer.value.id ? viewer.value.name : ''),
  )
  const activeChat = computed<EvolutionChatMessage[]>(() => {
    const id = activeUserId.value
    return id ? (state.value.chats[id] ?? []) : []
  })
  const activeProposals = computed<EvolutionProposal[]>(() => {
    const id = activeUserId.value
    return id ? state.value.proposals.filter((proposal) => proposal.userId === id) : []
  })
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
      state.value.versions
        .filter(
          (version) =>
            version.userId === activeUserId.value &&
            version.status === 'preview' &&
            version.id !== OFFICIAL_VERSION_ID,
        )
        .sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt))[0] ?? null
    )
  })

  function persist(): void {
    saveEvolutionState(state.value, storage.value)
  }

  function init(current: EvolutionViewer, nextStorage?: EvolutionStorage | null): void {
    viewer.value = { id: current.id || 'guest', name: current.name || '访客' }
    storage.value = nextStorage === undefined ? browserStorage() : nextStorage
    state.value = loadEvolutionState(viewer.value, Date.now(), storage.value)
    initialized.value = true
    persist()
  }

  function refreshViewer(current: EvolutionViewer): void {
    viewer.value = { id: current.id || 'guest', name: current.name || '访客' }
    state.value = ensureViewer(state.value, viewer.value)
    persist()
  }

  function selectVersion(versionId: string): void {
    const version = state.value.versions.find((item) => item.id === versionId)
    if (!version) {
      return
    }
    state.value = {
      ...state.value,
      selectedVersionId: version.id,
      selectedUserId: version.userId,
    }
    persist()
  }

  function submitProposal(content: string): EvolutionProposal | null {
    const before = state.value.proposals.length
    state.value = submitProposalState(state.value, viewer.value, content)
    persist()
    if (state.value.proposals.length === before) {
      return null
    }
    return state.value.proposals[0] ?? null
  }

  function submitChat(content: string): boolean {
    const before = state.value.chats[viewer.value.id]?.length ?? 0
    state.value = submitChatState(state.value, viewer.value, content)
    persist()
    return (state.value.chats[viewer.value.id]?.length ?? 0) > before
  }

  function confirmVersion(versionId: string): boolean {
    const version = state.value.versions.find((item) => item.id === versionId)
    if (!version) {
      return false
    }
    state.value = confirmVersionState(state.value, versionId)
    persist()
    return true
  }

  return {
    state,
    viewer,
    initialized,
    versions,
    proposals,
    chats,
    selectedVersionId,
    selectedUserId,
    versionGroups,
    latestVersions,
    selectedVersion,
    activeUserId,
    activeUserName,
    activeChat,
    activeProposals,
    pendingProposalCount,
    isOwnVersion,
    latestPreviewVersion,
    init,
    refreshViewer,
    selectVersion,
    submitProposal,
    submitChat,
    confirmVersion,
  }
})

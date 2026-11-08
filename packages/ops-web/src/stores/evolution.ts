import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  EvolutionAdminState,
  EvolutionChatMessage,
  EvolutionProposal,
  EvolutionTask,
  EvolutionVersion,
  EvolutionVersionGroup,
} from '@/types/evolution'
import { OFFICIAL_USER_ID, OFFICIAL_VERSION_LABEL, groupVersionsByUser } from '@/utils/evolution'
import {
  advanceTaskState,
  browserStorage,
  loadEvolutionState,
  saveEvolutionState,
  sendAdminReplyState,
  submitTaskState,
  type EvolutionStorage,
} from '@/utils/evolutionState'

const emptyState: EvolutionAdminState = {
  versions: [],
  proposals: [],
  chats: {},
  tasks: [],
  selectedVersionId: null,
  selectedUserId: null,
}

export const useEvolutionAdminStore = defineStore('evolution-admin', () => {
  const state = ref<EvolutionAdminState>(emptyState)
  const initialized = ref(false)
  const storage = ref<EvolutionStorage | null>(browserStorage())

  const versions = computed<EvolutionVersion[]>(() => state.value.versions)
  const proposals = computed<EvolutionProposal[]>(() => state.value.proposals)
  const versionGroups = computed<EvolutionVersionGroup[]>(() =>
    groupVersionsByUser(state.value.versions, ''),
  )
  const selectedVersion = computed<EvolutionVersion | null>(
    () =>
      state.value.versions.find((version) => version.id === state.value.selectedVersionId) ?? null,
  )
  const activeUserId = computed(
    () => selectedVersion.value?.userId ?? state.value.selectedUserId ?? null,
  )
  const activeChat = computed<EvolutionChatMessage[]>(() =>
    activeUserId.value ? (state.value.chats[activeUserId.value] ?? []) : [],
  )
  const activeProposals = computed<EvolutionProposal[]>(() =>
    activeUserId.value
      ? state.value.proposals.filter((proposal) => proposal.userId === activeUserId.value)
      : [],
  )
  const tasks = computed<EvolutionTask[]>(() =>
    [...state.value.tasks].sort((a, b) => Date.parse(b.createdAt) - Date.parse(a.createdAt)),
  )
  const selectedTask = computed<EvolutionTask | null>(
    () => tasks.value.find((task) => task.versionId === selectedVersion.value?.id) ?? null,
  )
  const canSubmit = computed(
    () =>
      Boolean(selectedVersion.value) &&
      selectedVersion.value?.userId !== OFFICIAL_USER_ID &&
      !selectedTask.value,
  )

  function persist(): void {
    saveEvolutionState(state.value, storage.value)
  }

  function init(nextStorage?: EvolutionStorage | null): void {
    storage.value = nextStorage === undefined ? browserStorage() : nextStorage
    state.value = loadEvolutionState(Date.now(), storage.value)
    initialized.value = true
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

  function sendReply(content: string): boolean {
    if (!selectedVersion.value || selectedVersion.value.userId === OFFICIAL_USER_ID) {
      return false
    }
    const before = activeChat.value.length
    state.value = sendAdminReplyState(state.value, content)
    persist()
    return activeChat.value.length > before
  }

  function submitTask(): boolean {
    if (!canSubmit.value) {
      return false
    }
    const before = state.value.tasks.length
    state.value = submitTaskState(state.value, OFFICIAL_VERSION_LABEL)
    persist()
    return state.value.tasks.length > before
  }

  function advanceTask(taskId: string): void {
    state.value = advanceTaskState(state.value, taskId)
    persist()
  }

  return {
    state,
    initialized,
    versions,
    proposals,
    versionGroups,
    selectedVersion,
    activeUserId,
    activeChat,
    activeProposals,
    tasks,
    selectedTask,
    canSubmit,
    init,
    selectVersion,
    sendReply,
    submitTask,
    advanceTask,
  }
})

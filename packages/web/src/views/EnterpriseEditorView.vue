<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { readFile } from '@/api/files'
import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePane from '@/components/editor/CodePane.vue'
import EditorHeader from '@/components/editor/EditorHeader.vue'
import LogConsole from '@/components/editor/LogConsole.vue'
import PreviewPane from '@/components/editor/PreviewPane.vue'
import DocumentWorkspace from '@/components/enterprise/DocumentWorkspace.vue'
import EnterpriseStageBar from '@/components/enterprise/EnterpriseStageBar.vue'
import PlaceholderWorkspace from '@/components/enterprise/PlaceholderWorkspace.vue'
import StageWorkspaceHeader from '@/components/enterprise/StageWorkspaceHeader.vue'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { useLogStore } from '@/stores/logs'
import { useProjectStore } from '@/stores/project'
import type { AgentRole, ChatReference, FileContent } from '@/types'
import {
  buildStagePrompt,
  completeStage,
  createInitialStageState,
  currentStageKey,
  loadEnterpriseStageState,
  markEnterpriseProject,
  saveEnterpriseStageState,
  stageDefinition,
  type EnterpriseStageKey,
  type EnterpriseStageStateMap,
} from '@/utils/enterprise'
import { shouldSendInitialPrompt, takeInitialAppend } from '@/utils/initialPrompt'
import '@/styles/editor.css'
import '@/styles/enterprise.css'

const route = useRoute()
const router = useRouter()
const projectStore = useProjectStore()
const agentRoles = useAgentRolesStore()
const files = useFileStore()
const chat = useChatStore()
const logs = useLogStore()

const id = computed(() => String(route.params.id))
const activeTab = ref<'preview' | 'code' | 'console'>('preview')
const previewReady = ref(false)
const stageStates = ref<EnterpriseStageStateMap>(createInitialStageState())
const viewingKey = ref<EnterpriseStageKey | null>(null)
const artifact = ref<FileContent | null>(null)
const artifactLoading = ref(false)
const artifactError = ref('')
const confirming = ref(false)
const confirmError = ref('')
const previewRevision = computed(() => chat.revision + files.revision)
const activeKey = computed(() => currentStageKey(stageStates.value))
const displayedKey = computed<EnterpriseStageKey>(
  () => viewingKey.value ?? activeKey.value ?? 'artifacts',
)
const displayedStage = computed(() => stageDefinition(displayedKey.value))
const activeStage = computed(() => (activeKey.value ? stageDefinition(activeKey.value) : null))
const isHistory = computed(() =>
  Boolean(activeKey.value && viewingKey.value && viewingKey.value !== activeKey.value),
)
const canComplete = computed(
  () =>
    Boolean(activeStage.value?.artifactPath) &&
    Boolean(artifact.value?.content.trim()) &&
    !chat.busy &&
    !confirming.value,
)
const headerRoles = computed(() =>
  (projectStore.current?.agentRoleIds ?? [])
    .map((roleId) => agentRoles.byId(roleId))
    .filter((role): role is AgentRole => Boolean(role)),
)
const tabMeta = computed(() => {
  if (activeTab.value === 'preview') {
    return '实时预览'
  }
  return activeTab.value === 'code' ? '只读文件' : '终端输出'
})
let statusTimer: ReturnType<typeof setInterval> | null = null
let backgroundProjectId: string | null = null

function stopStatusPolling() {
  if (statusTimer !== null) {
    clearInterval(statusTimer)
    statusTimer = null
  }
}

function stopBackgroundPolling() {
  backgroundProjectId = null
  stopStatusPolling()
}

async function refreshProjectLogs(projectId: string) {
  try {
    await logs.load(projectId)
  } catch {
    return
  }
}

async function pollBackground(projectId: string) {
  if (backgroundProjectId !== projectId) {
    return
  }
  await refreshProjectLogs(projectId)
  if (backgroundProjectId !== projectId) {
    return
  }
  let active = false
  try {
    active = await chat.resume(projectId)
  } catch {
    return
  }
  if (backgroundProjectId !== projectId || active) {
    return
  }
  backgroundProjectId = null
  stopStatusPolling()
  await refreshProjectLogs(projectId)
}

function startBackgroundPolling(projectId: string) {
  backgroundProjectId = projectId
  stopStatusPolling()
  statusTimer = setInterval(() => void pollBackground(projectId), 2500)
}

function promptKey(projectId: string) {
  return `vibebiz:initial-prompt:${projectId}`
}

function takeInitialReferences(projectId: string): ChatReference[] {
  const key = `vibebiz:initial-references:${projectId}`
  const raw = sessionStorage.getItem(key)
  sessionStorage.removeItem(key)
  if (!raw) {
    return []
  }
  try {
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? (parsed as ChatReference[]) : []
  } catch {
    return []
  }
}

function takeInitialPrompt(projectId: string) {
  const key = promptKey(projectId)
  const stored = sessionStorage.getItem(key)
  sessionStorage.removeItem(key)
  const state = window.history.state as { initialPrompt?: unknown } | null
  const fromHistory = typeof state?.initialPrompt === 'string' ? state.initialPrompt : ''
  if (state?.initialPrompt) {
    const nextState = { ...state }
    delete nextState.initialPrompt
    window.history.replaceState(nextState, '')
  }
  return fromHistory.trim() || stored?.trim() || ''
}

async function loadArtifact() {
  const stage = displayedStage.value
  artifactError.value = ''
  artifact.value = null
  if (!stage.artifactPath) {
    return
  }
  artifactLoading.value = true
  try {
    artifact.value = await readFile(id.value, stage.artifactPath)
  } catch (err) {
    if (!(err instanceof ApiError && err.status === 404)) {
      artifactError.value = (err as Error).message
    }
  } finally {
    artifactLoading.value = false
  }
}

async function load() {
  const projectId = id.value
  activeTab.value = 'preview'
  previewReady.value = false
  viewingKey.value = null
  stopBackgroundPolling()
  logs.clear()
  chat.reset()
  markEnterpriseProject(projectId)
  stageStates.value = loadEnterpriseStageState(projectId)
  saveEnterpriseStageState(projectId, stageStates.value)
  void agentRoles.load()
  await projectStore.load(projectId)

  files.entries = []
  files.active = null
  files.activePath = null
  files.revision = 0
  try {
    await files.loadTree(projectId)
  } catch {
    files.entries = []
  }

  let hasHistory = false
  try {
    hasHistory = await chat.loadHistory(projectId)
  } catch {
    hasHistory = false
  }
  const active = await chat.syncStatus(projectId).catch(() => false)
  await refreshProjectLogs(projectId)
  if (active) {
    startBackgroundPolling(projectId)
  }
  void loadArtifact()
  previewReady.value = true

  const prompt = takeInitialPrompt(projectId)
  const references = takeInitialReferences(projectId)
  const append = takeInitialAppend(projectId)
  if (
    activeStage.value?.key === 'requirements' &&
    shouldSendInitialPrompt(prompt, hasHistory, append)
  ) {
    if (chat.busy) {
      logs.append({ level: 'error', source: 'agent', message: '目标项目正在执行，消息未发送' })
    } else {
      void chat.send(
        projectId,
        buildStagePrompt(stageDefinition('requirements'), prompt),
        references,
      )
    }
  }
}

async function completeCurrentStage() {
  const key = activeKey.value
  const stage = activeStage.value
  if (!key || !stage || chat.busy || confirming.value) {
    return
  }
  confirming.value = true
  confirmError.value = ''
  try {
    const file = await readFile(id.value, stage.artifactPath)
    if (!file.content.trim()) {
      throw new Error(`请等待 ${stage.roleLabel} 完成 ${stage.artifactPath}`)
    }
    const result = completeStage(stageStates.value, key)
    if (!result) {
      throw new Error('当前阶段无法推进')
    }
    stageStates.value = result.states
    saveEnterpriseStageState(id.value, stageStates.value)
    viewingKey.value = null
    activeTab.value = 'preview'
    if (result.nextKey) {
      const next = stageDefinition(result.nextKey)
      void chat.send(id.value, buildStagePrompt(next))
    }
    await loadArtifact()
  } catch (err) {
    confirmError.value = (err as Error).message
  } finally {
    confirming.value = false
  }
}

function selectStage(key: EnterpriseStageKey) {
  if (stageStates.value[key].status !== 'done' && stageStates.value[key].status !== 'active') {
    return
  }
  viewingKey.value = key === activeKey.value ? null : key
}

function backToCurrent() {
  viewingKey.value = null
  activeTab.value = 'preview'
}

function goEnterpriseHome() {
  void router.push({ name: 'enterprise' })
}

watch(id, load)
watch(
  () => chat.revision,
  async () => {
    try {
      await files.loadTree(id.value)
      if (files.active && !files.dirty) {
        await files.reload(id.value)
      }
    } catch {
      files.entries = []
    }
  },
)
watch(
  () => [displayedKey.value, chat.revision, files.revision] as const,
  () => void loadArtifact(),
)

onMounted(load)
onBeforeUnmount(() => {
  stopBackgroundPolling()
  chat.disconnect()
})
</script>

<template>
  <div class="editor-shell enterprise-editor-shell">
    <EditorHeader
      :project-name="projectStore.current?.name || '加载中'"
      :project-state="projectStore.current?.state || 'starting'"
      :roles="headerRoles"
      model-label="DeepSeek Flash"
      @back="goEnterpriseHome"
    />

    <p v-if="projectStore.error" class="editor-error">{{ projectStore.error }}</p>

    <EnterpriseStageBar
      :states="stageStates"
      :active-key="activeKey"
      :viewing-key="viewingKey"
      @select="selectStage"
    />

    <div class="editor-body">
      <ChatPanel :project-id="id" :readonly="isHistory" required-role-code="team" />

      <main class="editor-workbench enterprise-workbench">
        <template v-if="displayedStage.workspace !== 'placeholder'">
          <StageWorkspaceHeader
            :stage="displayedStage"
            :read-only="isHistory"
            :busy="chat.busy"
            :can-complete="canComplete"
            :confirming="confirming"
            :error="confirmError"
            @complete="completeCurrentStage"
            @back="backToCurrent"
          />

          <DocumentWorkspace
            v-if="displayedStage.workspace === 'document'"
            :content="artifact?.content || ''"
            :path="displayedStage.artifactPath"
            :role-label="displayedStage.roleLabel"
            :loading="artifactLoading"
            :error="artifactError"
          />

          <template v-else>
            <header class="workbench-head">
              <div class="workbench-tabs">
                <button
                  type="button"
                  :class="{ active: activeTab === 'preview' }"
                  @click="activeTab = 'preview'"
                >
                  <i class="pi pi-desktop" />
                  应用查看器
                </button>
                <button
                  type="button"
                  :class="{ active: activeTab === 'code' }"
                  @click="activeTab = 'code'"
                >
                  <i class="pi pi-code" />
                  代码
                </button>
                <button
                  type="button"
                  :class="{ active: activeTab === 'console' }"
                  @click="activeTab = 'console'"
                >
                  <i class="pi pi-server" />
                  终端
                </button>
              </div>
              <span class="workbench-meta">{{ tabMeta }}</span>
            </header>

            <div class="workbench-body enterprise-development-body">
              <PreviewPane
                v-if="previewReady"
                v-show="activeTab === 'preview'"
                :project-id="id"
                :refresh-token="previewRevision"
                :active="activeTab === 'preview'"
              />
              <CodePane v-if="activeTab === 'code'" :project-id="id" :locked="isHistory" />
              <LogConsole
                v-else-if="activeTab === 'console'"
                class="workbench-log"
                :open="true"
                @close="activeTab = 'preview'"
              />
            </div>
          </template>
        </template>

        <PlaceholderWorkspace v-else :stage="displayedStage" />
      </main>
    </div>
  </div>
</template>

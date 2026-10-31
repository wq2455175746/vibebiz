<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { ApiError } from '@/api/client'
import { readFile } from '@/api/files'
import { completeProjectStage, listProjectStages, startProjectStage } from '@/api/projects'
import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePane from '@/components/editor/CodePane.vue'
import EditorHeader from '@/components/editor/EditorHeader.vue'
import PublishDialog from '@/components/editor/PublishDialog.vue'
import LogConsole from '@/components/editor/LogConsole.vue'
import PreviewPane from '@/components/editor/PreviewPane.vue'
import ArtifactWorkspace from '@/components/enterprise/ArtifactWorkspace.vue'
import DeliveryWorkspace from '@/components/enterprise/DeliveryWorkspace.vue'
import DocumentWorkspace from '@/components/enterprise/DocumentWorkspace.vue'
import EnterpriseStageBar from '@/components/enterprise/EnterpriseStageBar.vue'
import PlaceholderWorkspace from '@/components/enterprise/PlaceholderWorkspace.vue'
import StageWorkspaceHeader from '@/components/enterprise/StageWorkspaceHeader.vue'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useChatStore } from '@/stores/chat'
import { useEnterpriseFlowStore } from '@/stores/enterpriseFlow'
import { useFileStore } from '@/stores/files'
import { useLogStore } from '@/stores/logs'
import { useProjectStore } from '@/stores/project'
import type { AgentRole, ChatReference, FileContent, StageMutation } from '@/types'
import {
  buildStagePrompt,
  createInitialStageState,
  currentStageKey,
  markEnterpriseProject,
  stageArtifactReady,
  stageDefinition,
  stageStatesFromProject,
  type EnterpriseStageKey,
  type EnterpriseStageStateMap,
} from '@/utils/enterprise'
import { mergeEnterpriseStageStates } from '@/utils/enterpriseFlow'
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
const flows = useEnterpriseFlowStore()

const id = computed(() => String(route.params.id))
const activeTab = ref<'preview' | 'code' | 'console'>('preview')
const previewReady = ref(false)
const stageStates = ref<EnterpriseStageStateMap>(createInitialStageState())
const serverStageStates = ref<EnterpriseStageStateMap>(createInitialStageState())
const viewingKey = ref<EnterpriseStageKey | null>(null)
const artifact = ref<FileContent | null>(null)
const loadedArtifactPath = ref('')
const artifactLoading = ref(false)
const artifactError = ref('')
const confirming = ref(false)
const starting = ref(false)
const confirmError = ref('')
const stageLoadError = ref('')
const publishOpen = ref(false)
const previewRevision = computed(() => chat.revision + files.revision)
const activeKey = computed(() => currentStageKey(stageStates.value))
const activeState = computed(() => (activeKey.value ? stageStates.value[activeKey.value] : null))
const activeStarted = computed(() => Boolean(activeState.value?.startedAt))
const displayedKey = computed<EnterpriseStageKey>(
  () =>
    viewingKey.value ??
    activeKey.value ??
    (stageStates.value.delivery.status === 'done' ? 'delivery' : 'artifacts'),
)
const displayedStage = computed(() => stageDefinition(displayedKey.value))
const activeStage = computed(() => (activeKey.value ? stageDefinition(activeKey.value) : null))
const currentFlow = computed(() => flows.get(id.value))
const isHistory = computed(() =>
  Boolean(activeKey.value && viewingKey.value && viewingKey.value !== activeKey.value),
)
const chatReadOnly = computed(
  () =>
    isHistory.value ||
    displayedStage.value.workspace === 'artifact' ||
    displayedStage.value.workspace === 'delivery',
)
const canComplete = computed(
  () =>
    stageArtifactReady(
      activeStage.value,
      loadedArtifactPath.value,
      artifact.value?.content ?? '',
    ) &&
    activeStarted.value &&
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
  loadedArtifactPath.value = ''
  if (!stage.artifactPath) {
    return
  }
  artifactLoading.value = true
  try {
    artifact.value = await readFile(id.value, stage.artifactPath)
    loadedArtifactPath.value = stage.artifactPath
  } catch (err) {
    if (!(err instanceof ApiError && err.status === 404)) {
      artifactError.value = (err as Error).message
    }
  } finally {
    artifactLoading.value = false
  }
}

function syncStageOverlay(): void {
  const flow = currentFlow.value
  stageStates.value = flow
    ? mergeEnterpriseStageStates(serverStageStates.value, flow)
    : serverStageStates.value
}

function ensureArtifactFlow(): void {
  if (stageStates.value.development.status === 'done') {
    flows.ensureArtifact(id.value)
    syncStageOverlay()
  }
}

async function loadStages(projectId: string) {
  try {
    const result = await listProjectStages(projectId)
    serverStageStates.value = stageStatesFromProject(result.stages)
    stageStates.value = serverStageStates.value
    ensureArtifactFlow()
    stageLoadError.value = ''
  } catch (err) {
    stageLoadError.value = (err as Error).message || '阶段状态加载失败'
    serverStageStates.value = createInitialStageState()
    stageStates.value = serverStageStates.value
  }
}

function applyStageMutation(result: StageMutation) {
  const next = { ...serverStageStates.value }
  for (const stage of [result.stage, result.nextStage]) {
    if (!stage || !(stage.stageKey in next)) {
      continue
    }
    const key = stage.stageKey as EnterpriseStageKey
    next[key] = {
      status: stage.status,
      startedAt: stage.startedAt ?? null,
      completedAt: stage.completedAt ?? null,
    }
  }
  serverStageStates.value = next
  stageStates.value = next
}

async function startStage(
  key: EnterpriseStageKey,
  requirement = '',
  references: ChatReference[] = [],
): Promise<void> {
  if (starting.value || chat.busy) {
    return
  }
  starting.value = true
  confirmError.value = ''
  try {
    const result = await startProjectStage(id.value, key)
    applyStageMutation(result)
    const definition = stageDefinition(key)
    const request =
      requirement || (key === 'requirements' ? (projectStore.current?.prompt ?? '') : '')
    void chat.send(id.value, buildStagePrompt(definition, request), references)
  } catch (err) {
    confirmError.value = (err as Error).message
  } finally {
    starting.value = false
  }
}

function startCurrentStage() {
  if (
    !activeStage.value ||
    activeStage.value.workspace === 'artifact' ||
    activeStage.value.workspace === 'delivery'
  ) {
    return
  }
  void startStage(activeStage.value.key, projectStore.current?.prompt ?? '')
}

function confirmArtifactStage(): void {
  if (!flows.confirmArtifact(id.value)) {
    confirmError.value = '请先完成代码上传、Jenkins 构建和 Harbor 推送'
    return
  }
  viewingKey.value = null
  confirmError.value = ''
  syncStageOverlay()
}

function confirmDeliveryStage(): void {
  if (!flows.confirmDelivery(id.value)) {
    confirmError.value = '请先完成所选交付方式'
    return
  }
  viewingKey.value = null
  confirmError.value = ''
  syncStageOverlay()
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
  stageLoadError.value = ''
  void agentRoles.load()
  await projectStore.load(projectId)
  await loadStages(projectId)

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
    !stageStates.value.requirements.startedAt &&
    shouldSendInitialPrompt(prompt, hasHistory, append)
  ) {
    if (chat.busy) {
      logs.append({ level: 'error', source: 'agent', message: '目标项目正在执行，消息未发送' })
    } else {
      await startStage('requirements', prompt, references)
    }
  }
}

async function completeCurrentStage() {
  const key = activeKey.value
  const stage = activeStage.value
  if (!key || !stage || chat.busy || confirming.value) {
    return
  }
  if (!canComplete.value) {
    confirmError.value = `等待 ${stage.roleLabel} 写入 ${stage.artifactPath}`
    return
  }
  confirming.value = true
  confirmError.value = ''
  try {
    const result = await completeProjectStage(id.value, key)
    applyStageMutation(result)
    if (key === 'development') {
      ensureArtifactFlow()
    }
    viewingKey.value = null
    activeTab.value = 'preview'
    if (result.nextStage) {
      await startStage(result.nextStage.stageKey as EnterpriseStageKey)
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
      @publish="publishOpen = true"
    />

    <p v-if="projectStore.error" class="editor-error">{{ projectStore.error }}</p>
    <p v-if="stageLoadError" class="editor-error">{{ stageLoadError }}</p>
    <PublishDialog
      v-if="publishOpen"
      :project-id="id"
      :project-name="projectStore.current?.name || ''"
      @close="publishOpen = false"
    />

    <EnterpriseStageBar
      :states="stageStates"
      :active-key="activeKey"
      :viewing-key="viewingKey"
      @select="selectStage"
    />

    <div class="editor-body">
      <ChatPanel :project-id="id" :readonly="chatReadOnly" required-role-code="team" />

      <main class="editor-workbench enterprise-workbench">
        <ArtifactWorkspace
          v-if="displayedStage.workspace === 'artifact' && currentFlow"
          :project-id="id"
          :flow="currentFlow.artifact"
          @confirmed="confirmArtifactStage"
        />
        <DeliveryWorkspace
          v-else-if="displayedStage.workspace === 'delivery' && currentFlow"
          :project-id="id"
          :flow="currentFlow.delivery"
          @confirmed="confirmDeliveryStage"
        />
        <template v-else-if="displayedStage.workspace !== 'placeholder'">
          <StageWorkspaceHeader
            :stage="displayedStage"
            :read-only="isHistory"
            :busy="chat.busy"
            :started="activeStarted"
            :can-complete="canComplete"
            :confirming="confirming"
            :starting="starting"
            :error="confirmError"
            @start="startCurrentStage"
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

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePane from '@/components/editor/CodePane.vue'
import EditorHeader from '@/components/editor/EditorHeader.vue'
import LogConsole from '@/components/editor/LogConsole.vue'
import PreviewPane from '@/components/editor/PreviewPane.vue'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { useLogStore } from '@/stores/logs'
import { useProjectStore } from '@/stores/project'
import type { AgentRole } from '@/types'
import { shouldSendInitialPrompt, takeInitialAppend } from '@/utils/initialPrompt'
import '@/styles/editor.css'

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
const previewRevision = computed(() => chat.revision + files.revision)
const headerRoles = computed(() =>
  (projectStore.current?.agentRoleIds ?? [])
    .map((id) => agentRoles.byId(id))
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

function takeInitialReferences(projectId: string) {
  const key = `vibebiz:initial-references:${projectId}`
  const raw = sessionStorage.getItem(key)
  sessionStorage.removeItem(key)
  if (!raw) {
    return []
  }
  try {
    const parsed = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed : []
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

async function load() {
  const projectId = id.value
  activeTab.value = 'preview'
  previewReady.value = false
  stopBackgroundPolling()
  logs.clear()
  chat.reset()
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
  previewReady.value = true

  const prompt = takeInitialPrompt(projectId)
  const references = takeInitialReferences(projectId)
  const append = takeInitialAppend(projectId)
  if (shouldSendInitialPrompt(prompt, hasHistory, append)) {
    if (chat.busy) {
      logs.append({ level: 'error', source: 'agent', message: '目标项目正在执行，消息未发送' })
    } else {
      void chat.send(projectId, prompt, references)
    }
  }
}

function goHome() {
  void router.push({ name: 'home' })
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

onMounted(load)
onBeforeUnmount(() => {
  stopBackgroundPolling()
  chat.disconnect()
})
</script>

<template>
  <div class="editor-shell">
    <EditorHeader
      :project-name="projectStore.current?.name || '加载中'"
      :project-state="projectStore.current?.state || 'starting'"
      :roles="headerRoles"
      model-label="DeepSeek Flash"
      @back="goHome"
    />

    <p v-if="projectStore.error" class="editor-error">{{ projectStore.error }}</p>

    <div class="editor-body">
      <ChatPanel :project-id="id" />

      <main class="editor-workbench">
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

        <div class="workbench-body">
          <PreviewPane
            v-if="previewReady"
            v-show="activeTab === 'preview'"
            :project-id="id"
            :refresh-token="previewRevision"
            :active="activeTab === 'preview'"
          />
          <CodePane v-if="activeTab === 'code'" :project-id="id" />
          <LogConsole
            v-else-if="activeTab === 'console'"
            class="workbench-log"
            :open="true"
            @close="activeTab = 'preview'"
          />
        </div>
      </main>
    </div>
  </div>
</template>

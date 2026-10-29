<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePane from '@/components/editor/CodePane.vue'
import EditorHeader from '@/components/editor/EditorHeader.vue'
import LogConsole from '@/components/editor/LogConsole.vue'
import PreviewPane from '@/components/editor/PreviewPane.vue'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { useLogStore } from '@/stores/logs'
import { useProjectStore } from '@/stores/project'
import '@/styles/editor.css'

const route = useRoute()
const router = useRouter()
const projectStore = useProjectStore()
const files = useFileStore()
const chat = useChatStore()
const logs = useLogStore()

const id = computed(() => String(route.params.id))
const activeTab = ref<'preview' | 'code'>('preview')
const consoleOpen = ref(false)

function promptKey(projectId: string) {
  return `vibebiz:initial-prompt:${projectId}`
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

function openConsole() {
  consoleOpen.value = true
}

async function load() {
  const projectId = id.value
  activeTab.value = 'preview'
  consoleOpen.value = false
  logs.clear()
  chat.reset()
  await projectStore.load(projectId)

  files.entries = []
  files.active = null
  files.activePath = null
  try {
    await files.loadTree(projectId)
  } catch {
    files.entries = []
  }

  const prompt = takeInitialPrompt(projectId)
  if (prompt) {
    openConsole()
    void chat.send(projectId, prompt)
  }
}

function goHome() {
  void router.push({ name: 'home' })
}

watch(id, load)
watch(
  () => chat.streaming,
  (streaming) => {
    if (streaming) {
      openConsole()
    }
  },
)
watch(
  () => chat.revision,
  async () => {
    try {
      await files.loadTree(id.value)
    } catch {
      files.entries = []
    }
  },
)

onMounted(load)
onBeforeUnmount(() => chat.stop())
</script>

<template>
  <div class="editor-shell">
    <EditorHeader
      :project-name="projectStore.current?.name || '加载中'"
      :project-state="projectStore.current?.state || 'starting'"
      :console-open="consoleOpen"
      model-label="DeepSeek Flash"
      @back="goHome"
      @toggle-console="consoleOpen = !consoleOpen"
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
          </div>
          <span class="workbench-meta">{{
            activeTab === 'preview' ? '实时预览' : '只读文件'
          }}</span>
        </header>

        <div class="workbench-body">
          <PreviewPane
            v-if="activeTab === 'preview'"
            :project-id="id"
            :refresh-token="chat.revision"
          />
          <CodePane v-else :project-id="id" />
        </div>

        <LogConsole :open="consoleOpen" @close="consoleOpen = false" />
      </main>
    </div>
  </div>
</template>

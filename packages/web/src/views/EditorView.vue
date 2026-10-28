<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import Button from 'primevue/button'
import { useRoute, useRouter } from 'vue-router'

import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePanel from '@/components/editor/CodePanel.vue'
import PreviewPanel from '@/components/editor/PreviewPanel.vue'
import { useFileStore } from '@/stores/files'
import { useProjectStore } from '@/stores/project'

const route = useRoute()
const router = useRouter()
const projectStore = useProjectStore()
const files = useFileStore()

const mode = ref<'preview' | 'code'>('preview')
const id = computed(() => String(route.params.id))

async function load() {
  await projectStore.load(id.value)
  try {
    await files.loadTree(id.value)
  } catch {
    files.entries = []
  }
}

onMounted(load)
watch(id, load)
</script>

<template>
  <div class="editor-shell">
    <header class="editor-top">
      <div class="editor-left">
        <Button icon="pi pi-angle-left" text rounded aria-label="返回" @click="router.push('/')" />
        <strong class="project-title">{{ projectStore.current?.name || '加载中' }}</strong>
        <span class="tag">M0 SCAFFOLD</span>
      </div>

      <div class="mode-switch">
        <button :class="{ active: mode === 'preview' }" type="button" @click="mode = 'preview'">
          预览
        </button>
        <button :class="{ active: mode === 'code' }" type="button" @click="mode = 'code'">
          代码
        </button>
      </div>

      <div class="editor-right">
        <Button label="分享" text />
        <Button label="发布" />
      </div>
    </header>

    <p v-if="projectStore.error" class="editor-error">{{ projectStore.error }}</p>

    <div class="editor-body">
      <ChatPanel class="chat-pane" />
      <section class="stage">
        <PreviewPanel v-if="mode === 'preview'" :project-id="id" />
        <CodePanel v-else :project-id="id" />
      </section>
    </div>
  </div>
</template>

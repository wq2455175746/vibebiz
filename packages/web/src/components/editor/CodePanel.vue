<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'

import { useFileStore } from '@/stores/files'
import { detectLanguage } from '@/utils/language'

const props = defineProps<{ projectId: string }>()
const files = useFileStore()

const fileEntries = computed(() => files.entries.filter((entry) => entry.type === 'file'))
const language = computed(() => (files.activePath ? detectLanguage(files.activePath) : 'plaintext'))
const content = computed(() => {
  if (!files.active) {
    return ''
  }
  return files.active.isBinary ? '二进制文件，暂不支持文本预览。' : files.active.content
})

function depth(path: string) {
  return path.split('/').length - 1
}

async function select(path: string) {
  await files.open(props.projectId, path)
}

onMounted(async () => {
  if (!files.activePath && fileEntries.value.length > 0) {
    await select('index.html')
  }
})

watch(
  () => props.projectId,
  async () => {
    await files.loadTree(props.projectId)
    if (fileEntries.value.length > 0) {
      await select('index.html')
    }
  },
)
</script>

<template>
  <div class="code-panel">
    <aside class="file-pane">
      <div class="panel-head">
        <span>文件</span>
        <span class="tag">只读</span>
      </div>
      <button
        v-for="entry in fileEntries"
        :key="entry.path"
        class="file-row"
        :class="{ active: entry.path === files.activePath }"
        :style="{ paddingLeft: `${12 + depth(entry.path) * 14}px` }"
        type="button"
        @click="select(entry.path)"
      >
        <i class="pi pi-file" />
        <span>{{ entry.path.split('/').pop() }}</span>
      </button>
      <p v-if="fileEntries.length === 0" class="muted small file-empty">暂无文件</p>
    </aside>

    <section class="code-main">
      <div class="code-head">
        <span>{{ files.activePath || '未选择文件' }}</span>
        <span class="tag">{{ language }}</span>
      </div>
      <div v-if="files.loading" class="empty-state">加载中...</div>
      <pre v-else class="code-preview">{{ content }}</pre>
    </section>
  </div>
</template>

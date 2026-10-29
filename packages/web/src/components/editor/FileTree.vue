<script setup lang="ts">
import type { FileEntry } from '@/types'

defineProps<{ entries: FileEntry[]; activePath: string | null }>()

const emit = defineEmits<{ (event: 'select', path: string): void }>()

function depth(path: string) {
  return path.split('/').length - 1
}

function name(path: string) {
  return path.split('/').pop() ?? path
}
</script>

<template>
  <aside class="file-tree">
    <div class="file-tree-head">
      <span>文件</span>
      <span class="tag">只读</span>
    </div>
    <div class="file-tree-scroll">
      <template v-for="entry in entries" :key="entry.path">
        <div
          v-if="entry.type === 'dir'"
          class="file-row file-dir"
          :style="{ paddingLeft: `${12 + depth(entry.path) * 14}px` }"
        >
          <i class="pi pi-folder" />
          <span>{{ name(entry.path) }}</span>
        </div>
        <button
          v-else
          class="file-row"
          :class="{ active: entry.path === activePath }"
          :style="{ paddingLeft: `${12 + depth(entry.path) * 14}px` }"
          type="button"
          @click="emit('select', entry.path)"
        >
          <i class="pi pi-file" />
          <span>{{ name(entry.path) }}</span>
        </button>
      </template>
      <p v-if="entries.length === 0" class="file-empty muted small">暂无文件</p>
    </div>
  </aside>
</template>

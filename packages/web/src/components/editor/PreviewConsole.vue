<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

import { usePreviewLogStore } from '@/stores/previewLogs'

const emit = defineEmits<{ (event: 'close'): void }>()

const logs = usePreviewLogStore()
const scroller = ref<HTMLElement | null>(null)
let stickToBottom = true

function onScroll() {
  const element = scroller.value
  if (!element) {
    return
  }
  stickToBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 60
}

async function scrollToBottom() {
  await nextTick()
  if (stickToBottom && scroller.value) {
    scroller.value.scrollTop = scroller.value.scrollHeight
  }
}

function formatTime(ts: string) {
  return new Date(ts).toLocaleTimeString('zh-CN', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  })
}

watch(
  () => logs.entries.length,
  () => void scrollToBottom(),
)
</script>

<template>
  <section class="log-console preview-console">
    <header class="log-head">
      <div class="log-title">
        <span class="log-dot" />
        <strong>预览控制台</strong>
        <span class="log-count">{{ logs.entries.length }}</span>
      </div>
      <div class="log-actions">
        <button class="log-action" type="button" @click="logs.clear()">
          <i class="pi pi-trash" />
          清空
        </button>
        <button
          class="log-action log-close"
          type="button"
          aria-label="关闭预览控制台"
          @click="emit('close')"
        >
          <i class="pi pi-times" />
        </button>
      </div>
    </header>

    <div ref="scroller" class="log-scroll" @scroll.passive="onScroll">
      <p v-if="logs.entries.length === 0" class="log-empty">等待预览页日志…</p>
      <div
        v-for="entry in logs.entries"
        :key="entry.id"
        class="log-line"
        :class="`level-${entry.level}`"
      >
        <span class="log-time">{{ formatTime(entry.ts) }}</span>
        <span class="log-source">{{ entry.source }}</span>
        <span class="log-message">{{ entry.message }}</span>
      </div>
    </div>
  </section>
</template>

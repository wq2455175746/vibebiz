<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

import { useLogStore } from '@/stores/logs'
import type { LogLevel } from '@/types'

defineProps<{ open: boolean }>()

const emit = defineEmits<{ (event: 'close'): void }>()

const logs = useLogStore()
const scroller = ref<HTMLElement | null>(null)
let stickToBottom = true

const levels: { value: 'all' | LogLevel; label: string }[] = [
  { value: 'all', label: '全部' },
  { value: 'info', label: '信息' },
  { value: 'warn', label: '警告' },
  { value: 'error', label: '错误' },
]

function onScroll() {
  const element = scroller.value
  if (!element) {
    return
  }
  stickToBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 60
}

async function scrollToBottom() {
  await nextTick()
  if (stickToBottom && !logs.paused && scroller.value) {
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
  <section v-if="open" class="log-console">
    <header class="log-head">
      <div class="log-title">
        <span class="log-dot" />
        <strong>执行日志</strong>
        <span class="log-count">{{ logs.entries.length }}</span>
      </div>
      <div class="log-actions">
        <select
          class="log-filter"
          aria-label="日志级别"
          :value="logs.level"
          @change="logs.setLevel(($event.target as HTMLSelectElement).value as 'all' | LogLevel)"
        >
          <option v-for="item in levels" :key="item.value" :value="item.value">
            {{ item.label }}
          </option>
        </select>
        <button class="log-action" type="button" @click="logs.setPaused(!logs.paused)">
          <i :class="logs.paused ? 'pi pi-play' : 'pi pi-pause'" />
          {{ logs.paused ? '继续' : '暂停' }}
        </button>
        <button class="log-action" type="button" @click="logs.clear()">
          <i class="pi pi-trash" />
          清空
        </button>
        <button
          class="log-action log-close"
          type="button"
          aria-label="关闭控制台"
          @click="emit('close')"
        >
          <i class="pi pi-times" />
        </button>
      </div>
    </header>

    <div ref="scroller" class="log-scroll" @scroll.passive="onScroll">
      <p v-if="logs.visible.length === 0" class="log-empty">等待 Agent 执行日志…</p>
      <div
        v-for="entry in logs.visible"
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

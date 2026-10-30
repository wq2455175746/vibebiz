import { defineStore } from 'pinia'
import { ref } from 'vue'

import type { EditorLog, LogLevel } from '@/types'

const MAX_ENTRIES = 2000

let seq = 0

function normalizeLevel(level: string): LogLevel {
  if (level === 'warn' || level === 'warning') {
    return 'warn'
  }
  if (level === 'error') {
    return 'error'
  }
  return 'info'
}

export const usePreviewLogStore = defineStore('previewLogs', () => {
  const entries = ref<EditorLog[]>([])

  function append(level: string, message: string) {
    const text = message.trim()
    if (!text) {
      return
    }
    seq += 1
    entries.value.push({
      id: `preview-log-${seq}`,
      seq,
      ts: new Date().toISOString(),
      level: normalizeLevel(level),
      source: 'preview',
      message: text.length > 4000 ? `${text.slice(0, 3997)}...` : text,
    })
    if (entries.value.length > MAX_ENTRIES) {
      entries.value.splice(0, entries.value.length - MAX_ENTRIES)
    }
  }

  function clear() {
    entries.value = []
    seq = 0
  }

  return { entries, append, clear }
})

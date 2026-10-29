import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type { EditorLog, LogLevel } from '@/types'

const MAX_ENTRIES = 10_000

let seq = 0

export const useLogStore = defineStore('logs', () => {
  const entries = ref<EditorLog[]>([])
  const paused = ref(false)
  const level = ref<'all' | LogLevel>('all')

  const visible = computed(() =>
    level.value === 'all'
      ? entries.value
      : entries.value.filter((entry) => entry.level === level.value),
  )

  function append(input: Omit<EditorLog, 'id' | 'seq' | 'ts'>) {
    seq += 1
    entries.value.push({
      ...input,
      id: `log-${seq}`,
      seq,
      ts: new Date().toISOString(),
    })
    if (entries.value.length > MAX_ENTRIES) {
      entries.value.splice(0, entries.value.length - MAX_ENTRIES)
    }
  }

  function clear() {
    entries.value = []
    seq = 0
  }

  function setPaused(value: boolean) {
    paused.value = value
  }

  function setLevel(value: 'all' | LogLevel) {
    level.value = value
  }

  return { entries, visible, paused, level, append, clear, setPaused, setLevel }
})

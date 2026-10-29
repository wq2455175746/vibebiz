import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { ApiError } from '@/api/client'
import { listFiles, readFile, writeFile } from '@/api/files'
import type { FileContent, FileEntry } from '@/types'

export const useFileStore = defineStore('files', () => {
  const entries = ref<FileEntry[]>([])
  const activePath = ref<string | null>(null)
  const active = ref<FileContent | null>(null)
  const draft = ref('')
  const loading = ref(false)
  const saving = ref(false)
  const conflict = ref(false)
  const error = ref('')
  const revision = ref(0)

  const dirty = computed(() => active.value !== null && draft.value !== active.value.content)

  async function loadTree(projectId: string) {
    entries.value = (await listFiles(projectId)).filter(
      (entry) => !entry.path.split('/').includes('.chathistory'),
    )
  }

  async function open(projectId: string, path: string) {
    loading.value = true
    error.value = ''
    try {
      const file = await readFile(projectId, path)
      active.value = file
      activePath.value = path
      draft.value = file.content
      conflict.value = false
    } catch (err) {
      error.value = (err as Error).message
    } finally {
      loading.value = false
    }
  }

  async function save(projectId: string): Promise<boolean> {
    if (!active.value || !activePath.value || saving.value) {
      return false
    }
    saving.value = true
    error.value = ''
    try {
      const result = await writeFile(projectId, activePath.value, draft.value, active.value.version)
      active.value = {
        ...active.value,
        content: draft.value,
        version: result.version,
        size: result.size,
        mtime: result.mtime,
      }
      conflict.value = false
      revision.value += 1
      return true
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        conflict.value = true
      } else if (err instanceof ApiError && err.status === 423) {
        error.value = 'Agent 正在开发，请等待完成后再编辑'
      } else {
        error.value = (err as Error).message
      }
      return false
    } finally {
      saving.value = false
    }
  }

  function setDraft(value: string) {
    draft.value = value
  }

  function discard() {
    draft.value = active.value?.content ?? ''
    conflict.value = false
    error.value = ''
  }

  async function reload(projectId: string) {
    if (activePath.value) {
      await open(projectId, activePath.value)
    }
  }

  return {
    entries,
    activePath,
    active,
    draft,
    loading,
    saving,
    conflict,
    error,
    revision,
    dirty,
    loadTree,
    open,
    save,
    setDraft,
    discard,
    reload,
  }
})

<script setup lang="ts">
import type * as Monaco from 'monaco-editor'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import FileTree from './FileTree.vue'
import { useFileStore } from '@/stores/files'
import { detectLanguage } from '@/utils/language'

const props = defineProps<{ projectId: string }>()

const files = useFileStore()
const editorHost = ref<HTMLElement | null>(null)
const monacoFailed = ref(false)
let editor: Monaco.editor.IStandaloneCodeEditor | null = null
let monacoApi: typeof Monaco | null = null

const fileEntries = computed(() => files.entries.filter((entry) => entry.type === 'file'))
const language = computed(() => (files.activePath ? detectLanguage(files.activePath) : 'plaintext'))
const content = computed(() => {
  if (!files.active || files.active.isBinary) {
    return ''
  }
  return files.active.content
})

async function selectDefaultFile() {
  if (fileEntries.value.length === 0) {
    return
  }
  const preferred = fileEntries.value.find((entry) => entry.path === 'index.html')
  const target = preferred ?? fileEntries.value[0]
  if (target && files.activePath !== target.path) {
    await files.open(props.projectId, target.path)
  }
}

async function loadProjectFiles() {
  try {
    if (files.entries.length === 0) {
      await files.loadTree(props.projectId)
    }
  } catch {
    files.entries = []
  }
  await selectDefaultFile()
}

async function select(path: string) {
  await files.open(props.projectId, path)
}

async function mountMonaco() {
  if (!editorHost.value) {
    return
  }
  try {
    const module = await import('@/utils/monaco')
    monacoApi = module.monaco
    editor = monacoApi.editor.create(editorHost.value, {
      value: content.value,
      language: language.value,
      theme: 'vs',
      readOnly: true,
      domReadOnly: true,
      automaticLayout: true,
      minimap: { enabled: false },
      wordWrap: 'off',
      scrollBeyondLastLine: false,
      folding: true,
      fontSize: 12,
      lineHeight: 20,
      padding: { top: 14, bottom: 14 },
      renderLineHighlight: 'none',
      ariaLabel: files.activePath ?? '代码预览',
    })
  } catch {
    monacoFailed.value = true
  }
}

function syncEditor() {
  if (!editor || !monacoApi) {
    return
  }
  if (!files.active || files.active.isBinary) {
    editor.setValue('')
    return
  }
  editor.setValue(files.active.content)
  const model = editor.getModel()
  if (model) {
    monacoApi.editor.setModelLanguage(model, language.value)
  }
}

onMounted(async () => {
  await loadProjectFiles()
  await nextTick()
  await mountMonaco()
})

watch(
  () => props.projectId,
  async () => {
    editor?.setValue('')
    await loadProjectFiles()
    syncEditor()
  },
)

watch(
  () => [files.activePath, files.active?.content, files.active?.isBinary],
  () => syncEditor(),
)

onBeforeUnmount(() => {
  editor?.dispose()
  editor = null
})
</script>

<template>
  <div class="code-panel">
    <FileTree :entries="files.entries" :active-path="files.activePath" @select="select" />

    <section class="code-main">
      <div class="code-head">
        <span class="code-path">{{ files.activePath || '未选择文件' }}</span>
        <span class="tag">{{ language }}</span>
      </div>
      <div v-if="files.loading" class="empty-state code-state">加载中…</div>
      <div v-else-if="!files.active" class="empty-state code-state">选择一个文件查看代码</div>
      <div v-else-if="files.active.isBinary" class="empty-state code-state">
        二进制文件，暂不支持预览
      </div>
      <div v-else class="code-editor">
        <div ref="editorHost" class="monaco-host" />
        <pre v-if="monacoFailed" class="code-fallback">{{ content }}</pre>
      </div>
    </section>
  </div>
</template>

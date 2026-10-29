<script setup lang="ts">
import type * as Monaco from 'monaco-editor'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import FileTree from './FileTree.vue'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { detectLanguage } from '@/utils/language'

const props = defineProps<{ projectId: string }>()

const chat = useChatStore()
const files = useFileStore()
const editorHost = ref<HTMLElement | null>(null)
const monacoFailed = ref(false)
const editing = ref(false)
const notice = ref('')
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
const message = computed(() => {
  if (files.conflict) {
    return '文件已被修改，请取消编辑后重新打开'
  }
  return notice.value || files.error
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

function updateEditorMode() {
  editor?.updateOptions({
    readOnly: !editing.value || chat.busy,
    domReadOnly: !editing.value || chat.busy,
  })
}

async function startEdit() {
  if (!files.active || files.active.isBinary || editing.value) {
    return
  }
  notice.value = ''
  if (chat.busy) {
    notice.value = 'Agent 正在开发，请等待完成后再编辑'
    return
  }
  try {
    if (await chat.syncStatus(props.projectId)) {
      notice.value = 'Agent 正在开发，请等待完成后再编辑'
      return
    }
  } catch {
    notice.value = '无法确认 Agent 状态，请稍后重试'
    return
  }
  editing.value = true
  updateEditorMode()
  await nextTick()
  editor?.focus()
}

async function save() {
  if (!editing.value || files.saving || chat.busy) {
    return
  }
  notice.value = ''
  if (await files.save(props.projectId)) {
    editing.value = false
    notice.value = '已保存'
    updateEditorMode()
  }
}

function cancelEdit() {
  editing.value = false
  notice.value = ''
  files.discard()
  syncEditor()
  updateEditorMode()
}

async function select(path: string) {
  if (editing.value) {
    if (files.dirty) {
      notice.value = '请先保存或取消当前编辑'
      return
    }
    editing.value = false
    updateEditorMode()
  }
  notice.value = ''
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
      readOnly: !editing.value || chat.busy,
      domReadOnly: !editing.value || chat.busy,
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
    editor.onDidChangeModelContent(() => {
      if (editing.value && editor) {
        files.setDraft(editor.getValue())
      }
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
    editing.value = false
    notice.value = ''
    editor?.setValue('')
    await loadProjectFiles()
    syncEditor()
    updateEditorMode()
  },
)

watch(
  () => [files.activePath, files.active?.content, files.active?.isBinary],
  () => syncEditor(),
)

watch(
  () => chat.busy,
  () => updateEditorMode(),
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
        <div class="code-actions">
          <span v-if="files.dirty" class="tag">未保存</span>
          <span class="tag">{{ language }}</span>
          <template v-if="editing">
            <button
              type="button"
              class="code-action primary"
              :disabled="files.saving || chat.busy"
              @click="save"
            >
              保存
            </button>
            <button type="button" class="code-action" @click="cancelEdit">取消</button>
          </template>
          <button v-else type="button" class="code-action" @click="startEdit">编辑</button>
        </div>
      </div>
      <p v-if="message" class="code-message">{{ message }}</p>
      <div v-if="files.loading && !files.active" class="empty-state code-state">加载中…</div>
      <div v-else-if="!files.active" class="empty-state code-state">选择一个文件查看代码</div>
      <div v-else-if="files.active.isBinary" class="empty-state code-state">
        二进制文件，暂不支持预览
      </div>
      <div v-else class="code-editor">
        <div ref="editorHost" class="monaco-host" />
        <div v-if="files.loading" class="code-loading">
          <i class="pi pi-spin pi-spinner" />
          正在加载文件
        </div>
        <pre v-if="monacoFailed" class="code-fallback">{{ content }}</pre>
      </div>
    </section>
  </div>
</template>

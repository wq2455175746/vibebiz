<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Button from 'primevue/button'

import { ApiError } from '@/api/client'
import { runCloneTests } from '@/api/clones'
import { readFile } from '@/api/files'
import { completeProjectStage, listProjectStages, startProjectStage } from '@/api/projects'
import CloneAssetsDialog from '@/components/clone/CloneAssetsDialog.vue'
import ChatPanel from '@/components/editor/ChatPanel.vue'
import CodePane from '@/components/editor/CodePane.vue'
import EditorHeader from '@/components/editor/EditorHeader.vue'
import LogConsole from '@/components/editor/LogConsole.vue'
import PreviewPane from '@/components/editor/PreviewPane.vue'
import DocumentWorkspace from '@/components/enterprise/DocumentWorkspace.vue'
import { useChatStore } from '@/stores/chat'
import { useCloneProjectsStore } from '@/stores/cloneProjects'
import { useFileStore } from '@/stores/files'
import { useLogStore } from '@/stores/logs'
import { useProjectStore } from '@/stores/project'
import type { CloneStageKey, CloneTestResult } from '@/types/clone'
import type { FileContent, StageMutation } from '@/types'
import {
  CLONE_STAGES,
  buildCloneStagePrompt,
  cloneStageDefinition,
  cloneStagesFromProject,
  createInitialCloneStages,
  formatCloneSize,
} from '@/utils/clone'
import { renderMarkdown } from '@/utils/markdown'
import '@/styles/editor.css'
import '@/styles/enterprise.css'
import '@/styles/clone.css'

const route = useRoute()
const router = useRouter()
const cloneStore = useCloneProjectsStore()
const projectStore = useProjectStore()
const files = useFileStore()
const chat = useChatStore()
const logs = useLogStore()

const id = computed(() => String(route.params.id))
const project = computed(() => {
  if (cloneStore.current?.id === id.value) {
    return cloneStore.current
  }
  return cloneStore.projects.find((item) => item.id === id.value) ?? null
})
const stageStates = ref(createInitialCloneStages())
const viewingKey = ref<CloneStageKey | null>(null)
const assetsOpen = ref(false)
const artifact = ref<FileContent | null>(null)
const report = ref<FileContent | null>(null)
const artifactLoading = ref(false)
const artifactError = ref('')
const stageLoadError = ref('')
const stageError = ref('')
const starting = ref(false)
const confirming = ref(false)
const testRunning = ref(false)
const testResult = ref<CloneTestResult | null>(null)
const testLogs = ref<string[]>([])
const devTab = ref<'preview' | 'code' | 'console'>('preview')
const previewReady = ref(false)

const activeKey = computed<CloneStageKey>(() => {
  return (
    CLONE_STAGES.find((stage) => stageStates.value[stage.key].status === 'active')?.key ??
    CLONE_STAGES[CLONE_STAGES.length - 1].key
  )
})
const activeStage = computed(() => cloneStageDefinition(activeKey.value))
const displayedKey = computed(() => viewingKey.value ?? activeKey.value)
const displayedStage = computed(() => cloneStageDefinition(displayedKey.value))
const activeState = computed(() => stageStates.value[activeKey.value])
const isHistory = computed(
  () => Boolean(viewingKey.value && viewingKey.value !== activeKey.value),
)
const allDone = computed(() =>
  CLONE_STAGES.every((stage) => stageStates.value[stage.key].status === 'done'),
)
const testReady = computed(
  () => Boolean(testResult.value) || Boolean(report.value?.content.trim()),
)
const canStart = computed(
  () =>
    !isHistory.value &&
    activeState.value.status === 'active' &&
    !activeState.value.startedAt &&
    !chat.busy &&
    !starting.value &&
    !confirming.value,
)
const canConfirm = computed(
  () =>
    !isHistory.value &&
    activeState.value.status === 'active' &&
    Boolean(activeState.value.startedAt) &&
    !chat.busy &&
    !confirming.value &&
    (displayedKey.value !== 'test' || testReady.value),
)
const displayedContent = computed(() => artifact.value?.content ?? '')
const testHtml = computed(() => {
  const sections = [artifact.value?.content ?? '', report.value?.content ?? ''].filter((item) =>
    item.trim(),
  )
  return sections.length > 0 ? renderMarkdown(sections.join('\n\n---\n\n')) : ''
})
const previewRevision = computed(() => chat.revision + files.revision)
const assets = computed(() => project.value?.assets ?? [])
const unitTest = computed(
  () => assets.value.find((asset) => asset.role === 'unit_test') ?? null,
)
const docs = computed(() => assets.value.filter((asset) => asset.role === 'docs'))
const imageCount = computed(
  () =>
    assets.value.filter((asset) => asset.role === 'cover' || asset.role === 'gallery')
      .length,
)

function applyMutation(result: StageMutation) {
  const next = { ...stageStates.value }
  for (const stage of [result.stage, result.nextStage]) {
    if (!stage || !CLONE_STAGES.some((item) => item.key === stage.stageKey)) {
      continue
    }
    const key = stage.stageKey as CloneStageKey
    next[key] = {
      status: stage.status === 'placeholder' ? 'locked' : stage.status,
      startedAt: stage.startedAt ?? null,
      completedAt: stage.completedAt ?? null,
    }
  }
  stageStates.value = next
}

async function loadStages(projectId: string) {
  try {
    const result = await listProjectStages(projectId)
    stageStates.value = cloneStagesFromProject(result.stages)
    stageLoadError.value = ''
  } catch (err) {
    stageLoadError.value = (err as Error).message || '阶段状态加载失败'
    stageStates.value = createInitialCloneStages()
  }
}

async function loadArtifact() {
  const stage = displayedStage.value
  artifact.value = null
  report.value = null
  artifactError.value = ''
  if (!stage.artifactPath) {
    return
  }
  artifactLoading.value = true
  try {
    artifact.value = await readFile(id.value, stage.artifactPath)
  } catch (err) {
    if (!(err instanceof ApiError && err.status === 404)) {
      artifactError.value = (err as Error).message
    }
  }
  if (displayedKey.value === 'test') {
    try {
      report.value = await readFile(id.value, 'docs/clone/test-report.md')
    } catch (err) {
      if (!(err instanceof ApiError && err.status === 404)) {
        artifactError.value = (err as Error).message
      }
    }
  }
  artifactLoading.value = false
}

async function load() {
  const projectId = id.value
  viewingKey.value = null
  previewReady.value = false
  stageError.value = ''
  stageLoadError.value = ''
  testResult.value = null
  testLogs.value = []
  chat.reset()
  logs.clear()
  const loaded = await cloneStore.loadOne(projectId)
  if (!loaded) {
    void router.replace({ name: 'clones' })
    return
  }
  await projectStore.load(projectId)
  await loadStages(projectId)
  try {
    await files.loadTree(projectId)
  } catch {
    files.entries = []
  }
  try {
    await chat.loadHistory(projectId)
  } catch {
    chat.reset()
  }
  await chat.syncStatus(projectId).catch(() => false)
  await logs.load(projectId).catch(() => undefined)
  await loadArtifact()
  previewReady.value = true
}

async function startStage() {
  if (!canStart.value) {
    return
  }
  starting.value = true
  stageError.value = ''
  try {
    const result = await startProjectStage(id.value, activeKey.value)
    applyMutation(result)
    const prompt = buildCloneStagePrompt(activeStage.value, project.value ?? {
      name: '', description: '', sourceUrl: '',
    })
    if (prompt) {
      void chat.send(id.value, prompt, [])
    }
  } catch (err) {
    stageError.value = (err as Error).message
  } finally {
    starting.value = false
  }
}

async function confirmStage() {
  if (!canConfirm.value) {
    return
  }
  confirming.value = true
  stageError.value = ''
  try {
    const result = await completeProjectStage(id.value, activeKey.value)
    applyMutation(result)
    viewingKey.value = null
    devTab.value = 'preview'
    await loadArtifact()
    await files.loadTree(id.value).catch(() => undefined)
  } catch (err) {
    stageError.value = (err as Error).message
  } finally {
    confirming.value = false
  }
}

async function runTests() {
  if (testRunning.value) {
    return
  }
  testRunning.value = true
  stageError.value = ''
  testLogs.value = ['正在准备测试环境…']
  try {
    const result = await runCloneTests(id.value)
    testResult.value = result
    testLogs.value = result.output.trim()
      ? result.output.trim().split('\n')
      : [`状态：${result.status}`]
    await loadArtifact()
    await files.loadTree(id.value).catch(() => undefined)
  } catch (err) {
    stageError.value = (err as Error).message
    testLogs.value = [...testLogs.value, (err as Error).message]
  } finally {
    testRunning.value = false
  }
}

function selectable(key: CloneStageKey): boolean {
  const status = stageStates.value[key].status
  return status === 'done' || status === 'active'
}

function selectStage(key: CloneStageKey) {
  if (!selectable(key)) {
    return
  }
  viewingKey.value = key === activeKey.value ? null : key
}

function stageClass(key: CloneStageKey) {
  return [
    `status-${stageStates.value[key].status}`,
    {
      viewed: viewingKey.value
        ? viewingKey.value === key
        : activeKey.value === key,
    },
  ]
}

function stageStatusText(key: CloneStageKey): string {
  const status = stageStates.value[key].status
  if (status === 'done') return '已完成'
  if (status === 'active') return '进行中'
  return '未解锁'
}

function backToCurrent() {
  viewingKey.value = null
  devTab.value = 'preview'
}

function backToClones() {
  void router.push({ name: 'clones' })
}

watch(id, load)
watch(displayedKey, () => {
  devTab.value = 'preview'
})
watch(
  () => [displayedKey.value, chat.revision, files.revision] as const,
  () => void loadArtifact(),
)
watch(
  () => chat.revision,
  async () => {
    try {
      await files.loadTree(id.value)
      if (files.active && !files.dirty) {
        await files.reload(id.value)
      }
    } catch {
      files.entries = []
    }
  },
)

onMounted(load)
onBeforeUnmount(() => {
  chat.disconnect()
})
</script>

<template>
  <div v-if="project" class="editor-shell clone-editor-shell">
    <EditorHeader
      :project-name="project.name"
      :project-state="allDone ? '已完成' : activeStage.title"
      :roles="[]"
      model-label="Clone"
      :show-actions="false"
      @back="backToClones"
    />

    <nav class="enterprise-stage-bar clone-stage-bar" aria-label="克隆阶段进度">
      <button
        v-for="stage in CLONE_STAGES"
        :key="stage.key"
        class="enterprise-stage-step"
        :class="stageClass(stage.key)"
        type="button"
        :disabled="!selectable(stage.key)"
        :aria-current="activeKey === stage.key ? 'step' : undefined"
        @click="selectStage(stage.key)"
      >
        <span class="enterprise-stage-index">
          <i v-if="stageStates[stage.key].status === 'done'" class="pi pi-check" />
          <span v-else>{{ stage.seq }}</span>
        </span>
        <span class="enterprise-stage-copy">
          <strong>{{ stage.title }}</strong>
          <small>{{ stage.roleLabel }}</small>
        </span>
        <span
          v-if="stage.key === activeKey && stageStates[stage.key].status === 'active'"
          class="enterprise-stage-pulse"
        >
          进行中
        </span>
      </button>
    </nav>

    <p v-if="stageLoadError" class="editor-error">{{ stageLoadError }}</p>

    <div class="editor-body">
      <ChatPanel
        :project-id="id"
        :readonly="isHistory || displayedStage.workspace === 'artifact'"
        required-role-code="team"
      />

      <main class="editor-workbench clone-workbench">
        <header class="enterprise-stage-head clone-stage-head">
          <div class="enterprise-stage-title">
            <p class="enterprise-stage-eyebrow">
              阶段 {{ displayedStage.seq }} / {{ CLONE_STAGES.length }}
            </p>
            <h1>{{ displayedStage.title }}</h1>
            <p class="clone-stage-summary">
              {{ displayedStage.summary }}
              <span v-if="displayedStage.artifactPath">· {{ displayedStage.artifactPath }}</span>
            </p>
          </div>
          <div class="clone-stage-actions">
            <Button
              v-if="isHistory"
              label="回到当前阶段"
              text
              size="small"
              @click="backToCurrent"
            />
            <Button
              v-else-if="canStart"
              label="开始阶段"
              size="small"
              :loading="starting"
              @click="startStage"
            />
            <Button
              v-else-if="canConfirm"
              :label="displayedKey === 'artifacts' ? '确认完成' : '确认并继续'"
              size="small"
              :loading="confirming"
              @click="confirmStage"
            />
            <span v-else class="tag">{{ stageStatusText(displayedKey) }}</span>
          </div>
        </header>

        <div class="clone-source-strip">
          <div class="clone-source-thumb">
            <img v-if="project.coverUrl" :src="project.coverUrl" :alt="project.name" />
            <i v-else class="pi pi-image" />
          </div>
          <div class="clone-source-strip-copy">
            <strong>来源资料</strong>
            <span>{{ imageCount }} / 9 图片 · {{ docs.length }} 份文档</span>
          </div>
          <Button label="补充" text size="small" @click="assetsOpen = true" />
        </div>

        <p v-if="stageError" class="clone-form-error clone-stage-error">{{ stageError }}</p>

        <DocumentWorkspace
          v-if="displayedStage.workspace === 'document'"
          :content="displayedContent"
          :path="displayedStage.artifactPath"
          :role-label="displayedStage.roleLabel"
          :loading="artifactLoading"
          :error="artifactError"
        />

        <section v-else-if="displayedStage.workspace === 'test'" class="clone-test-workspace">
          <div class="clone-test-head">
            <div>
              <strong>单元测试</strong>
              <span v-if="unitTest">
                {{ unitTest.displayName || unitTest.name }} · {{ formatCloneSize(unitTest.size) }}
              </span>
              <span v-else>未上传单测 zip，将尝试运行项目内测试</span>
            </div>
            <Button
              label="运行单测"
              size="small"
              :loading="testRunning"
              :disabled="isHistory || !activeState.startedAt"
              @click="runTests"
            />
          </div>
          <div class="clone-test-body">
            <article v-if="testHtml" class="clone-doc-body" v-html="testHtml" />
            <div v-else class="clone-test-empty">
              <i class="pi pi-file-edit" />
              <strong>等待测试方案与报告</strong>
              <span>开始阶段后由 Agent 生成方案；运行单测会写入测试报告。</span>
            </div>
            <aside class="clone-test-log">
              <strong>执行日志</strong>
              <pre>{{ testLogs.length ? testLogs.join('\n') : '尚未执行。' }}</pre>
              <span v-if="!testReady" class="clone-asset-hint">运行单测生成报告后才能确认本阶段。</span>
            </aside>
          </div>
        </section>

        <section v-else-if="displayedStage.workspace === 'development'" class="clone-dev-workspace">
          <div class="workbench-head">
            <div class="workbench-tabs">
              <button
                type="button"
                :class="{ active: devTab === 'preview' }"
                @click="devTab = 'preview'"
              >
                <i class="pi pi-desktop" />
                应用查看器
              </button>
              <button type="button" :class="{ active: devTab === 'code' }" @click="devTab = 'code'">
                <i class="pi pi-code" />
                代码
              </button>
              <button
                type="button"
                :class="{ active: devTab === 'console' }"
                @click="devTab = 'console'"
              >
                <i class="pi pi-server" />
                终端
              </button>
            </div>
            <span class="workbench-meta">实时工作区</span>
          </div>
          <div class="workbench-body clone-dev-body">
            <PreviewPane
              v-if="previewReady"
              v-show="devTab === 'preview'"
              :project-id="id"
              :refresh-token="previewRevision"
              :active="devTab === 'preview'"
            />
            <CodePane v-if="devTab === 'code'" :project-id="id" :locked="isHistory" />
            <LogConsole
              v-else-if="devTab === 'console'"
              class="workbench-log"
              :open="true"
              @close="devTab = 'preview'"
            />
          </div>
        </section>

        <section v-else class="clone-artifact-workspace">
          <div class="clone-artifact-head">
            <div>
              <strong>制品空间</strong>
              <span>代码、文档与测试报告已归档到工作区，不再进入交付空间。</span>
            </div>
          </div>
          <div class="clone-artifact-summary">
            <div>
              <span>项目</span>
              <strong>{{ project.name }}</strong>
            </div>
            <div>
              <span>文件</span>
              <strong>{{ files.entries.filter((entry) => entry.type === 'file').length }}</strong>
            </div>
            <div>
              <span>图片 / 文档</span>
              <strong>{{ imageCount }} / {{ docs.length }}</strong>
            </div>
            <div>
              <span>单元测试</span>
              <strong>{{ unitTest ? '已上传' : '未上传' }}</strong>
            </div>
          </div>
          <div class="clone-artifact-note">
            <i class="pi pi-box" />
            <p>制品流水线接入后，会在这里生成源码归档、镜像与版本说明。</p>
          </div>
        </section>
      </main>
    </div>

    <CloneAssetsDialog v-model:visible="assetsOpen" :project-id="id" />
  </div>

  <div v-else class="clone-missing">
    <p>克隆项目不存在或已被删除。</p>
    <Button label="返回克隆页" @click="backToClones" />
  </div>
</template>

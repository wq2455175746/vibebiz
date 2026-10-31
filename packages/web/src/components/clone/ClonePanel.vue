<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'

import { listProjectStages } from '@/api/projects'
import { useCloneProjectsStore } from '@/stores/cloneProjects'
import type { ProjectStage } from '@/types'
import type { CloneAssetView, CloneProject } from '@/types/clone'
import { CLONE_STAGES, cloneStagesFromProject, formatCloneSize } from '@/utils/clone'
import CloneCreateDialog from './CloneCreateDialog.vue'
import type { CloneCreatePayload } from './CloneCreateDialog.vue'

const router = useRouter()
const store = useCloneProjectsStore()
const createOpen = ref(false)
const selectedId = ref<string | null>(null)
const selectedStages = ref<ProjectStage[]>([])
const stagesLoading = ref(false)

const projects = computed(() => store.projects)
const selected = computed(
  () => projects.value.find((project) => project.id === selectedId.value) ?? null,
)
const stageStates = computed(() => cloneStagesFromProject(selectedStages.value))
const currentStageTitle = computed(
  () =>
    CLONE_STAGES.find((stage) => stageStates.value[stage.key].status === 'active')?.title ??
    (selected.value?.state === 'ready' ? '待开始' : selected.value?.state ?? '—'),
)

function doneCount(): number {
  return CLONE_STAGES.filter((stage) => stageStates.value[stage.key].status === 'done').length
}

async function loadSelectedStages(projectId: string | null) {
  if (!projectId) {
    selectedStages.value = []
    return
  }
  stagesLoading.value = true
  try {
    selectedStages.value = (await listProjectStages(projectId)).stages
  } catch {
    selectedStages.value = []
  } finally {
    stagesLoading.value = false
  }
}

function hostOf(project: CloneProject): string {
  if (!project.sourceUrl) {
    return '未填写 URL'
  }
  try {
    return new URL(project.sourceUrl).host
  } catch {
    return project.sourceUrl
  }
}

function formatDate(value: string): string {
  return new Date(value).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function imageCount(project: CloneProject): number {
  return project.assets.filter(
    (asset) => asset.role === 'cover' || asset.role === 'gallery',
  ).length
}

function docsOf(project: CloneProject) {
  return project.assets.filter((asset) => asset.role === 'docs')
}

function unitTestOf(project: CloneProject): CloneAssetView | null {
  return project.assets.find((asset) => asset.role === 'unit_test') ?? null
}

function openCreate() {
  store.error = ''
  createOpen.value = true
}

async function handleCreate(payload: CloneCreatePayload) {
  try {
    const project = await store.create(payload.draft, payload.assets, payload.coverAssetId)
    createOpen.value = false
    void router.push({ name: 'clone-editor', params: { id: project.id } })
  } catch {
    createOpen.value = true
  }
}

function selectProject(project: CloneProject) {
  selectedId.value = project.id
}

function openProject(project: CloneProject) {
  void router.push({ name: 'clone-editor', params: { id: project.id } })
}

async function removeProject(project: CloneProject) {
  if (!window.confirm(`确定删除克隆项目「${project.name}」吗？`)) {
    return
  }
  try {
    await store.remove(project.id)
    if (selectedId.value === project.id) {
      selectedId.value = projects.value[0]?.id ?? null
    }
  } catch {
    return
  }
}

watch(
  projects,
  (list) => {
    if (!selectedId.value || !list.some((project) => project.id === selectedId.value)) {
      selectedId.value = list[0]?.id ?? null
    }
  },
  { immediate: true },
)

watch(selectedId, (projectId) => void loadSelectedStages(projectId), { immediate: true })

onMounted(async () => {
  await store.load()
  if (!selectedId.value) {
    selectedId.value = projects.value[0]?.id ?? null
  }
})
</script>

<template>
  <section class="clone-panel">
    <header class="clone-head">
      <div>
        <h1>克隆</h1>
        <p>拆解目标软件的技术架构，再按阶段复刻为可运行的制品。</p>
      </div>
      <Button v-if="projects.length > 0" label="创建克隆" @click="openCreate" />
    </header>

    <p v-if="store.error" class="clone-form-error">{{ store.error }}</p>

    <div v-if="store.loading && projects.length === 0" class="clone-empty">
      <div class="clone-empty-inner">
        <span class="clone-empty-icon"><i class="pi pi-spin pi-spinner" /></span>
        <p>加载克隆项目…</p>
      </div>
    </div>

    <div v-else-if="projects.length === 0" class="clone-empty">
      <div class="clone-empty-inner">
        <span class="clone-empty-icon"><i class="pi pi-copy" /></span>
        <h2>克隆一个软件或网站</h2>
        <p>上传参考资料，让 Vibebiz 先拆解目标架构，再按阶段复刻。</p>
        <Button label="创建克隆" @click="openCreate" />
      </div>
    </div>

    <div v-else class="clone-layout">
      <aside class="clone-list">
        <div class="clone-list-head">
          <strong>克隆项目</strong>
          <span>{{ projects.length }}</span>
        </div>
        <button
          v-for="project in projects"
          :key="project.id"
          type="button"
          class="clone-list-item"
          :class="{ active: project.id === selectedId }"
          @click="selectProject(project)"
        >
          <span class="clone-list-title">{{ project.name }}</span>
          <span class="clone-list-url">{{ hostOf(project) }}</span>
          <span class="clone-list-meta">{{ formatDate(project.updatedAt) }}</span>
        </button>
      </aside>

      <article v-if="selected" class="clone-detail">
        <header class="clone-detail-head">
          <div>
            <h2>{{ selected.name }}</h2>
            <p>{{ hostOf(selected) }}</p>
          </div>
          <span class="tag">{{ currentStageTitle }}</span>
        </header>

        <div class="clone-detail-actions">
          <Button label="打开编辑器" size="small" @click="openProject(selected)" />
          <Button label="删除" text size="small" @click="removeProject(selected)" />
        </div>

        <div class="clone-detail-grid">
          <div class="full">
            <span>描述</span>
            <p>{{ selected.description }}</p>
          </div>
          <div>
            <span>目标 URL</span>
            <p>{{ selected.sourceUrl || '未填写（仅按描述与附件复刻）' }}</p>
          </div>
          <div>
            <span>阶段进度</span>
            <p>{{ doneCount() }} / {{ CLONE_STAGES.length }}</p>
          </div>
          <div>
            <span>创建时间</span>
            <p>{{ formatDate(selected.createdAt) }}</p>
          </div>
          <div>
            <span>最近更新</span>
            <p>{{ formatDate(selected.updatedAt) }}</p>
          </div>
        </div>

        <section class="clone-detail-section">
          <div class="clone-detail-section-head">
            <strong>来源资料</strong>
            <span>{{ imageCount(selected) }} 张图片</span>
          </div>
          <div class="clone-source">
            <div class="clone-source-cover">
              <img v-if="selected.coverUrl" :src="selected.coverUrl" :alt="selected.name" />
              <span v-else><i class="pi pi-image" /></span>
            </div>
            <div class="clone-source-copy">
              <p>
                <strong>单元测试</strong>
                <span v-if="unitTestOf(selected)">
                  {{ unitTestOf(selected)?.name }} · {{ formatCloneSize(unitTestOf(selected)?.size ?? 0) }}
                </span>
                <span v-else>未上传</span>
              </p>
              <p>
                <strong>文档说明</strong>
                <span v-if="docsOf(selected).length > 0">
                  {{ docsOf(selected).map((doc) => doc.name).join('、') }}
                </span>
                <span v-else>未上传</span>
              </p>
              <p>
                <strong>图片</strong>
                <span>{{ imageCount(selected) }} / 9</span>
              </p>
            </div>
          </div>
        </section>

        <section class="clone-detail-section">
          <div class="clone-detail-section-head">
            <strong>阶段</strong>
            <span>逐步确认</span>
          </div>
          <p v-if="stagesLoading" class="clone-asset-hint">加载阶段中…</p>
          <ol v-else class="clone-progress-list">
            <li
              v-for="stage in CLONE_STAGES"
              :key="stage.key"
              :class="stageStates[stage.key].status"
            >
              <span class="clone-progress-index">{{ stage.seq }}</span>
              <span class="clone-progress-copy">
                <strong>{{ stage.title }}</strong>
                <small>{{ stage.summary }}</small>
              </span>
              <span class="clone-progress-state">
                {{
                  stageStates[stage.key].status === 'done'
                    ? '已完成'
                    : stageStates[stage.key].status === 'active'
                      ? '进行中'
                      : '未解锁'
                }}
              </span>
            </li>
          </ol>
        </section>
      </article>
    </div>

    <CloneCreateDialog
      v-model:visible="createOpen"
      :submitting="store.creating"
      :submit-error="store.creating ? '' : store.error"
      @submit="handleCreate"
    />
  </section>
</template>

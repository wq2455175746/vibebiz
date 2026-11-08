<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'

import { listPublishedProjects } from '@/api/publish'
import type { PublishedProject } from '@/types'

const props = defineProps<{ existingIds: string[]; full: boolean; error?: string }>()
const emit = defineEmits<{ close: []; add: [project: PublishedProject] }>()

const projects = ref<PublishedProject[]>([])
const loading = ref(true)
const loadError = ref('')
const pending = ref<string[]>([])
const existing = computed(() => new Set(props.existingIds))

function disabled(project: PublishedProject): boolean {
  return props.full || existing.value.has(project.projectId) || pending.value.includes(project.projectId)
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    projects.value = await listPublishedProjects()
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '加载已发布项目失败'
  } finally {
    loading.value = false
  }
}

function add(project: PublishedProject) {
  if (pending.value.includes(project.projectId)) return
  pending.value = [...pending.value, project.projectId]
  emit('add', project)
}

watch(
  () => props.error,
  () => {
    pending.value = []
  },
)

onMounted(load)
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <div class="dialog-card add-project-dialog">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">内容模块</p>
          <h2>添加已发布项目</h2>
        </div>
        <button class="dialog-close" type="button" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <p class="add-project-note">只能添加自己已经发布的项目，最多 5 个。</p>
      <p v-if="full" class="add-project-note">已达到 5 个上限，请先移除一个项目。</p>
      <p v-if="props.error" class="error-text" role="alert">{{ props.error }}</p>
      <div v-if="loading" class="dialog-state">加载中...</div>
      <div v-else-if="loadError" class="dialog-state">
        <span class="error-text">{{ loadError }}</span>
        <button class="dialog-retry" type="button" @click="load">重试</button>
      </div>
      <p v-else-if="projects.length === 0" class="dialog-state">暂无已发布项目</p>
      <ul v-else class="add-project-list">
        <li v-for="project in projects" :key="project.projectId" class="add-project-row">
          <img
            v-if="project.coverUrl"
            class="add-project-cover"
            :src="project.coverUrl"
            :alt="project.name"
          />
          <span v-else class="add-project-cover placeholder"><i class="pi pi-image" /></span>
          <div class="add-project-info">
            <strong>{{ project.name }}</strong>
            <p>{{ project.description || '暂无简介' }}</p>
          </div>
          <button
            class="add-project-action"
            type="button"
            :disabled="disabled(project)"
            @click="add(project)"
          >
            {{ existing.has(project.projectId) ? '已添加' : '添加' }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

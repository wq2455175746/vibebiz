<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'

import { listPublishedProjects } from '@/api/publish'
import type { PublishedProject } from '@/types'

const props = defineProps<{ existingIds: string[]; full: boolean }>()
const emit = defineEmits<{ close: []; add: [project: PublishedProject] }>()

const projects = ref<PublishedProject[]>([])
const loading = ref(true)
const error = ref('')
const existing = computed(() => new Set(props.existingIds))

function disabled(project: PublishedProject): boolean {
  return props.full || existing.value.has(project.projectId)
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    projects.value = await listPublishedProjects()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载已发布项目失败'
  } finally {
    loading.value = false
  }
}

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
      <div v-if="loading" class="dialog-state">加载中...</div>
      <div v-else-if="error" class="dialog-state">
        <span class="error-text">{{ error }}</span>
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
            @click="emit('add', project)"
          >
            {{ existing.has(project.projectId) ? '已添加' : '添加' }}
          </button>
        </li>
      </ul>
    </div>
  </div>
</template>

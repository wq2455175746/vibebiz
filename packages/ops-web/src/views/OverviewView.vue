<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { getOverview, getProjects } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import type { AdminProject, Overview, TableColumn } from '@/types'
import { formatDateTime, formatNumber } from '@/utils/format'

const overview = ref<Overview | null>(null)
const projects = ref<AdminProject[]>([])
const loading = ref(true)
const overviewError = ref('')
const projectsError = ref('')

const projectColumns: TableColumn[] = [
  { key: 'name', label: '项目', width: '240px' },
  { key: 'state', label: '状态', width: '110px', tag: true, highlight: ['ready', 'running'] },
  { key: 'owner', label: '所有者', width: '160px' },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
]

function message(error: unknown): string {
  return error instanceof Error ? error.message : '加载失败'
}

function metric(value: number | undefined): string {
  if (loading.value) {
    return '...'
  }
  if (!overview.value || value === undefined) {
    return '—'
  }
  return formatNumber(value)
}

async function load() {
  loading.value = true
  overviewError.value = ''
  projectsError.value = ''
  const [overviewResult, projectsResult] = await Promise.allSettled([
    getOverview(),
    getProjects({ page: 1, pageSize: 5 }),
  ])
  if (overviewResult.status === 'fulfilled') {
    overview.value = overviewResult.value
  } else {
    overviewError.value = message(overviewResult.reason)
  }
  if (projectsResult.status === 'fulfilled') {
    projects.value = projectsResult.value.items
  } else {
    projectsError.value = message(projectsResult.reason)
  }
  loading.value = false
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>概览</h1>
        <p class="page-desc">本地控制面运行状态与最近项目。</p>
      </div>
      <span class="tag">LOCAL</span>
    </header>

    <p v-if="overviewError" class="error-text inline-error">{{ overviewError }}</p>

    <section class="metric-grid">
      <article class="metric-card">
        <p class="metric-label">项目总数</p>
        <p class="metric-value">{{ metric(overview?.projects) }}</p>
      </article>
      <article class="metric-card">
        <p class="metric-label">用户数</p>
        <p class="metric-value">{{ metric(overview?.users) }}</p>
      </article>
      <article class="metric-card">
        <p class="metric-label">运行中沙箱</p>
        <p class="metric-value">{{ metric(overview?.runningSandboxes) }}</p>
      </article>
      <article class="metric-card">
        <p class="metric-label">今日请求</p>
        <p class="metric-value">{{ metric(overview?.todayRequests) }}</p>
      </article>
    </section>

    <section class="section">
      <div class="section-head">
        <h2>最近项目</h2>
        <span class="muted small">最多显示 5 条</span>
      </div>
      <DataTable
        :columns="projectColumns"
        :rows="projects"
        :loading="loading"
        :error="projectsError"
        empty-text="暂无项目"
        @retry="load"
      />
    </section>
  </div>
</template>

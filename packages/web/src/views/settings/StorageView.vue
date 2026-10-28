<script setup lang="ts">
import Button from 'primevue/button'
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { getStorage } from '@/api/storage'
import type { StorageSummary } from '@/types'
import { formatBytes, formatDateTime, storageRatio } from '@/utils/storage'

const router = useRouter()
const loading = ref(true)
const error = ref('')
const summary = ref<StorageSummary | null>(null)

const ratio = computed(() =>
  summary.value ? storageRatio(summary.value.usedBytes, summary.value.quotaBytes) : 0,
)
const percent = computed(() => `${(ratio.value * 100).toFixed(2)}%`)

function openProject(id: string) {
  void router.push({ name: 'editor', params: { id } })
}

function projectPercent(value: number): string {
  return `${(Math.max(value, 0) * 100).toFixed(2)}%`
}

onMounted(async () => {
  try {
    summary.value = await getStorage()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '磁盘空间加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>磁盘空间</h1>
        <p>当前工作区的存储使用情况。</p>
      </div>
    </header>

    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="error" class="settings-error" role="alert">{{ error }}</p>

    <template v-else-if="summary">
      <div class="settings-card storage-overview">
        <div class="storage-overview-head">
          <span>已使用 {{ formatBytes(summary.usedBytes) }}</span>
          <span class="muted">共 {{ formatBytes(summary.quotaBytes) }}</span>
        </div>
        <div class="storage-bar" role="progressbar" :aria-valuenow="Math.round(ratio * 100)">
          <span :style="{ width: percent }" />
        </div>
        <div class="storage-meta">
          <span>{{ percent }}</span>
          <span>{{ summary.projectCount }} 个项目</span>
        </div>
      </div>

      <div v-if="summary.projects.length === 0" class="empty-state">当前工作区还没有项目</div>

      <div v-else class="settings-card settings-table-wrap">
        <table class="settings-table">
          <thead>
            <tr>
              <th>项目</th>
              <th>大小</th>
              <th>占比</th>
              <th>文件数</th>
              <th>最后修改</th>
              <th aria-label="操作" />
            </tr>
          </thead>
          <tbody>
            <tr v-for="project in summary.projects" :key="project.id">
              <td class="cell-name">{{ project.name }}</td>
              <td>{{ project.available ? formatBytes(project.sizeBytes) : '不可用' }}</td>
              <td>{{ project.available ? projectPercent(project.ratio) : '—' }}</td>
              <td>{{ project.available ? project.fileCount : '—' }}</td>
              <td>{{ project.available ? formatDateTime(project.lastModifiedAt) : '—' }}</td>
              <td class="cell-action">
                <Button label="打开" size="small" text @click="openProject(project.id)" />
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </section>
</template>

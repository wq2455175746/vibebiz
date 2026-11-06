<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Select from 'primevue/select'

import { getOverview, getProjects, getUsageDaily, getUsers } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import UsageTrendChart from '@/components/UsageTrendChart.vue'
import type { AdminProject, AdminUser, Overview, TableColumn, UsageDailyResult } from '@/types'
import { formatDateTime, formatNumber } from '@/utils/format'
import {
  buildProjectsOption,
  buildRequestsOption,
  buildTokensOption,
  buildUploadsOption,
  dateRangeForDays,
  fillUsageSeries,
} from '@/utils/usage'

const overview = ref<Overview | null>(null)
const projects = ref<AdminProject[]>([])
const users = ref<AdminUser[]>([])
const usage = ref<UsageDailyResult | null>(null)
const loading = ref(true)
const usageLoading = ref(true)
const overviewError = ref('')
const projectsError = ref('')
const usageError = ref('')
const rangeDays = ref(30)
const selectedUserId = ref<number | null>(null)

const projectColumns: TableColumn[] = [
  { key: 'name', label: '项目', width: '240px' },
  { key: 'state', label: '状态', width: '110px', tag: true, highlight: ['ready', 'running'] },
  { key: 'owner', label: '所有者', width: '160px' },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
]

const userOptions = computed(() => [
  { label: '全部用户', value: null },
  ...users.value.map((user) => ({ label: `${user.name} (${user.id})`, value: Number(user.id) })),
])

const dates = computed(() => dateRangeForDays(rangeDays.value))
const points = computed(() => fillUsageSeries(usage.value?.series ?? [], dates.value))
const projectsOption = computed(() => buildProjectsOption(points.value))
const tokensOption = computed(() => buildTokensOption(points.value))
const requestsOption = computed(() => buildRequestsOption(points.value))
const uploadsOption = computed(() => buildUploadsOption(points.value))

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

async function loadUsage() {
  usageLoading.value = true
  usageError.value = ''
  const days = dateRangeForDays(rangeDays.value)
  try {
    usage.value = await getUsageDaily({
      from: days[0],
      to: days[days.length - 1],
      userId: selectedUserId.value ?? undefined,
    })
  } catch (error) {
    usageError.value = message(error)
  } finally {
    usageLoading.value = false
  }
}

async function load() {
  loading.value = true
  overviewError.value = ''
  projectsError.value = ''
  const [overviewResult, projectsResult, usersResult] = await Promise.allSettled([
    getOverview(),
    getProjects({ page: 1, pageSize: 5 }),
    getUsers({ page: 1, pageSize: 100 }),
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
  if (usersResult.status === 'fulfilled') {
    users.value = usersResult.value.items
  }
  loading.value = false
  await loadUsage()
}

function selectRange(days: number) {
  if (rangeDays.value === days) {
    return
  }
  rangeDays.value = days
  void loadUsage()
}

function selectUser(value: number | null) {
  selectedUserId.value = value
  void loadUsage()
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>概览</h1>
        <p class="page-desc">本地控制面运行状态、使用趋势与最近项目。</p>
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
      <div class="section-head usage-head">
        <div>
          <h2>使用趋势</h2>
          <span class="muted small">按自然日统计</span>
        </div>
        <div class="usage-filters">
          <div class="range-switch">
            <button
              v-for="days in [7, 30, 90]"
              :key="days"
              type="button"
              :class="{ active: rangeDays === days }"
              @click="selectRange(days)"
            >
              最近 {{ days }} 天
            </button>
          </div>
          <Select
            :model-value="selectedUserId"
            :options="userOptions"
            option-label="label"
            option-value="value"
            placeholder="全部用户"
            class="user-select"
            @update:model-value="selectUser"
          />
        </div>
      </div>

      <p v-if="usageError" class="error-text inline-error">{{ usageError }}</p>

      <div class="chart-grid">
        <article class="chart-card">
          <div class="chart-head">
            <h3>每日创建项目数</h3>
            <span class="muted small">{{ formatNumber(usage?.totals.projectsCreated ?? 0) }}</span>
          </div>
          <UsageTrendChart :option="projectsOption" />
        </article>
        <article class="chart-card">
          <div class="chart-head">
            <h3>每日 Token</h3>
            <span class="muted small">
              输入 {{ formatNumber(usage?.totals.inputTokens ?? 0) }} / 输出
              {{ formatNumber(usage?.totals.outputTokens ?? 0) }}
            </span>
          </div>
          <UsageTrendChart :option="tokensOption" />
        </article>
        <article class="chart-card">
          <div class="chart-head">
            <h3>每日模型请求</h3>
            <span class="muted small">{{ formatNumber(usage?.totals.modelRequests ?? 0) }}</span>
          </div>
          <UsageTrendChart :option="requestsOption" />
        </article>
        <article class="chart-card">
          <div class="chart-head">
            <h3>每日上传</h3>
            <span class="muted small">
              {{ formatNumber(usage?.totals.uploadCount ?? 0) }} 个 /
              {{ formatNumber(usage?.totals.uploadBytes ?? 0) }} B
            </span>
          </div>
          <UsageTrendChart :option="uploadsOption" />
        </article>
      </div>
      <p v-if="usageLoading" class="muted small">正在加载使用趋势...</p>
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

<style scoped>
.usage-head {
  align-items: flex-start;
}

.usage-filters {
  display: flex;
  align-items: center;
  gap: 12px;
}

.range-switch {
  display: inline-flex;
  border: 1px solid var(--line);
  border-radius: 8px;
  overflow: hidden;
}

.range-switch button {
  padding: 7px 12px;
  background: var(--surface);
  border: none;
  border-right: 1px solid var(--line);
  color: var(--muted);
  cursor: pointer;
  font-size: 12px;
}

.range-switch button:last-child {
  border-right: none;
}

.range-switch button.active {
  background: var(--blue-soft);
  color: var(--blue);
}

.user-select {
  min-width: 180px;
}

.chart-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16px;
}

.chart-card {
  padding: 16px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 12px;
}

.chart-head {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 8px;
}

.chart-head h3 {
  margin: 0;
  color: var(--text);
  font-size: 14px;
  font-weight: 600;
}

@media (max-width: 1080px) {
  .chart-grid {
    grid-template-columns: 1fr;
  }

  .usage-filters {
    flex-wrap: wrap;
  }
}
</style>

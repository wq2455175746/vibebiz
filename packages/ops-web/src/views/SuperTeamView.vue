<script setup lang="ts">
import { computed, ref } from 'vue'

import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import {
  adminSuperTeamActions,
  adminSuperTeamPipelines,
  adminSuperTeamSources,
} from '@/mocks/superTeam'
import type { TableColumn, TableRow } from '@/types'
import {
  actionStatusLabel,
  agentActionLabel,
  formatDuration,
  scorePercent,
  sourceKindLabel,
  superStepLabel,
} from '@/utils/superTeam'

type TabKey = 'sources' | 'pipelines' | 'actions'

const tab = ref<TabKey>('sources')
const keyword = ref('')
const page = ref(1)
const pageSize = ref(10)

const tabs: { key: TabKey; label: string; count: number }[] = [
  { key: 'sources', label: '事件源', count: adminSuperTeamSources.length },
  { key: 'pipelines', label: '三步项目', count: adminSuperTeamPipelines.length },
  { key: 'actions', label: 'Agent 行为审计', count: adminSuperTeamActions.length },
]

const sourceColumns: TableColumn[] = [
  { key: 'owner', label: '归属用户', width: '120px' },
  {
    key: 'kind',
    label: '类型',
    width: '110px',
    format: (value) => sourceKindLabel(String(value)),
  },
  { key: 'status', label: '状态', width: '110px' },
  { key: 'enabled', label: '启用', width: '80px', format: (value) => (value ? '是' : '否') },
  { key: 'lastEventAt', label: '最近事件', width: '170px', format: (value) => formatDate(value) },
  { key: 'eventCount', label: '事件数', width: '90px' },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDate(value) },
]

const pipelineColumns: TableColumn[] = [
  { key: 'owner', label: '用户', width: '120px' },
  { key: 'workspace', label: '工作区', width: '140px' },
  { key: 'requirements', label: '需求挖掘', width: '100px' },
  { key: 'development', label: '系统开发', width: '100px' },
  { key: 'operations', label: '观测运维', width: '100px' },
  {
    key: 'currentStep',
    label: '当前步骤',
    width: '120px',
    format: (value) => superStepLabel(String(value)),
  },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDate(value) },
]

const actionColumns: TableColumn[] = [
  { key: 'eventId', label: '事件 ID', width: '120px' },
  {
    key: 'source',
    label: '来源',
    width: '100px',
    format: (value) => sourceKindLabel(String(value)),
  },
  {
    key: 'step',
    label: '步骤',
    width: '100px',
    format: (value) => superStepLabel(String(value)),
  },
  { key: 'agent', label: 'Agent', width: '120px' },
  {
    key: 'action',
    label: '动作',
    width: '110px',
    format: (value) => agentActionLabel(String(value)),
  },
  {
    key: 'noiseScore',
    label: '噪音分',
    width: '90px',
    format: (value) => scorePercent(Number(value)),
  },
  {
    key: 'semanticScore',
    label: '语义分',
    width: '90px',
    format: (value) => scorePercent(Number(value)),
  },
  {
    key: 'status',
    label: '状态',
    width: '90px',
    format: (value) => actionStatusLabel(String(value)),
  },
  {
    key: 'durationMs',
    label: '耗时',
    width: '90px',
    format: (value) => formatDuration(Number(value)),
  },
  { key: 'createdAt', label: '时间', width: '170px', format: (value) => formatDate(value) },
]

function formatDate(value: unknown): string {
  if (typeof value !== 'string' || !value) {
    return '—'
  }
  return new Date(value).toLocaleString('zh-CN')
}

const rows = computed<TableRow[]>(() => {
  if (tab.value === 'sources') {
    return adminSuperTeamSources
  }
  if (tab.value === 'pipelines') {
    return adminSuperTeamPipelines
  }
  return adminSuperTeamActions
})

const columns = computed<TableColumn[]>(() => {
  if (tab.value === 'sources') {
    return sourceColumns
  }
  if (tab.value === 'pipelines') {
    return pipelineColumns
  }
  return actionColumns
})

const filteredRows = computed<TableRow[]>(() => {
  const query = keyword.value.trim().toLowerCase()
  if (!query) {
    return rows.value
  }
  return rows.value.filter((row) => JSON.stringify(row).toLowerCase().includes(query))
})

const total = computed(() => filteredRows.value.length)
const pagedRows = computed<TableRow[]>(() =>
  filteredRows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value),
)

function switchTab(next: TabKey): void {
  tab.value = next
  keyword.value = ''
  page.value = 1
}

function search(): void {
  page.value = 1
}

function setPage(next: number): void {
  page.value = next
}

function setPageSize(next: number): void {
  pageSize.value = next
  page.value = 1
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>超级团队</h1>
        <p class="page-desc">只读视图；事件源由用户在 web 端配置。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <div class="super-tabs">
      <button
        v-for="item in tabs"
        :key="item.key"
        type="button"
        class="super-tab"
        :class="{ active: tab === item.key }"
        @click="switchTab(item.key)"
      >
        {{ item.label }} · {{ item.count }}
      </button>
    </div>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable :columns="columns" :rows="pagedRows" empty-text="暂无数据" />

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

<style scoped>
.super-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 14px;
}

.super-tab {
  padding: 8px 14px;
  color: var(--muted);
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 999px;
  cursor: pointer;
}

.super-tab.active {
  color: var(--blue);
  background: var(--blue-soft);
  border-color: rgba(37, 99, 235, 0.35);
}
</style>

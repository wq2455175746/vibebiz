<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'

import { getAccessLogs } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AccessLog, PageQuery, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const eventType = ref('')
const from = ref('')
const to = ref('')
const eventOptions = [
  { label: '全部事件', value: '' },
  { label: '登录', value: 'login' },
  { label: '请求', value: 'request' },
]

async function loader(query: PageQuery) {
  return getAccessLogs({
    ...query,
    eventType: eventType.value || undefined,
    from: from.value || undefined,
    to: to.value || undefined,
  })
}

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(loader)

const columns: TableColumn[] = [
  { key: 'eventAt', label: '时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'eventType', label: '类型', width: '80px' },
  { key: 'username', label: '用户', width: '120px' },
  { key: 'ip', label: 'IP', width: '130px' },
  { key: 'path', label: '方法 + 路径', width: '300px' },
  { key: 'statusCode', label: '状态', width: '70px' },
  { key: 'durationMs', label: '耗时', width: '80px', format: (value) => formatDuration(value) },
  { key: 'terminal', label: '终端', width: '160px' },
  { key: 'deviceId', label: '设备码', width: '160px' },
  { key: 'projectId', label: '项目', width: '120px' },
]

const eventLabels: Record<string, string> = { login: '登录', request: '请求' }

function terminal(row: unknown): string {
  const log = row as AccessLog
  return [log.os, log.browser, log.deviceType].filter(Boolean).join(' / ')
}

function eventLabel(row: unknown): string {
  const type = (row as AccessLog).eventType
  return eventLabels[type] ?? type
}

function formatDuration(value: unknown): string {
  if (value === null || value === undefined || value === '') {
    return '—'
  }
  return `${value} ms`
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>访问日志</h1>
        <p class="page-desc">web 用户登录与接口访问痕迹，先落本地日志再批量入库。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <div class="filters">
      <Select
        v-model="eventType"
        :options="eventOptions"
        option-label="label"
        option-value="value"
        placeholder="事件类型"
        aria-label="事件类型"
      />
      <InputText v-model="from" type="date" placeholder="开始日期" aria-label="开始日期" />
      <InputText v-model="to" type="date" placeholder="结束日期" aria-label="结束日期" />
      <Button label="筛选" icon="pi pi-filter" @click="search" />
    </div>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无访问日志"
      @retry="load"
    >
      <template #cell-eventType="{ row }">
        <span class="tag">{{ eventLabel(row) }}</span>
      </template>
      <template #cell-path="{ row }">
        {{ (row as AccessLog).method }} {{ (row as AccessLog).path }}
      </template>
      <template #cell-statusCode="{ value }">
        <span :class="{ 'status-fail': Number(value) >= 400 }">{{ value }}</span>
      </template>
      <template #cell-terminal="{ row }">
        {{ terminal(row) }}
      </template>
    </DataTable>

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
.filters {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.status-fail {
  color: var(--blue, #1d4ed8);
  font-weight: 600;
}
</style>

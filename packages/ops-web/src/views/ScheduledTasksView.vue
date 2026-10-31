<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'

import {
  getAdminScheduledTasks,
  pauseAdminScheduledTask,
  resumeAdminScheduledTask,
  stopAdminScheduledTask,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminScheduledTask, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getAdminScheduledTasks)

const actionError = ref('')
const columns: TableColumn[] = [
  { key: 'username', label: '用户', width: '120px' },
  { key: 'name', label: '任务名称' },
  { key: 'projectId', label: '项目', width: '180px' },
  { key: 'cronExpression', label: 'Cron', width: '140px' },
  { key: 'timezone', label: '时区', width: '150px' },
  { key: 'enabled', label: '状态', width: '90px' },
  { key: 'lastStatus', label: '上次状态', width: '100px' },
  { key: 'lastRunAt', label: '上次执行', width: '170px' },
  { key: 'runCount', label: '次数', width: '80px' },
  { key: 'actions', label: '操作', width: '180px' },
]

async function toggle(row: unknown) {
  const task = row as AdminScheduledTask
  actionError.value = ''
  try {
    if (task.enabled) {
      await pauseAdminScheduledTask(task.id)
    } else {
      await resumeAdminScheduledTask(task.id)
    }
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '操作失败'
  }
}

async function stop(row: unknown) {
  const task = row as AdminScheduledTask
  actionError.value = ''
  try {
    await stopAdminScheduledTask(task.id)
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '停止失败'
  }
}

function formatDate(value: unknown) {
  if (typeof value !== 'string' || !value) return '—'
  return new Date(value).toLocaleString('zh-CN')
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>定时任务</h1>
        <p class="page-desc">查看用户定时任务，可暂停、恢复或停止执行。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无定时任务"
      @retry="load"
    >
      <template #cell-enabled="{ row }">
        <span class="tag">{{ (row as AdminScheduledTask).enabled ? '启用' : '暂停' }}</span>
      </template>
      <template #cell-lastRunAt="{ row }">
        {{ formatDate((row as AdminScheduledTask).lastRunAt) }}
      </template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminScheduledTask).enabled ? '暂停' : '恢复'"
            size="small"
            text
            @click="toggle(row)"
          />
          <Button
            v-if="(row as AdminScheduledTask).lastStatus === 'running'"
            label="停止执行"
            size="small"
            text
            severity="danger"
            @click="stop(row)"
          />
        </div>
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

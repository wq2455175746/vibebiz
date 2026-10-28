<script setup lang="ts">
import { getAudit } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getAudit)

const columns: TableColumn[] = [
  { key: 'actor', label: '操作人', width: '150px' },
  { key: 'action', label: '动作', width: '180px' },
  { key: 'target', label: '目标', width: '150px' },
  { key: 'id', label: '事件 ID', width: '150px' },
  { key: 'createdAt', label: '时间', width: '170px', format: (value) => formatDateTime(value) },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>审计日志</h1>
        <p class="page-desc">控制面关键操作事件，只读。</p>
      </div>
      <span class="tag">{{ rows.length }} 条</span>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无审计事件"
      @retry="load"
    />
  </div>
</template>

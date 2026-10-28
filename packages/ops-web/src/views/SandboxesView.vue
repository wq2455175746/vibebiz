<script setup lang="ts">
import { getSandboxes } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getSandboxes)

const columns: TableColumn[] = [
  { key: 'id', label: '节点 ID', width: '150px' },
  { key: 'projectId', label: '项目 ID', width: '150px' },
  { key: 'node', label: '节点', width: '140px' },
  { key: 'state', label: '状态', width: '110px', tag: true, highlight: ['running'] },
  {
    key: 'startedAt',
    label: '启动时间',
    width: '170px',
    format: (value) => formatDateTime(value),
  },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>沙箱节点</h1>
        <p class="page-desc">本地沙箱节点的运行状态。</p>
      </div>
      <span class="tag">{{ rows.length }} 条</span>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无沙箱节点"
      @retry="load"
    />
  </div>
</template>

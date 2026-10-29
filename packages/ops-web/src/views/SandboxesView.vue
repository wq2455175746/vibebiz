<script setup lang="ts">
import { getSandboxes } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { AdminSandbox, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getSandboxes)

const columns: TableColumn[] = [
  { key: 'name', label: '名称', width: '160px' },
  {
    key: 'host',
    label: '地址',
    width: '190px',
    format: (_value, row) => {
      const item = row as AdminSandbox
      return `${item.host}:${item.port}`
    },
  },
  { key: 'status', label: '状态', width: '100px', tag: true, highlight: ['online'] },
  { key: 'healthStatus', label: '健康', width: '110px', tag: true, highlight: ['healthy'] },
  {
    key: 'activeRequests',
    label: '并发',
    width: '90px',
    format: (_value, row) => {
      const item = row as AdminSandbox
      return `${item.activeRequests}/${item.maxConcurrency}`
    },
  },
  {
    key: 'lastHeartbeatAt',
    label: '最后心跳',
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
        <p class="page-desc">sandbox 实例注册、健康状态与实时并发。</p>
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

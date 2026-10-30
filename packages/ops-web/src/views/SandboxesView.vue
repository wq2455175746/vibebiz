<script setup lang="ts">
import { getSandboxes } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminSandbox, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getSandboxes)

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
      <span class="tag">{{ total }} 条</span>
    </header>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无沙箱节点"
      @retry="load"
    />

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />
  </div>
</template>

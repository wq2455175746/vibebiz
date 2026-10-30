<script setup lang="ts">
import { getProjects } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getProjects)

const columns: TableColumn[] = [
  { key: 'id', label: '项目 ID', width: '150px' },
  { key: 'name', label: '名称' },
  { key: 'state', label: '状态', width: '110px', tag: true, highlight: ['ready', 'running'] },
  { key: 'owner', label: '所有者', width: '150px' },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>项目</h1>
        <p class="page-desc">CP 内存项目与运营附加字段。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无项目"
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

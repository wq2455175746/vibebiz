<script setup lang="ts">
import { getUsers } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getUsers)

const columns: TableColumn[] = [
  { key: 'name', label: '用户', width: '160px' },
  { key: 'email', label: '邮箱' },
  { key: 'plan', label: '套餐', width: '100px' },
  { key: 'status', label: '状态', width: '110px', tag: true, highlight: ['active'] },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>用户</h1>
        <p class="page-desc">MySQL 中的业务用户与套餐视图。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无用户"
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

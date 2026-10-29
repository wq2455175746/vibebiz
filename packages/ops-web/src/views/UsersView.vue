<script setup lang="ts">
import { getUsers } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getUsers)

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
      <span class="tag">{{ rows.length }} 条</span>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无用户"
      @retry="load"
    />
  </div>
</template>

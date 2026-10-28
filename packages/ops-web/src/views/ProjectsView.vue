<script setup lang="ts">
import { getProjects } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getProjects)

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
      <span class="tag">{{ rows.length }} 条</span>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无项目"
      @retry="load"
    />
  </div>
</template>

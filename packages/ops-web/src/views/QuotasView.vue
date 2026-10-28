<script setup lang="ts">
import { getQuotas } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatNumber } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getQuotas)

const columns: TableColumn[] = [
  { key: 'userId', label: '用户 ID', width: '150px' },
  { key: 'plan', label: '套餐', width: '110px' },
  {
    key: 'llmTokens',
    label: 'LLM Tokens',
    width: '140px',
    format: (value) => formatNumber(value),
  },
  {
    key: 'sandboxMinutes',
    label: '沙箱分钟',
    width: '120px',
    format: (value) => formatNumber(value),
  },
  {
    key: 'storageMb',
    label: '存储 (MB)',
    width: '120px',
    format: (value) => formatNumber(value),
  },
]
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>配额</h1>
        <p class="page-desc">用户套餐与资源用量快照。</p>
      </div>
      <span class="tag">{{ rows.length }} 条</span>
    </header>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无配额"
      @retry="load"
    />
  </div>
</template>

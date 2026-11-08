<script setup lang="ts">
import Button from 'primevue/button'
import { ref } from 'vue'

import { deleteCard, listCards, updateCardHidden } from '@/api/cards'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminCardSummary, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(listCards)

const columns: TableColumn[] = [
  { key: 'ownerName', label: '名片', width: '140px' },
  { key: 'user', label: '用户', width: '140px' },
  { key: 'tags', label: '标签', format: (value) => (Array.isArray(value) ? value.join(' / ') : '—') },
  { key: 'projectCount', label: '项目数', width: '90px' },
  { key: 'featuredType', label: '主预览', width: '100px' },
  { key: 'hidden', label: '状态', width: '100px' },
  { key: 'updatedAt', label: '更新时间', width: '180px' },
  { key: 'actions', label: '操作', width: '180px' },
]

const notice = ref('')
const actionError = ref('')
const busyId = ref('')

async function toggleHidden(row: unknown) {
  const card = row as AdminCardSummary
  busyId.value = card.id
  actionError.value = ''
  try {
    await updateCardHidden(card.id, !card.hidden)
    notice.value = card.hidden ? '已恢复名片' : '已隐藏名片'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '更新名片失败'
  } finally {
    busyId.value = ''
  }
}

async function remove(row: unknown) {
  const card = row as AdminCardSummary
  if (!window.confirm(`确定删除「${card.ownerName}」的名片吗？`)) return
  busyId.value = card.id
  actionError.value = ''
  try {
    await deleteCard(card.id)
    notice.value = '名片已删除'
    if (page.value > 1 && rows.value.length === 1) {
      await setPage(page.value - 1)
    } else {
      await load()
    }
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除名片失败'
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>名片管理</h1>
        <p class="page-desc">查看用户发布的名片，隐藏或删除违规内容。</p>
      </div>
      <span class="tag">{{ total }} 张</span>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>
    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无名片"
      @retry="load"
    >
      <template #cell-hidden="{ value }">{{ value ? '已隐藏' : '正常' }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminCardSummary).hidden ? '恢复' : '隐藏'"
            size="small"
            text
            :loading="busyId === (row as AdminCardSummary).id"
            @click="toggleHidden(row)"
          />
          <Button
            label="删除"
            size="small"
            text
            :loading="busyId === (row as AdminCardSummary).id"
            @click="remove(row)"
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

<style scoped>
.inline-notice {
  margin-bottom: 12px;
  color: var(--blue);
  font-size: 13px;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
}
</style>

<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'

import {
  deleteLeaderboardEntry,
  getLeaderboard,
  updateLeaderboardHidden,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminLeaderboardRow, TableColumn } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getLeaderboard)

const columns: TableColumn[] = [
  { key: 'rank', label: '排名', width: '70px' },
  { key: 'name', label: '项目名' },
  { key: 'author', label: '作者', width: '140px' },
  { key: 'visits', label: '访问数', width: '100px' },
  { key: 'exposure', label: '曝光量', width: '100px' },
  { key: 'clicks', label: '点击数', width: '100px' },
  { key: 'hidden', label: '状态', width: '100px' },
  { key: 'actions', label: '操作', width: '220px' },
]

const notice = ref('')
const actionError = ref('')
const busyId = ref('')

async function toggleHidden(row: unknown) {
  const item = row as AdminLeaderboardRow
  busyId.value = item.projectId
  actionError.value = ''
  try {
    await updateLeaderboardHidden(item.projectId, !item.hidden)
    notice.value = item.hidden ? '已取消隐藏' : '已隐藏'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '更新排行榜失败'
  } finally {
    busyId.value = ''
  }
}

async function remove(row: unknown) {
  const item = row as AdminLeaderboardRow
  if (!window.confirm(`确定删除「${item.name}」的榜单数据吗？已发布应用不会被删除。`)) {
    return
  }
  busyId.value = item.projectId
  actionError.value = ''
  try {
    await deleteLeaderboardEntry(item.projectId)
    notice.value = '榜单数据已删除'
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除榜单数据失败'
  } finally {
    busyId.value = ''
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>排行榜</h1>
        <p class="page-desc">查看已发布作品热度，隐藏或删除榜单条目。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>
    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无排行榜数据"
      @retry="load"
    >
      <template #cell-hidden="{ value }">{{ value ? '已隐藏' : '正常' }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            :label="(row as AdminLeaderboardRow).hidden ? '取消隐藏' : '隐藏'"
            size="small"
            text
            :loading="busyId === (row as AdminLeaderboardRow).projectId"
            @click="toggleHidden(row)"
          />
          <Button
            label="删除"
            size="small"
            text
            :loading="busyId === (row as AdminLeaderboardRow).projectId"
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

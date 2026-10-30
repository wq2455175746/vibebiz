<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import Select from 'primevue/select'

import { getUsers, updateUserStatus } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminUser, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getUsers)

const columns: TableColumn[] = [
  { key: 'name', label: '用户', width: '160px' },
  { key: 'email', label: '邮箱' },
  { key: 'plan', label: '套餐', width: '100px' },
  { key: 'status', label: '状态', width: '110px', tag: true, highlight: ['active'] },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '80px' },
]

const dialogVisible = ref(false)
const saving = ref(false)
const actionError = ref('')
const editingId = ref('')
const editingName = ref('')
const editingStatus = ref(1)

function openEdit(row: unknown) {
  const user = row as AdminUser
  editingId.value = user.id
  editingName.value = user.name
  editingStatus.value = user.status === 'active' ? 1 : 0
  actionError.value = ''
  dialogVisible.value = true
}

async function submit() {
  saving.value = true
  actionError.value = ''
  try {
    await updateUserStatus(editingId.value, editingStatus.value)
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}
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
    >
      <template #cell-actions="{ row }">
        <button class="link-button" type="button" @click="openEdit(row)">编辑</button>
      </template>
    </DataTable>

    <TablePagination
      :page="page"
      :page-size="pageSize"
      :total="total"
      @page="setPage"
      @page-size="setPageSize"
    />

    <Dialog
      v-model:visible="dialogVisible"
      header="编辑用户状态"
      modal
      :style="{ width: '420px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <p class="edit-hint">{{ editingName }}</p>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="editingStatus"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
          />
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>

      <template #footer>
        <Button label="取消" severity="secondary" text @click="dialogVisible = false" />
        <Button label="保存" :loading="saving" @click="submit" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  color: var(--text);
  font-size: 13px;
}

.field span {
  color: var(--muted);
  font-size: 12px;
}

.edit-hint {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.link-button {
  padding: 0;
  background: none;
  border: none;
  color: var(--blue);
  cursor: pointer;
  font-size: 13px;
}

.link-button:hover {
  text-decoration: underline;
}
</style>

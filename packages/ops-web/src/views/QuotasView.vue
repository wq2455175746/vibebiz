<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'

import { getQuotas, updateQuota } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { AdminQuota, QuotaUpdateInput, TableColumn } from '@/types'
import { formatNumber } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getQuotas)

function usage(used: number, quota: number): string {
  return `${formatNumber(used)} / ${formatNumber(quota)}`
}

const columns: TableColumn[] = [
  { key: 'userId', label: '用户 ID', width: '130px' },
  { key: 'plan', label: '套餐', width: '100px' },
  {
    key: 'llmTokensUsed',
    label: 'LLM Tokens（已用 / 额度）',
    width: '200px',
    format: (_value, row) => {
      const item = row as AdminQuota
      return usage(item.llmTokensUsed, item.llmTokensQuota)
    },
  },
  {
    key: 'sandboxMinutesUsed',
    label: '沙箱分钟（已用 / 额度）',
    width: '200px',
    format: (_value, row) => {
      const item = row as AdminQuota
      return usage(item.sandboxMinutesUsed, item.sandboxMinutesQuota)
    },
  },
  {
    key: 'storageUsedMb',
    label: '存储 MB（已用 / 额度）',
    width: '190px',
    format: (_value, row) => {
      const item = row as AdminQuota
      return usage(item.storageUsedMb, item.storageQuotaMb)
    },
  },
  { key: 'actions', label: '操作', width: '80px' },
]

const dialogVisible = ref(false)
const saving = ref(false)
const actionError = ref('')
const editingUserId = ref('')
const form = ref<QuotaUpdateInput>({
  llmTokensQuota: 0,
  sandboxMinutesQuota: 0,
  storageQuotaMb: 0,
})

function openEdit(row: unknown) {
  const quota = row as AdminQuota
  editingUserId.value = quota.userId
  form.value = {
    llmTokensQuota: quota.llmTokensQuota,
    sandboxMinutesQuota: quota.sandboxMinutesQuota,
    storageQuotaMb: quota.storageQuotaMb,
  }
  actionError.value = ''
  dialogVisible.value = true
}

function setNumber(field: keyof QuotaUpdateInput, value: number | null) {
  form.value[field] = Number(value ?? 0)
}

async function submit() {
  saving.value = true
  actionError.value = ''
  try {
    await updateQuota(editingUserId.value, form.value)
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
        <h1>配额</h1>
        <p class="page-desc">用户套餐与资源额度。</p>
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
    >
      <template #cell-actions="{ row }">
        <button class="link-button" type="button" @click="openEdit(row)">编辑</button>
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      header="编辑配额额度"
      modal
      :style="{ width: '460px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <p class="edit-hint">用户 ID：{{ editingUserId }}</p>
        <label class="field">
          <span>LLM Token 月额度</span>
          <InputNumber
            :model-value="form.llmTokensQuota"
            :min="0"
            :max="1000000000000"
            @update:model-value="setNumber('llmTokensQuota', $event)"
          />
        </label>
        <label class="field">
          <span>沙箱分钟月额度</span>
          <InputNumber
            :model-value="form.sandboxMinutesQuota"
            :min="0"
            :max="100000000"
            @update:model-value="setNumber('sandboxMinutesQuota', $event)"
          />
        </label>
        <label class="field">
          <span>存储额度（MB）</span>
          <InputNumber
            :model-value="form.storageQuotaMb"
            :min="0"
            :max="100000000"
            @update:model-value="setNumber('storageQuotaMb', $event)"
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

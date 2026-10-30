<script setup lang="ts">
import { ref } from 'vue'
import axios from 'axios'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import { createRole, getRoles, presignRoleAvatar, updateRole, uploadRoleAvatar } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  buildRoleInput,
  buildRoleUpdateInput,
  type RoleFormState,
  type RoleUpdateFormState,
} from '@/utils/roleForm'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getRoles)

const columns: TableColumn[] = [
  { key: 'avatarUrl', label: '头像', width: '70px' },
  { key: 'roleNameZh', label: '中文名', width: '130px' },
  { key: 'roleNameEn', label: '英文名', width: '110px' },
  { key: 'roleCode', label: '编码', width: '110px' },
  {
    key: 'isBuiltin',
    label: '来源',
    width: '80px',
    format: (value) => (value ? '内置' : '自定义'),
  },
  {
    key: 'status',
    label: '状态',
    width: '80px',
    format: (value) => (Number(value) === 1 ? '启用' : '禁用'),
  },
  { key: 'sortOrder', label: '排序', width: '70px' },
  { key: 'createdAt', label: '创建时间', width: '160px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '80px' },
]

const dialogVisible = ref(false)
const dialogMode = ref<'create' | 'edit'>('create')
const editingId = ref<number | null>(null)
const editingLabel = ref('')
const saving = ref(false)
const uploading = ref(false)
const actionError = ref('')
const notice = ref('')
const avatarPreview = ref('')
const avatarInput = ref<HTMLInputElement | null>(null)

function emptyForm(): RoleFormState {
  return {
    roleCode: '',
    roleNameEn: '',
    roleNameZh: '',
    description: '',
    systemPrompt: '',
    avatarKey: '',
    sortOrder: 0,
    status: 1,
  }
}

const form = ref<RoleFormState>(emptyForm())
const editForm = ref<RoleUpdateFormState>({ description: '', systemPrompt: '', status: 1 })

function openCreate() {
  dialogMode.value = 'create'
  editingId.value = null
  editingLabel.value = ''
  form.value = emptyForm()
  avatarPreview.value = ''
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

function openEdit(row: unknown) {
  const record = row as Record<string, unknown>
  dialogMode.value = 'edit'
  editingId.value = Number(record.id)
  editingLabel.value = String(record.roleNameZh ?? record.roleNameEn ?? '')
  editForm.value = {
    description: String(record.description ?? ''),
    systemPrompt: String(record.systemPrompt ?? ''),
    status: Number(record.status ?? 1),
  }
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

async function chooseAvatar(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) {
    return
  }
  if (!['image/jpeg', 'image/png', 'image/webp'].includes(file.type)) {
    actionError.value = '头像只支持 JPG、PNG、WebP'
    return
  }
  if (file.size <= 0 || file.size > 5 * 1024 * 1024) {
    actionError.value = '头像不能超过 5MB'
    return
  }
  uploading.value = true
  actionError.value = ''
  try {
    let key = ''
    try {
      const presign = await presignRoleAvatar({
        filename: file.name,
        contentType: file.type,
        size: file.size,
      })
      await axios.put(presign.uploadUrl, file, { headers: presign.headers })
      key = presign.key
    } catch {
      key = await uploadRoleAvatar(file)
    }
    form.value.avatarKey = key
    avatarPreview.value = URL.createObjectURL(file)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '头像上传失败'
  } finally {
    uploading.value = false
  }
}

function updateSortOrder(value: number | null) {
  form.value.sortOrder = Number(value ?? 0)
}

async function submit() {
  if (dialogMode.value === 'edit') {
    await submitEdit()
    return
  }
  const result = buildRoleInput(form.value)
  if ('error' in result) {
    actionError.value = result.error
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await createRole(result.input)
    notice.value = '角色已创建'
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function submitEdit() {
  if (editingId.value === null) {
    actionError.value = '缺少角色 ID'
    return
  }
  const result = buildRoleUpdateInput(editForm.value)
  if ('error' in result) {
    actionError.value = result.error
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await updateRole(editingId.value, result.input)
    notice.value = '角色已更新'
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

function avatarUrl(row: unknown): string {
  return String((row as Record<string, unknown>).avatarUrl ?? '')
}

function initial(row: unknown): string {
  const record = row as Record<string, unknown>
  return String(record.roleNameZh ?? record.roleNameEn ?? '?')
    .slice(0, 1)
    .toUpperCase()
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>角色管理</h1>
        <p class="page-desc">内置与自定义 Agent 角色、提示词和头像。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ total }} 条</span>
        <Button label="新增角色" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无角色"
      @retry="load"
    >
      <template #cell-avatarUrl="{ row }">
        <img v-if="avatarUrl(row)" class="role-avatar" :src="avatarUrl(row)" alt="" />
        <span v-else class="role-avatar role-avatar-fallback">{{ initial(row) }}</span>
      </template>
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
      :header="dialogMode === 'create' ? '新增角色' : '编辑角色'"
      modal
      :style="{ width: '680px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <p v-if="dialogMode === 'edit'" class="edit-hint">
          {{ editingLabel }}（仅可修改描述、System Prompt 与状态）
        </p>
        <div v-if="dialogMode === 'create'" class="avatar-field">
          <img
            v-if="avatarPreview"
            class="role-avatar role-avatar-large"
            :src="avatarPreview"
            alt=""
          />
          <span v-else class="role-avatar role-avatar-large role-avatar-fallback">
            {{ form.roleNameZh.slice(0, 1).toUpperCase() || '?' }}
          </span>
          <input
            ref="avatarInput"
            class="hidden-file"
            type="file"
            accept="image/jpeg,image/png,image/webp"
            @change="chooseAvatar"
          />
          <Button
            label="上传头像"
            severity="secondary"
            :loading="uploading"
            @click="avatarInput?.click()"
          />
        </div>
        <label v-if="dialogMode === 'create'" class="field">
          <span>角色编码</span>
          <InputText v-model="form.roleCode" maxlength="32" placeholder="例如 frank" />
        </label>
        <label v-if="dialogMode === 'create'" class="field">
          <span>中文名</span>
          <InputText v-model="form.roleNameZh" maxlength="64" />
        </label>
        <label v-if="dialogMode === 'create'" class="field">
          <span>英文名</span>
          <InputText v-model="form.roleNameEn" maxlength="64" />
        </label>
        <label class="field">
          <span>描述</span>
          <Textarea
            :model-value="dialogMode === 'create' ? form.description : editForm.description"
            rows="2"
            maxlength="1000"
            @update:model-value="
              (value) =>
                dialogMode === 'create'
                  ? (form.description = String(value ?? ''))
                  : (editForm.description = String(value ?? ''))
            "
          />
        </label>
        <label class="field">
          <span>System Prompt</span>
          <Textarea
            :model-value="dialogMode === 'create' ? form.systemPrompt : editForm.systemPrompt"
            rows="6"
            @update:model-value="
              (value) =>
                dialogMode === 'create'
                  ? (form.systemPrompt = String(value ?? ''))
                  : (editForm.systemPrompt = String(value ?? ''))
            "
          />
        </label>
        <label v-if="dialogMode === 'create'" class="field">
          <span>排序</span>
          <InputNumber
            :model-value="form.sortOrder"
            :min="0"
            :max="100000"
            @update:model-value="updateSortOrder"
          />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            :model-value="dialogMode === 'create' ? form.status : editForm.status"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
            @update:model-value="
              (value) =>
                dialogMode === 'create'
                  ? (form.status = Number(value))
                  : (editForm.status = Number(value))
            "
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
.inline-notice {
  margin-bottom: 16px;
  color: var(--blue);
  font-size: 13px;
}

.form {
  display: flex;
  flex-direction: column;
  gap: 16px;
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

.avatar-field {
  display: flex;
  align-items: center;
  gap: 12px;
}

.role-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  object-fit: cover;
}

.role-avatar-large {
  width: 56px;
  height: 56px;
}

.role-avatar-fallback {
  display: inline-grid;
  place-items: center;
  background: var(--soft);
  color: var(--muted);
  font-size: 13px;
  font-weight: 600;
}

.hidden-file {
  display: none;
}
</style>

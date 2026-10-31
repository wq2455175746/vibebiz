<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'

import { createFdeResource, getFdeResources, updateFdeResource } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { AdminFdeResource, FdeResourceInput, TableColumn, TableRow } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, loading, error, load } = useAdminList(getFdeResources)

const columns: TableColumn[] = [
  { key: 'name', label: '姓名', width: '120px' },
  { key: 'contact', label: '联系方式', width: '190px' },
  {
    key: 'skills',
    label: '技能',
    format: (value) => (Array.isArray(value) ? value.join('、') : '—'),
  },
  { key: 'region', label: '区域', width: '120px' },
  {
    key: 'enabled',
    label: '状态',
    width: '90px',
    tag: true,
    highlight: ['true'],
    format: (value) => (value ? '启用' : '停用'),
  },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '90px' },
]

const dialogVisible = ref(false)
const saving = ref(false)
const actionError = ref('')
const notice = ref('')
const editingId = ref<number | null>(null)
const form = ref<FdeResourceInput>({
  name: '',
  contact: '',
  skills: [],
  region: '',
  note: '',
  enabled: true,
})

function emptyForm(): FdeResourceInput {
  return { name: '', contact: '', skills: [], region: '', note: '', enabled: true }
}

function openCreate() {
  editingId.value = null
  form.value = emptyForm()
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

function openEdit(row: TableRow) {
  const item = row as AdminFdeResource
  editingId.value = item.id
  form.value = {
    name: item.name,
    contact: item.contact,
    skills: [...item.skills],
    region: item.region,
    note: item.note,
    enabled: item.enabled,
  }
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

function updateSkills(event: Event) {
  const value = (event.target as HTMLInputElement).value
  form.value.skills = value
    .split(/[,，、\n]/)
    .map((item) => item.trim())
    .filter(Boolean)
}

function skillsText(skills: string[]): string {
  return skills.join('、')
}

async function submit() {
  if (!form.value.name.trim() || !form.value.contact.trim()) {
    actionError.value = '姓名和联系方式不能为空'
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    if (editingId.value === null) {
      await createFdeResource(form.value)
      notice.value = 'FDE 资源已创建'
    } else {
      await updateFdeResource(editingId.value, form.value)
      notice.value = 'FDE 资源已更新'
    }
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
        <h1>FDE 资源</h1>
        <p class="page-desc">维护可被企业项目订购的 FDE 人力资源。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 人</span>
        <Button label="新增 FDE" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无 FDE 资源"
      @retry="load"
    >
      <template #cell-actions="{ row }">
        <Button label="编辑" size="small" text @click="openEdit(row)" />
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      :header="editingId === null ? '新增 FDE' : '编辑 FDE'"
      modal
      :style="{ width: '560px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>姓名</span>
          <InputText v-model="form.name" maxlength="64" />
        </label>
        <label class="field">
          <span>联系方式</span>
          <InputText v-model="form.contact" maxlength="128" placeholder="邮箱、电话或 IM 账号" />
        </label>
        <label class="field">
          <span>技能</span>
          <InputText
            :model-value="skillsText(form.skills)"
            placeholder="K8s、Java、交付实施"
            @input="updateSkills"
          />
          <small>用逗号、顿号或换行分隔。</small>
        </label>
        <label class="field">
          <span>区域</span>
          <InputText v-model="form.region" maxlength="64" placeholder="例如：上海" />
        </label>
        <label class="field">
          <span>备注</span>
          <Textarea v-model="form.note" rows="3" maxlength="500" />
        </label>
        <label class="field-inline">
          <Checkbox v-model="form.enabled" binary />
          <span>启用并允许用户订购</span>
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

.field small {
  color: var(--muted);
  font-size: 11px;
}

.field-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
}
</style>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import { createTopic, getAdminTopics, updateTopic } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import type { TableColumn, TopicInput, TopicNode } from '@/types'

interface TopicRow {
  id: number
  code: string
  name: string
  description: string
  promptTemplate: string
  parentId: number | null
  parentName: string
  sortOrder: number
  status: number
}

const topics = ref<TopicNode[]>([])
const loading = ref(false)
const error = ref('')
const notice = ref('')
const actionError = ref('')

const rows = computed<TopicRow[]>(() => {
  const out: TopicRow[] = []
  for (const parent of topics.value) {
    out.push({
      id: parent.id,
      code: parent.code,
      name: parent.name,
      description: parent.description,
      promptTemplate: parent.promptTemplate,
      parentId: null,
      parentName: '',
      sortOrder: parent.sortOrder,
      status: parent.status,
    })
    for (const child of parent.children) {
      out.push({
        id: child.id,
        code: child.code,
        name: child.name,
        description: child.description,
        promptTemplate: child.promptTemplate,
        parentId: parent.id,
        parentName: parent.name,
        sortOrder: child.sortOrder,
        status: child.status,
      })
    }
  }
  return out
})

const parentOptions = computed(() => topics.value)

const columns: TableColumn[] = [
  { key: 'name', label: '名称', width: '200px' },
  { key: 'code', label: '编码', width: '150px' },
  { key: 'parentName', label: '父类别', width: '130px' },
  { key: 'sortOrder', label: '排序', width: '80px' },
  {
    key: 'status',
    label: '状态',
    width: '80px',
    format: (value) => (Number(value) === 1 ? '启用' : '禁用'),
  },
  { key: 'actions', label: '操作', width: '180px' },
]

const dialogVisible = ref(false)
const saving = ref(false)
const editingId = ref<number | null>(null)
const form = ref<TopicInput>({
  parentId: null,
  code: '',
  name: '',
  description: '',
  promptTemplate: '',
  sortOrder: 0,
  status: 1,
})

async function load() {
  loading.value = true
  error.value = ''
  try {
    topics.value = await getAdminTopics()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载类别失败'
  } finally {
    loading.value = false
  }
}

function openCreate(parentId: number | null) {
  editingId.value = null
  form.value = {
    parentId,
    code: '',
    name: '',
    description: '',
    promptTemplate: '',
    sortOrder: 0,
    status: 1,
  }
  actionError.value = ''
  dialogVisible.value = true
}

function openEdit(row: unknown) {
  const item = row as TopicRow
  editingId.value = item.id
  form.value = {
    parentId: item.parentId,
    code: item.code,
    name: item.name,
    description: item.description,
    promptTemplate: item.promptTemplate,
    sortOrder: item.sortOrder,
    status: item.status,
  }
  actionError.value = ''
  dialogVisible.value = true
}

async function submit() {
  saving.value = true
  actionError.value = ''
  try {
    if (editingId.value === null) {
      await createTopic(form.value)
      notice.value = '类别已创建'
    } else {
      await updateTopic(editingId.value, form.value)
      notice.value = '类别已更新'
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>类别管理</h1>
        <p class="page-desc">维护模板的父类别与子类别。</p>
      </div>
      <Button label="新增父类别" icon="pi pi-plus" @click="openCreate(null)" />
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>
    <p v-if="actionError" class="error-text">{{ actionError }}</p>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无类别"
      @retry="load"
    >
      <template #cell-name="{ row }">
        <span :class="{ child: (row as TopicRow).parentId }">
          {{ (row as TopicRow).parentId ? '└ ' : '' }}{{ (row as TopicRow).name }}
        </span>
      </template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            v-if="!(row as TopicRow).parentId"
            label="新增子类"
            size="small"
            text
            @click="openCreate((row as TopicRow).id)"
          />
          <Button label="编辑" size="small" text @click="openEdit(row)" />
        </div>
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      :header="editingId === null ? '新增类别' : '编辑类别'"
      modal
      :style="{ width: '560px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>父类别</span>
          <Select
            :model-value="form.parentId"
            :options="parentOptions"
            option-label="name"
            option-value="id"
            show-clear
            placeholder="不选则为父类别"
            @update:model-value="form.parentId = $event ?? null"
          />
        </label>
        <label class="field">
          <span>编码</span>
          <InputText v-model="form.code" maxlength="32" placeholder="例如 game_action" />
        </label>
        <label class="field">
          <span>名称</span>
          <InputText v-model="form.name" maxlength="64" />
        </label>
        <label class="field">
          <span>描述</span>
          <Textarea v-model="form.description" rows="2" maxlength="1000" />
        </label>
        <label class="field">
          <span>提示词模板</span>
          <Textarea v-model="form.promptTemplate" rows="4" />
        </label>
        <label class="field">
          <span>排序</span>
          <InputNumber v-model="form.sortOrder" :min="0" :max="100000" />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="form.status"
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
.inline-notice {
  margin-bottom: 12px;
  color: var(--blue);
  font-size: 13px;
}

.row-actions {
  display: flex;
  gap: 2px;
}

.child {
  padding-left: 18px;
  color: var(--muted);
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
</style>

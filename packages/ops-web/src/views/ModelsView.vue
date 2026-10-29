<script setup lang="ts">
import { ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import { createModel, getModels } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import { useAdminList } from '@/composables/useAdminList'
import type { TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  buildModelInput,
  MODEL_PROVIDERS,
  MODEL_TYPES,
  type ModelFormState,
} from '@/utils/modelForm'

const { rows, loading, error, load } = useAdminList(getModels)

const columns: TableColumn[] = [
  { key: 'modelName', label: '模型名称', width: '160px' },
  { key: 'modelCode', label: '编码', width: '150px' },
  { key: 'modelType', label: '类型', width: '110px', tag: true, highlight: ['LLM'] },
  { key: 'providerCode', label: '供应商', width: '110px' },
  { key: 'apiUrl', label: 'API URL' },
  {
    key: 'maxTokens',
    label: 'Tokens',
    width: '90px',
    format: (value) => (Number(value) > 0 ? String(value) : '—'),
  },
  { key: 'isDefault', label: '默认', width: '70px', format: (value) => (value ? '是' : '否') },
  {
    key: 'status',
    label: '状态',
    width: '80px',
    format: (value) => (Number(value) === 1 ? '启用' : '禁用'),
  },
  { key: 'createdAt', label: '创建时间', width: '160px', format: (value) => formatDateTime(value) },
]

const dialogVisible = ref(false)
const saving = ref(false)
const actionError = ref('')
const notice = ref('')

function emptyForm(): ModelFormState {
  return {
    modelName: '',
    modelCode: '',
    apiUrl: '',
    apiKey: '',
    providerCode: 'Other',
    modelType: 'LLM',
    maxTokens: 4096,
    isDefault: false,
    status: 1,
    extraParams: '',
  }
}

const form = ref<ModelFormState>(emptyForm())

function openCreate() {
  form.value = emptyForm()
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

function updateMaxTokens(value: number | null) {
  form.value.maxTokens = Number(value ?? 0)
}

async function submit() {
  const result = buildModelInput(form.value)
  if ('error' in result) {
    actionError.value = result.error
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await createModel(result.input)
    notice.value = '模型已创建'
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
        <h1>模型管理</h1>
        <p class="page-desc">平台 LLM、Embedding、ASR、TTS 等模型目录。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 条</span>
        <Button label="新增模型" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无模型"
      @retry="load"
    />

    <Dialog
      v-model:visible="dialogVisible"
      header="新增模型"
      modal
      :style="{ width: '620px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>模型名称</span>
          <InputText v-model="form.modelName" maxlength="128" />
        </label>
        <label class="field">
          <span>模型编码</span>
          <InputText v-model="form.modelCode" maxlength="64" placeholder="例如 deepseek-chat" />
        </label>
        <label class="field">
          <span>模型类型</span>
          <Select
            v-model="form.modelType"
            :options="MODEL_TYPES"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>供应商</span>
          <Select
            v-model="form.providerCode"
            :options="MODEL_PROVIDERS"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>API URL</span>
          <InputText v-model="form.apiUrl" placeholder="https://" />
        </label>
        <label class="field">
          <span>API Key</span>
          <InputText v-model="form.apiKey" type="password" />
        </label>
        <label v-if="form.modelType === 'LLM'" class="field">
          <span>最大 tokens</span>
          <InputNumber
            :model-value="form.maxTokens"
            :min="1"
            :max="1000000"
            @update:model-value="updateMaxTokens"
          />
        </label>
        <label class="field">
          <span>extra_params（JSON 对象，可选）</span>
          <Textarea v-model="form.extraParams" rows="4" placeholder='{"voice":"alloy"}' />
        </label>
        <label class="field-inline">
          <Checkbox v-model="form.isDefault" binary />
          <span>设为该类型的默认模型</span>
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

.field-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
}
</style>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createChannelGuide,
  deleteChannelGuide,
  getChannelGuide,
  getChannelGuides,
  updateChannelGuide,
  uploadChannelGuideMarkdown,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import type {
  AdminChannelGuide,
  ChannelGuideCreateInput,
  ChannelGuideStatus,
  TableColumn,
  TableRow,
} from '@/types'
import { formatDateTime } from '@/utils/format'
import { isMarkdownFile, splitLines } from '@/utils/channelGuides'

interface GuideForm {
  channel: string
  title: string
  summary: string
  markdown: string
  stepsText: string
  requirementsText: string
  enabled: boolean
  sortOrder: number
}

const rows = ref<AdminChannelGuide[]>([])
const loading = ref(false)
const error = ref('')
const notice = ref('')
const keyword = ref('')
const status = ref<ChannelGuideStatus>('all')

const dialogVisible = ref(false)
const saving = ref(false)
const uploading = ref(false)
const actionError = ref('')
const editingChannel = ref('')
const form = ref<GuideForm>(emptyForm())
const fileInput = ref<HTMLInputElement | null>(null)

const statusOptions: { label: string; value: ChannelGuideStatus }[] = [
  { label: '全部状态', value: 'all' },
  { label: '已启用', value: 'enabled' },
  { label: '已停用', value: 'disabled' },
]

const columns: TableColumn[] = [
  { key: 'channel', label: '渠道标识', width: '180px' },
  { key: 'title', label: '名称', width: '140px' },
  {
    key: 'builtin',
    label: '类型',
    width: '80px',
    format: (value) => (value ? '内置' : '自定义'),
  },
  {
    key: 'enabled',
    label: '状态',
    width: '90px',
    tag: true,
    highlight: ['true'],
    format: (value) => (value ? '启用' : '停用'),
  },
  { key: 'sortOrder', label: '排序', width: '70px' },
  {
    key: 'markdownBytes',
    label: '正文大小',
    width: '100px',
    format: (value) => `${value ?? 0} B`,
  },
  { key: 'updatedAt', label: '更新时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '220px' },
]

const markdownBytes = computed(() => new TextEncoder().encode(form.value.markdown).length)

function emptyForm(): GuideForm {
  return {
    channel: '',
    title: '',
    summary: '',
    markdown: '',
    stepsText: '',
    requirementsText: '',
    enabled: true,
    sortOrder: 100,
  }
}

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    rows.value = await getChannelGuides({
      keyword: keyword.value.trim() || undefined,
      status: status.value,
    })
  } catch (err) {
    error.value = errorMessage(err, '加载发布渠道失败')
  } finally {
    loading.value = false
  }
}

function search() {
  void load()
}

function openCreate() {
  editingChannel.value = ''
  form.value = emptyForm()
  actionError.value = ''
  notice.value = ''
  dialogVisible.value = true
}

async function openEdit(row: TableRow) {
  const item = row as AdminChannelGuide
  actionError.value = ''
  notice.value = ''
  try {
    const detail = await getChannelGuide(item.channel)
    editingChannel.value = detail.channel
    form.value = {
      channel: detail.channel,
      title: detail.title,
      summary: detail.summary,
      markdown: detail.markdown,
      stepsText: detail.steps.join('\n'),
      requirementsText: detail.requirements.join('\n'),
      enabled: detail.enabled,
      sortOrder: detail.sortOrder,
    }
    dialogVisible.value = true
  } catch (err) {
    notice.value = errorMessage(err, '加载渠道详情失败')
  }
}

function payload() {
  return {
    title: form.value.title,
    summary: form.value.summary,
    markdown: form.value.markdown,
    steps: splitLines(form.value.stepsText),
    requirements: splitLines(form.value.requirementsText),
    enabled: form.value.enabled,
    sortOrder: form.value.sortOrder,
  }
}

async function submit() {
  if (!form.value.title.trim()) {
    actionError.value = '渠道名称不能为空'
    return
  }
  if (!editingChannel.value && !/^[a-z][a-z0-9_]{1,31}$/.test(form.value.channel.trim())) {
    actionError.value = '渠道标识需为小写字母开头，可包含小写字母、数字和下划线'
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    if (editingChannel.value) {
      await updateChannelGuide(editingChannel.value, payload())
      notice.value = '渠道已更新'
    } else {
      const input: ChannelGuideCreateInput = { channel: form.value.channel.trim(), ...payload() }
      await createChannelGuide(input)
      notice.value = '渠道已创建'
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = errorMessage(err, '保存失败')
  } finally {
    saving.value = false
  }
}

async function toggle(row: TableRow) {
  const item = row as AdminChannelGuide
  try {
    const detail = await getChannelGuide(item.channel)
    await updateChannelGuide(item.channel, {
      title: detail.title,
      summary: detail.summary,
      markdown: detail.markdown,
      steps: detail.steps,
      requirements: detail.requirements,
      enabled: !detail.enabled,
      sortOrder: detail.sortOrder,
    })
    await load()
  } catch (err) {
    notice.value = errorMessage(err, '切换状态失败')
  }
}

async function remove(row: TableRow) {
  const item = row as AdminChannelGuide
  if (!window.confirm(`确定删除渠道「${item.title}」吗？`)) {
    return
  }
  try {
    await deleteChannelGuide(item.channel)
    notice.value = '渠道已删除'
    await load()
  } catch (err) {
    notice.value = errorMessage(err, '删除失败')
  }
}

function chooseFile() {
  fileInput.value?.click()
}

async function onFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  if (!isMarkdownFile(file.name)) {
    actionError.value = '只支持 .md 或 .markdown 文件'
    return
  }
  actionError.value = ''
  if (editingChannel.value) {
    uploading.value = true
    try {
      const detail = await uploadChannelGuideMarkdown(editingChannel.value, file)
      form.value.markdown = detail.markdown
      notice.value = 'markdown 已上传并覆盖正文'
    } catch (err) {
      actionError.value = errorMessage(err, '上传失败')
    } finally {
      uploading.value = false
    }
    return
  }
  try {
    form.value.markdown = await file.text()
  } catch {
    actionError.value = '读取文件失败'
  }
}

onMounted(load)
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>发布渠道</h1>
        <p class="page-desc">管理前台展示的渠道信息、资质、步骤和 markdown 指引。</p>
      </div>
      <div class="page-side">
        <span class="tag">{{ rows.length }} 个</span>
        <Button label="新增渠道" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <div class="filters">
      <div class="search-row">
        <InputText v-model="keyword" placeholder="搜索标识、名称或简介" @keyup.enter="search" />
        <Select
          v-model="status"
          :options="statusOptions"
          option-label="label"
          option-value="value"
          aria-label="渠道状态"
          @change="search"
        />
        <Button label="搜索" icon="pi pi-search" @click="search" />
      </div>
    </div>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无发布渠道"
      @retry="load"
    >
      <template #cell-actions="{ row }">
        <Button label="编辑" size="small" text @click="openEdit(row)" />
        <Button
          :label="(row as AdminChannelGuide).enabled ? '停用' : '启用'"
          size="small"
          text
          @click="toggle(row)"
        />
        <Button
          v-if="!(row as AdminChannelGuide).builtin"
          label="删除"
          size="small"
          text
          @click="remove(row)"
        />
      </template>
    </DataTable>

    <Dialog
      v-model:visible="dialogVisible"
      :header="editingChannel ? '编辑渠道' : '新增渠道'"
      modal
      :style="{ width: '760px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>渠道标识</span>
          <InputText
            v-model="form.channel"
            :disabled="Boolean(editingChannel)"
            maxlength="32"
            placeholder="taptap_mobile"
          />
          <small>创建后不可修改；内置渠道不可删除。</small>
        </label>
        <div class="field-grid">
          <label class="field">
            <span>名称</span>
            <InputText v-model="form.title" maxlength="64" />
          </label>
          <label class="field">
            <span>排序</span>
            <InputNumber v-model="form.sortOrder" :min="0" :max="9999" />
          </label>
        </div>
        <label class="field">
          <span>简介</span>
          <InputText v-model="form.summary" maxlength="255" />
        </label>
        <div class="field-grid">
          <label class="field">
            <span>发布步骤（每行一条）</span>
            <Textarea v-model="form.stepsText" rows="5" />
          </label>
          <label class="field">
            <span>资质要求（每行一条）</span>
            <Textarea v-model="form.requirementsText" rows="5" />
          </label>
        </div>
        <label class="field">
          <span>指引 markdown（{{ markdownBytes }} B / 524288 B）</span>
          <Textarea v-model="form.markdown" rows="14" />
        </label>
        <div class="upload-row">
          <input
            ref="fileInput"
            type="file"
            accept=".md,.markdown,text/markdown,text/plain"
            hidden
            @change="onFileChange"
          />
          <Button
            label="上传 markdown"
            icon="pi pi-upload"
            severity="secondary"
            :loading="uploading"
            @click="chooseFile"
          />
          <small>编辑已有渠道时上传会立即覆盖数据库正文；新增渠道时先读入文本，保存后生效。</small>
        </div>
        <label class="field-inline">
          <Checkbox v-model="form.enabled" binary />
          <span>在前台设置页展示</span>
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

.filters {
  margin-bottom: 12px;
}

.search-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.search-row :deep(input) {
  width: 280px;
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

.field small,
.upload-row small {
  color: var(--muted);
  font-size: 11px;
}

.field-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.field-inline {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  font-size: 13px;
}

.upload-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}
</style>

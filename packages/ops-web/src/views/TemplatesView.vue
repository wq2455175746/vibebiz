<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  deleteTemplate,
  getAdminTemplates,
  getAdminTopics,
  getTemplateFiles,
  updateTemplate,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminTemplate, ProjectFile, TableColumn, TopicNode } from '@/types'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getAdminTemplates)

const columns: TableColumn[] = [
  { key: 'coverUrl', label: '封面', width: '90px' },
  { key: 'templateName', label: '模板名称' },
  { key: 'parentTopicName', label: '父类别', width: '110px' },
  { key: 'topicName', label: '子类别', width: '110px' },
  { key: 'recommendScore', label: '推荐分', width: '90px' },
  {
    key: 'status',
    label: '状态',
    width: '90px',
    format: (value) => (Number(value) === 1 ? '可用' : '上传中/禁用'),
  },
  { key: 'actions', label: '操作', width: '240px' },
]

const notice = ref('')
const actionError = ref('')
const topics = ref<TopicNode[]>([])

const editVisible = ref(false)
const editing = ref<AdminTemplate | null>(null)
const parentTopicId = ref<number | null>(null)
const childTopicId = ref<number | null>(null)
const editForm = ref({
  templateName: '',
  description: '',
  recommendScore: 80,
  status: 1,
})
const saving = ref(false)

const previewVisible = ref(false)
const previewTemplate = ref<AdminTemplate | null>(null)

const filesVisible = ref(false)
const files = ref<ProjectFile[]>([])
const filesLoading = ref(false)

const parentTopics = computed(() => topics.value)
const childTopics = computed(
  () => parentTopics.value.find((item) => item.id === parentTopicId.value)?.children ?? [],
)

async function ensureTopics() {
  if (topics.value.length > 0) return
  try {
    topics.value = await getAdminTopics()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '加载类别失败'
  }
}

async function openEdit(row: unknown) {
  const item = row as AdminTemplate
  await ensureTopics()
  editing.value = item
  parentTopicId.value = item.parentTopicId || topics.value[0]?.id || null
  childTopicId.value = item.topicId
  editForm.value = {
    templateName: item.templateName,
    description: item.description,
    recommendScore: item.recommendScore,
    status: item.status,
  }
  actionError.value = ''
  editVisible.value = true
}

function onParentChange() {
  childTopicId.value = childTopics.value[0]?.id ?? null
}

async function submitEdit() {
  if (!editing.value || !childTopicId.value) {
    actionError.value = '请选择子类别'
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    await updateTemplate(editing.value.id, {
      templateName: editForm.value.templateName,
      description: editForm.value.description,
      topicId: childTopicId.value,
      recommendScore: Number(editForm.value.recommendScore ?? 0),
      status: editForm.value.status,
    })
    notice.value = '模板已更新'
    editVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

function openPreview(row: unknown) {
  previewTemplate.value = row as AdminTemplate
  previewVisible.value = true
}

async function openFiles(row: unknown) {
  const item = row as AdminTemplate
  filesVisible.value = true
  filesLoading.value = true
  files.value = []
  try {
    files.value = await getTemplateFiles(item.id)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '读取文件失败'
  } finally {
    filesLoading.value = false
  }
}

async function remove(row: unknown) {
  const item = row as AdminTemplate
  if (!window.confirm(`确定删除模板「${item.templateName}」吗？`)) return
  actionError.value = ''
  try {
    await deleteTemplate(item.id)
    notice.value = '模板已删除'
    if (rows.value.length === 1 && page.value > 1) {
      await setPage(page.value - 1)
    } else {
      await load()
    }
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '删除失败'
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>模板管理</h1>
        <p class="page-desc">审核后的项目模板与推荐信息。</p>
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
      empty-text="暂无模板"
      @retry="load"
    >
      <template #cell-coverUrl="{ row }">
        <img
          v-if="(row as AdminTemplate).coverUrl"
          class="template-cover"
          :src="(row as AdminTemplate).coverUrl"
          alt=""
        />
        <span v-else class="template-cover template-cover-empty">无图</span>
      </template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button label="编辑" size="small" text @click="openEdit(row)" />
          <Button label="预览" size="small" text @click="openPreview(row)" />
          <Button label="文件" size="small" text @click="openFiles(row)" />
          <Button label="删除" size="small" text severity="danger" @click="remove(row)" />
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

    <Dialog
      v-model:visible="editVisible"
      header="编辑模板"
      modal
      :style="{ width: '560px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>模板名称</span>
          <InputText v-model="editForm.templateName" maxlength="128" />
        </label>
        <label class="field">
          <span>描述</span>
          <Textarea v-model="editForm.description" rows="3" maxlength="1000" />
        </label>
        <label class="field">
          <span>父类别</span>
          <Select
            v-model="parentTopicId"
            :options="parentTopics"
            option-label="name"
            option-value="id"
            @change="onParentChange"
          />
        </label>
        <label class="field">
          <span>子类别</span>
          <Select
            v-model="childTopicId"
            :options="childTopics"
            option-label="name"
            option-value="id"
          />
        </label>
        <label class="field">
          <span>推荐分数（0-100）</span>
          <InputNumber v-model="editForm.recommendScore" :min="0" :max="100" />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="editForm.status"
            :options="[
              { label: '可用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
          />
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="editVisible = false" />
        <Button label="保存" :loading="saving" @click="submitEdit" />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="previewVisible"
      :header="previewTemplate ? `预览：${previewTemplate.templateName}` : '模板预览'"
      modal
      :style="{ width: '760px' }"
    >
      <div v-if="previewTemplate" class="preview-body">
        <img
          v-if="previewTemplate.coverUrl"
          class="preview-image"
          :src="previewTemplate.coverUrl"
          alt=""
        />
        <div v-else class="preview-empty">暂无封面</div>
        <div class="preview-meta">
          <p>{{ previewTemplate.description || '暂无描述' }}</p>
          <p>
            类别：{{ previewTemplate.parentTopicName }} / {{ previewTemplate.topicName }} ·
            推荐分：{{ previewTemplate.recommendScore }}
          </p>
        </div>
      </div>
    </Dialog>

    <Dialog v-model:visible="filesVisible" header="模板文件" modal :style="{ width: '620px' }">
      <p v-if="filesLoading" class="file-empty">加载中...</p>
      <p v-else-if="files.length === 0" class="file-empty">暂无文件记录</p>
      <ul v-else class="file-list">
        <li v-for="file in files" :key="file.path">
          <i :class="file.type === 'dir' ? 'pi pi-folder' : 'pi pi-file'" />
          <span>{{ file.path }}</span>
        </li>
      </ul>
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
  flex-wrap: wrap;
  gap: 2px;
}

.template-cover {
  width: 64px;
  height: 40px;
  border: 1px solid var(--line);
  object-fit: cover;
}

.template-cover-empty {
  display: inline-grid;
  place-items: center;
  background: var(--soft);
  color: var(--muted);
  font-size: 11px;
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

.preview-image {
  width: 100%;
  max-height: 420px;
  object-fit: contain;
  background: var(--soft);
}

.preview-empty,
.file-empty {
  padding: 32px;
  color: var(--muted);
  text-align: center;
  font-size: 13px;
}

.preview-meta {
  padding-top: 12px;
  color: var(--muted);
  font-size: 13px;
}

.file-list {
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 12px;
}

.file-list li {
  display: flex;
  gap: 6px;
  padding: 5px 0;
  color: var(--text);
}

.file-list li i {
  color: var(--blue);
}
</style>

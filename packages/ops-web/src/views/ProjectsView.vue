<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputNumber from 'primevue/inputnumber'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createTemplate,
  getProjectFiles,
  getProjects,
  getTemplateTopics,
  projectPreviewURL,
  reviewProject,
} from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminProject, ProjectFile, TableColumn, TopicNode } from '@/types'
import { formatDateTime } from '@/utils/format'

const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getProjects)

const columns: TableColumn[] = [
  { key: 'coverUrl', label: '截图', width: '90px' },
  { key: 'name', label: '名称' },
  { key: 'state', label: '状态', width: '100px', tag: true, highlight: ['ready', 'running'] },
  { key: 'reviewStatus', label: '审核', width: '100px' },
  { key: 'owner', label: '所有者', width: '120px' },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '280px' },
]

const notice = ref('')
const actionError = ref('')

const previewVisible = ref(false)
const previewProject = ref<AdminProject | null>(null)
const files = ref<ProjectFile[]>([])
const filesLoading = ref(false)
let fileTimer: ReturnType<typeof setInterval> | null = null

const reviewVisible = ref(false)
const reviewTarget = ref<AdminProject | null>(null)
const reviewStatus = ref<'approved' | 'rejected'>('approved')
const reviewNote = ref('')
const reviewing = ref(false)

const templateVisible = ref(false)
const templateTarget = ref<AdminProject | null>(null)
const topics = ref<TopicNode[]>([])
const parentTopicId = ref<number | null>(null)
const childTopicId = ref<number | null>(null)
const templateForm = ref({
  templateName: '',
  description: '',
  recommendScore: 80,
})
const savingTemplate = ref(false)

const parentTopics = computed(() => topics.value)
const childTopics = computed(
  () => parentTopics.value.find((item) => item.id === parentTopicId.value)?.children ?? [],
)
const previewURL = computed(() =>
  previewProject.value ? projectPreviewURL(previewProject.value.id) : '',
)

function reviewLabel(value: string): string {
  if (value === 'approved') return '已通过'
  if (value === 'rejected') return '未通过'
  return '待审核'
}

function stateLabel(value: string): string {
  if (value === 'disabled') return '已禁用'
  return value
}

async function refreshFiles(id: string) {
  filesLoading.value = true
  try {
    files.value = await getProjectFiles(id)
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '读取文件目录失败'
  } finally {
    filesLoading.value = false
  }
}

function openPreview(row: unknown) {
  const project = row as AdminProject
  previewProject.value = project
  previewVisible.value = true
  actionError.value = ''
  void refreshFiles(project.id)
  if (fileTimer) clearInterval(fileTimer)
  fileTimer = setInterval(() => void refreshFiles(project.id), 3000)
}

function closePreview() {
  previewVisible.value = false
  if (fileTimer) {
    clearInterval(fileTimer)
    fileTimer = null
  }
}

function openReview(row: unknown) {
  const project = row as AdminProject
  reviewTarget.value = project
  reviewStatus.value = project.reviewStatus === 'rejected' ? 'rejected' : 'approved'
  reviewNote.value = project.reviewNote
  reviewVisible.value = true
  actionError.value = ''
}

async function submitReview() {
  if (!reviewTarget.value) return
  reviewing.value = true
  actionError.value = ''
  try {
    await reviewProject(reviewTarget.value.id, reviewStatus.value, reviewNote.value)
    notice.value = '审核结果已保存'
    reviewVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '审核失败'
  } finally {
    reviewing.value = false
  }
}

async function openTemplate(row: unknown) {
  const project = row as AdminProject
  templateTarget.value = project
  if (topics.value.length === 0) {
    try {
      topics.value = await getTemplateTopics()
    } catch (err) {
      actionError.value = err instanceof Error ? err.message : '读取类别失败'
      return
    }
  }
  parentTopicId.value = parentTopics.value[0]?.id ?? null
  childTopicId.value = childTopics.value[0]?.id ?? null
  templateForm.value = {
    templateName: project.name,
    description: '',
    recommendScore: 80,
  }
  templateVisible.value = true
  actionError.value = ''
}

function onParentChange() {
  childTopicId.value = childTopics.value[0]?.id ?? null
}

async function submitTemplate() {
  if (!templateTarget.value) return
  if (!childTopicId.value) {
    actionError.value = '请选择子类别'
    return
  }
  savingTemplate.value = true
  actionError.value = ''
  try {
    await createTemplate(templateTarget.value.id, {
      templateName: templateForm.value.templateName,
      description: templateForm.value.description,
      topicId: childTopicId.value,
      recommendScore: Number(templateForm.value.recommendScore ?? 0),
    })
    notice.value = '模板已创建，文件正在后台上传'
    templateVisible.value = false
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '创建模板失败'
  } finally {
    savingTemplate.value = false
  }
}

onUnmounted(() => {
  if (fileTimer) clearInterval(fileTimer)
})
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>项目</h1>
        <p class="page-desc">项目预览、审核与模板转换。</p>
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
      empty-text="暂无项目"
      @retry="load"
    >
      <template #cell-coverUrl="{ row }">
        <button
          v-if="(row as AdminProject).coverUrl"
          class="cover-button"
          type="button"
          @click="openPreview(row)"
        >
          <img class="project-cover" :src="(row as AdminProject).coverUrl" alt="" />
        </button>
        <span v-else class="project-cover project-cover-empty">无图</span>
      </template>
      <template #cell-state="{ value }">{{ stateLabel(String(value)) }}</template>
      <template #cell-reviewStatus="{ value }">{{ reviewLabel(String(value)) }}</template>
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button label="预览" size="small" text @click="openPreview(row)" />
          <Button label="文件" size="small" text @click="openPreview(row)" />
          <Button label="审核" size="small" text @click="openReview(row)" />
          <Button label="转模板" size="small" text @click="openTemplate(row)" />
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
      v-model:visible="previewVisible"
      :header="previewProject ? `预览：${previewProject.name}` : '项目预览'"
      modal
      :style="{ width: '1180px', maxWidth: '96vw' }"
      @hide="closePreview"
    >
      <div class="preview-layout">
        <iframe v-if="previewURL" class="preview-frame" :src="previewURL" title="项目预览" />
        <aside class="file-panel">
          <div class="file-panel-head">
            <span>文件目录</span>
            <Button
              icon="pi pi-refresh"
              size="small"
              text
              :loading="filesLoading"
              @click="previewProject && refreshFiles(previewProject.id)"
            />
          </div>
          <p v-if="filesLoading && files.length === 0" class="file-empty">加载中...</p>
          <p v-else-if="files.length === 0" class="file-empty">暂无文件</p>
          <ul v-else class="file-list">
            <li v-for="file in files" :key="file.path" :class="{ dir: file.type === 'dir' }">
              <i :class="file.type === 'dir' ? 'pi pi-folder' : 'pi pi-file'" />
              <span>{{ file.path }}</span>
            </li>
          </ul>
        </aside>
      </div>
    </Dialog>

    <Dialog
      v-model:visible="reviewVisible"
      header="项目审核"
      modal
      :style="{ width: '480px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <p class="hint">{{ reviewTarget?.name }}</p>
        <label class="field">
          <span>审核结果</span>
          <Select
            v-model="reviewStatus"
            :options="[
              { label: '通过', value: 'approved' },
              { label: '非法（禁用项目）', value: 'rejected' },
            ]"
            option-label="label"
            option-value="value"
          />
        </label>
        <label class="field">
          <span>备注</span>
          <Textarea v-model="reviewNote" rows="3" maxlength="1000" />
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="reviewVisible = false" />
        <Button label="保存" :loading="reviewing" @click="submitReview" />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="templateVisible"
      header="转换成模板"
      modal
      :style="{ width: '560px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>模板名称</span>
          <InputText v-model="templateForm.templateName" maxlength="128" />
        </label>
        <label class="field">
          <span>描述</span>
          <Textarea v-model="templateForm.description" rows="3" maxlength="1000" />
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
          <InputNumber v-model="templateForm.recommendScore" :min="0" :max="100" />
        </label>
        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="templateVisible = false" />
        <Button label="确认转换" :loading="savingTemplate" @click="submitTemplate" />
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
  flex-wrap: wrap;
  gap: 2px;
}

.cover-button {
  padding: 0;
  background: none;
  border: none;
  cursor: zoom-in;
}

.project-cover {
  width: 64px;
  height: 40px;
  border: 1px solid var(--line);
  object-fit: cover;
}

.project-cover-empty {
  display: inline-grid;
  place-items: center;
  background: var(--soft);
  color: var(--muted);
  font-size: 11px;
}

.preview-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 300px;
  gap: 16px;
  min-height: 560px;
}

.preview-frame {
  width: 100%;
  height: 560px;
  border: 1px solid var(--line);
  background: var(--surface);
}

.file-panel {
  overflow: auto;
  max-height: 560px;
  border: 1px solid var(--line);
  background: var(--surface);
}

.file-panel-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  border-bottom: 1px solid var(--line);
  font-size: 12px;
  color: var(--muted);
}

.file-list {
  margin: 0;
  padding: 8px 12px;
  list-style: none;
  font-size: 12px;
}

.file-list li {
  display: flex;
  gap: 6px;
  padding: 4px 0;
  color: var(--text);
  word-break: break-all;
}

.file-list li.dir {
  color: var(--blue);
}

.file-empty {
  padding: 24px;
  color: var(--muted);
  font-size: 12px;
  text-align: center;
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

.field span,
.hint {
  color: var(--muted);
  font-size: 12px;
}

.hint {
  margin: 0;
}
</style>

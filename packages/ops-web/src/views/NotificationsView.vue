<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import DatePicker from 'primevue/datepicker'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createNotification,
  deleteNotification,
  listNotifications,
  publishNotification,
  unpublishNotification,
  updateNotification,
} from '@/api/notifications'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import type {
  NotificationInput,
  NotificationStatus,
  NotificationType,
  OpsNotification,
  TableColumn,
  TableRow,
} from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  canEdit,
  canPublish,
  canUnpublish,
  notificationDateBounds,
  notificationStatusLabel,
  notificationTypeLabel,
  resolveExpireAt,
} from '@/utils/notificationStatus'

const rows = ref<OpsNotification[]>([])
const loading = ref(false)
const error = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const keyword = ref('')

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const result = await listNotifications({
      page: page.value,
      pageSize: pageSize.value,
      keyword: keyword.value.trim(),
    })
    rows.value = result.notifications
    total.value = result.total
    page.value = result.page
    pageSize.value = result.pageSize
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function search(): void {
  page.value = 1
  void load()
}

function setPage(next: number): void {
  page.value = next
  void load()
}

function setPageSize(next: number): void {
  pageSize.value = next
  page.value = 1
  void load()
}

onMounted(load)

const columns: TableColumn[] = [
  { key: 'title', label: '标题', width: '200px' },
  {
    key: 'type',
    label: '类型',
    width: '110px',
    format: (value) => notificationTypeLabel(value as NotificationType),
  },
  {
    key: 'status',
    label: '状态',
    width: '110px',
    tag: true,
    highlight: ['published'],
    format: (value) => notificationStatusLabel(value as NotificationStatus),
  },
  { key: 'version', label: '版本号', width: '110px' },
  { key: 'expireAt', label: '过期时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'createdByName', label: '创建人', width: '110px' },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '220px' },
]

const typeOptions = [
  { label: '版本发布', value: 'version_release' as NotificationType },
  { label: '收集箱', value: 'inbox' as NotificationType },
]

const dateBounds = ref(notificationDateBounds())
const dialogVisible = ref(false)
const editingId = ref<number | null>(null)
const saving = ref(false)
const busyId = ref<number | null>(null)
const actionError = ref('')
const notice = ref('')
const expireDate = ref<Date | null>(null)
const form = ref<{
  title: string
  content: string
  type: NotificationType
  version: string
  link: string
}>({
  title: '',
  content: '',
  type: 'version_release',
  version: '',
  link: '',
})

const dialogTitle = computed(() => (editingId.value === null ? '新建消息' : '编辑消息'))

function endOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate(), 23, 59, 59)
}

function defaultExpireDate(): Date {
  const date = new Date()
  date.setDate(date.getDate() + 7)
  return endOfDay(date)
}

function refreshDateBounds(): void {
  dateBounds.value = notificationDateBounds()
}

function resetForm(): void {
  form.value = { title: '', content: '', type: 'version_release', version: '', link: '' }
  expireDate.value = defaultExpireDate()
}

function clearFeedback(): void {
  actionError.value = ''
  notice.value = ''
}

function closeDialog(): void {
  clearFeedback()
  dialogVisible.value = false
}

function openCreate(): void {
  editingId.value = null
  clearFeedback()
  refreshDateBounds()
  resetForm()
  dialogVisible.value = true
}

function openEdit(item: OpsNotification): void {
  editingId.value = item.id
  clearFeedback()
  refreshDateBounds()
  form.value = {
    title: item.title,
    content: item.content,
    type: item.type,
    version: item.version,
    link: item.link,
  }
  const current = new Date(item.expireAt)
  expireDate.value = current.getTime() > Date.now() ? endOfDay(current) : defaultExpireDate()
  dialogVisible.value = true
}

function buildInput(): NotificationInput | null {
  if (form.value.title.trim() === '' || form.value.content.trim() === '') {
    actionError.value = '标题和内容不能为空'
    return null
  }
  if (!expireDate.value) {
    actionError.value = '请选择过期时间'
    return null
  }
  const expireAt = resolveExpireAt(expireDate.value)
  if (new Date(expireAt).getTime() <= Date.now()) {
    actionError.value = '过期时间必须晚于当前时间'
    return null
  }
  return {
    title: form.value.title.trim(),
    content: form.value.content,
    type: form.value.type,
    version: form.value.type === 'version_release' ? form.value.version.trim() : '',
    link: form.value.link.trim(),
    expireAt,
  }
}

async function submit(): Promise<void> {
  const input = buildInput()
  if (!input) {
    return
  }
  saving.value = true
  actionError.value = ''
  try {
    if (editingId.value === null) {
      await createNotification(input)
      notice.value = '消息已创建'
      page.value = 1
    } else {
      await updateNotification(editingId.value, input)
      notice.value = '消息已更新'
    }
    dialogVisible.value = false
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '保存失败'
  } finally {
    saving.value = false
  }
}

async function runAction(
  id: number,
  action: () => Promise<unknown>,
  message: string,
): Promise<void> {
  busyId.value = id
  actionError.value = ''
  notice.value = ''
  try {
    await action()
    notice.value = message
    if (rows.value.length === 1 && page.value > 1) {
      page.value -= 1
    }
    await load()
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '操作失败'
  } finally {
    busyId.value = null
  }
}

function publish(item: OpsNotification): void {
  void runAction(item.id, () => publishNotification(item.id), '消息已发布')
}

function unpublish(item: OpsNotification): void {
  void runAction(item.id, () => unpublishNotification(item.id), '已取消发布')
}

function remove(item: OpsNotification): void {
  if (!window.confirm(`确定删除「${item.title}」吗？`)) {
    return
  }
  void runAction(item.id, () => deleteNotification(item.id), '消息已删除')
}

function asNotification(row: TableRow): OpsNotification {
  return row as OpsNotification
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>消息通知</h1>
        <p class="page-desc">管理面向用户端的版本发布与收集箱通知。</p>
      </div>
      <div class="page-side">
        <Button label="刷新" icon="pi pi-refresh" severity="secondary" outlined @click="load" />
        <Button label="新建消息" icon="pi pi-plus" @click="openCreate" />
      </div>
    </header>

    <p v-if="actionError" class="inline-error error-text">{{ actionError }}</p>
    <p v-if="notice" class="inline-notice">{{ notice }}</p>

    <TableSearch v-model:keyword="keyword" @search="search" />

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无消息"
      @retry="load"
    >
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <Button
            label="编辑"
            size="small"
            text
            :disabled="!canEdit(asNotification(row)) || busyId === asNotification(row).id"
            @click="openEdit(asNotification(row))"
          />
          <Button
            v-if="canPublish(asNotification(row))"
            label="发布"
            size="small"
            text
            :loading="busyId === asNotification(row).id"
            @click="publish(asNotification(row))"
          />
          <Button
            v-if="canUnpublish(asNotification(row))"
            label="取消发布"
            size="small"
            text
            :loading="busyId === asNotification(row).id"
            @click="unpublish(asNotification(row))"
          />
          <Button
            label="删除"
            size="small"
            text
            severity="secondary"
            :disabled="busyId === asNotification(row).id"
            @click="remove(asNotification(row))"
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

    <Dialog
      v-model:visible="dialogVisible"
      :header="dialogTitle"
      modal
      :style="{ width: '560px' }"
      @hide="actionError = ''"
    >
      <div class="form">
        <label class="field">
          <span>标题</span>
          <InputText v-model="form.title" maxlength="200" />
        </label>

        <label class="field">
          <span>类型</span>
          <Select
            v-model="form.type"
            :options="typeOptions"
            option-label="label"
            option-value="value"
          />
        </label>

        <label v-if="form.type === 'version_release'" class="field">
          <span>版本号</span>
          <InputText v-model="form.version" maxlength="64" placeholder="例如 1.2.0" />
        </label>

        <label class="field">
          <span>链接</span>
          <InputText v-model="form.link" placeholder="https://" />
        </label>

        <label class="field">
          <span>内容（Markdown）</span>
          <Textarea v-model="form.content" rows="6" auto-resize />
        </label>

        <label class="field">
          <span>过期日期</span>
          <DatePicker
            v-model="expireDate"
            date-format="yy-mm-dd"
            :show-icon="true"
            :manual-input="false"
            :min-date="dateBounds.minDate"
            :max-date="dateBounds.maxDate"
          />
        </label>

        <p v-if="actionError" class="error-text">{{ actionError }}</p>
      </div>

      <template #footer>
        <Button label="取消" severity="secondary" text @click="closeDialog" />
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

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
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

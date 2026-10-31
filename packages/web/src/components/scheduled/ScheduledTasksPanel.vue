<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'

import {
  createScheduledTask,
  deleteScheduledTask,
  listScheduledTasks,
  parseScheduledTask,
  pauseScheduledTask,
  resumeScheduledTask,
  stopScheduledTask,
  updateScheduledTask,
} from '@/api/scheduledTasks'
import type { ScheduledTask, ScheduledTaskDraft, ScheduledTaskInput } from '@/types'

const router = useRouter()
const tasks = ref<ScheduledTask[]>([])
const selectedId = ref(0)
const loading = ref(false)
const error = ref('')
const notice = ref('')
const createMenuOpen = ref(false)
const dialogOpen = ref(false)
const mode = ref<'ai' | 'manual'>('ai')
const prompt = ref('')
const draft = ref<ScheduledTaskDraft | null>(null)
const editing = ref<ScheduledTask | null>(null)
const deleteTarget = ref<ScheduledTask | null>(null)
const deleteProject = ref(false)

const timezones = ['Asia/Shanghai', 'Asia/Tokyo', 'UTC', 'America/Los_Angeles']
const weekdayOptions = [
  { label: '周日', value: 0 },
  { label: '周一', value: 1 },
  { label: '周二', value: 2 },
  { label: '周三', value: 3 },
  { label: '周四', value: 4 },
  { label: '周五', value: 5 },
  { label: '周六', value: 6 },
]
const weekdayNames = ['周日', '周一', '周二', '周三', '周四', '周五', '周六']

const selectedTask = computed(
  () => tasks.value.find((task) => task.id === selectedId.value) ?? null,
)

function emptyForm(): ScheduledTaskInput {
  return {
    name: '',
    description: '',
    scheduleType: 'daily',
    time: '09:00',
    weekdays: [],
    cronExpression: '0 9 * * *',
    timezone: 'Asia/Shanghai',
  }
}

const form = ref<ScheduledTaskInput>(emptyForm())

async function load() {
  loading.value = true
  error.value = ''
  try {
    tasks.value = await listScheduledTasks()
    if (!tasks.value.some((task) => task.id === selectedId.value)) {
      selectedId.value = tasks.value[0]?.id ?? 0
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

function openCreateMenu() {
  createMenuOpen.value = true
}

function chooseCreate(nextMode: 'ai' | 'manual') {
  createMenuOpen.value = false
  openCreate(nextMode)
}

function openCreate(nextMode: 'ai' | 'manual') {
  mode.value = nextMode
  editing.value = null
  draft.value = null
  prompt.value = ''
  form.value = emptyForm()
  dialogOpen.value = true
}

function openEdit(task: ScheduledTask) {
  mode.value = 'manual'
  editing.value = task
  draft.value = null
  form.value = {
    name: task.name,
    description: task.description,
    scheduleType: task.scheduleType,
    time: task.time || '09:00',
    weekdays: [...task.weekdays],
    cronExpression: task.cronExpression,
    timezone: task.timezone,
  }
  dialogOpen.value = true
}

async function generateDraft() {
  error.value = ''
  try {
    draft.value = await parseScheduledTask(prompt.value)
    form.value = { ...draft.value, source: 'ai' }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '解析失败，请改用手动设置'
  }
}

function switchToManual() {
  mode.value = 'manual'
  draft.value = null
  form.value = { ...emptyForm(), description: prompt.value }
}

async function save() {
  error.value = ''
  try {
    let saved: ScheduledTask
    if (editing.value) {
      saved = await updateScheduledTask(editing.value.id, form.value)
    } else {
      saved = await createScheduledTask(form.value)
    }
    dialogOpen.value = false
    notice.value = '任务已保存'
    selectedId.value = saved.id
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '保存失败'
  }
}

async function toggle(task: ScheduledTask) {
  error.value = ''
  try {
    if (task.enabled) {
      await pauseScheduledTask(task.id)
    } else {
      await resumeScheduledTask(task.id)
    }
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '操作失败'
  }
}

async function stop(task: ScheduledTask) {
  error.value = ''
  try {
    await stopScheduledTask(task.id)
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '停止失败'
  }
}

function requestDelete(task: ScheduledTask) {
  deleteTarget.value = task
  deleteProject.value = false
}

async function confirmDelete() {
  if (!deleteTarget.value) return
  error.value = ''
  try {
    await deleteScheduledTask(deleteTarget.value.id, deleteProject.value)
    deleteTarget.value = null
    selectedId.value = 0
    await load()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除失败'
  }
}

function openProject(task: ScheduledTask) {
  if (task.projectId) {
    void router.push({ name: 'editor', params: { id: task.projectId } })
  }
}

function scheduleLabel(task: ScheduledTask) {
  if (task.scheduleType === 'daily') {
    return `每日 ${task.time || '09:00'}`
  }
  if (task.scheduleType === 'weekly') {
    const days = task.weekdays
      .map((day) => weekdayNames[day] ?? '')
      .filter(Boolean)
      .join('、')
    return `每周 ${days || '—'} ${task.time || ''}`.trim()
  }
  return `Cron ${task.cronExpression}`
}

function formatDate(value?: string) {
  return value ? new Date(value).toLocaleString('zh-CN') : '—'
}

onMounted(load)
</script>

<template>
  <section class="scheduled-panel">
    <header class="scheduled-head">
      <div>
        <h1>定时任务</h1>
        <p>让 Vibebiz 安排任务、设置提醒，或监控更新。</p>
      </div>
      <Button v-if="tasks.length > 0" label="创建任务" @click="openCreateMenu" />
    </header>

    <p v-if="error" class="error-text">{{ error }}</p>
    <p v-if="notice" class="notice-text">{{ notice }}</p>

    <p v-if="loading" class="discovery-empty">加载中...</p>

    <div v-else-if="tasks.length === 0" class="scheduled-empty">
      <div class="scheduled-empty-inner">
        <p>让 Vibebiz 安排任务、设置提醒，或监控更新。</p>
        <Button label="创建定时任务" @click="openCreateMenu" />
      </div>
    </div>

    <div v-else class="scheduled-layout">
      <aside class="task-list-panel">
        <div class="task-list-head">
          <strong>任务</strong>
          <span>{{ tasks.length }}</span>
        </div>
        <button
          v-for="task in tasks"
          :key="task.id"
          type="button"
          class="task-list-item"
          :class="{ active: task.id === selectedId }"
          @click="selectedId = task.id"
        >
          <strong>{{ task.name }}</strong>
          <span>{{ scheduleLabel(task) }}</span>
          <span class="task-list-status">
            {{ task.enabled ? '启用' : '暂停' }} · {{ task.lastStatus || '未执行' }}
          </span>
        </button>
      </aside>

      <article v-if="selectedTask" class="task-detail-panel">
        <header class="task-detail-head">
          <div>
            <h2>{{ selectedTask.name }}</h2>
            <p>{{ scheduleLabel(selectedTask) }} · {{ selectedTask.timezone }}</p>
          </div>
          <span class="tag">{{ selectedTask.enabled ? '启用' : '暂停' }}</span>
        </header>

        <div class="task-detail-actions">
          <Button
            v-if="selectedTask.projectId"
            label="打开项目"
            text
            size="small"
            @click="openProject(selectedTask)"
          />
          <Button label="编辑" text size="small" @click="openEdit(selectedTask)" />
          <Button
            :label="selectedTask.enabled ? '暂停' : '恢复'"
            text
            size="small"
            @click="toggle(selectedTask)"
          />
          <Button
            v-if="selectedTask.lastStatus === 'running'"
            label="停止执行"
            text
            size="small"
            @click="stop(selectedTask)"
          />
          <Button label="删除" text size="small" @click="requestDelete(selectedTask)" />
        </div>

        <div class="task-detail-grid">
          <div class="full">
            <span>任务描述</span>
            <p>{{ selectedTask.description }}</p>
          </div>
          <div>
            <span>Cron</span>
            <p>{{ selectedTask.cronExpression }}</p>
          </div>
          <div>
            <span>时区</span>
            <p>{{ selectedTask.timezone }}</p>
          </div>
          <div>
            <span>下次执行</span>
            <p>{{ formatDate(selectedTask.nextRunAt) }}</p>
          </div>
          <div>
            <span>上次执行</span>
            <p>{{ formatDate(selectedTask.lastRunAt) }}</p>
          </div>
          <div>
            <span>执行次数</span>
            <p>{{ selectedTask.runCount }}</p>
          </div>
          <div>
            <span>上次状态</span>
            <p>{{ selectedTask.lastStatus || '未执行' }}</p>
          </div>
          <div v-if="selectedTask.lastError" class="full">
            <span>最近错误</span>
            <p class="error-text">{{ selectedTask.lastError }}</p>
          </div>
        </div>
      </article>
    </div>

    <Dialog
      v-model:visible="createMenuOpen"
      header="创建定时任务"
      modal
      :style="{ width: '480px' }"
    >
      <div class="create-methods">
        <button type="button" class="create-method" @click="chooseCreate('ai')">
          <i class="pi pi-sparkles" />
          <span>
            <strong>使用 Vibebiz 创建</strong>
            <small>描述你想要的结果</small>
          </span>
        </button>
        <button type="button" class="create-method" @click="chooseCreate('manual')">
          <i class="pi pi-pencil" />
          <span>
            <strong>手动设置</strong>
            <small>配置任务和日程</small>
          </span>
        </button>
      </div>
    </Dialog>

    <Dialog v-model:visible="dialogOpen" header="定时任务" modal :style="{ width: '640px' }">
      <div v-if="mode === 'ai' && !draft" class="task-ai">
        <Textarea
          v-model="prompt"
          rows="5"
          placeholder="例如：每天早上 9 点检查项目依赖有没有更新"
        />
        <div class="task-create">
          <Button label="生成任务草稿" @click="generateDraft" />
          <Button label="改用手动设置" text @click="switchToManual" />
        </div>
      </div>
      <div v-else class="task-form">
        <label>任务名称<InputText v-model="form.name" /></label>
        <label>任务描述<Textarea v-model="form.description" rows="4" /></label>
        <label
          >频率
          <Select v-model="form.scheduleType" :options="['daily', 'weekly', 'cron']" />
        </label>
        <label v-if="form.scheduleType !== 'cron'"
          >时间<InputText v-model="form.time" placeholder="09:00" />
        </label>
        <label v-if="form.scheduleType === 'weekly'"
          >星期
          <Select
            v-model="form.weekdays"
            :options="weekdayOptions"
            option-label="label"
            option-value="value"
            multiple
          />
        </label>
        <label v-if="form.scheduleType === 'cron'"
          >Cron<InputText v-model="form.cronExpression" placeholder="0 9 * * *" />
        </label>
        <label>时区<Select v-model="form.timezone" :options="timezones" /></label>
      </div>
      <template #footer>
        <Button label="取消" text @click="dialogOpen = false" />
        <Button label="保存" @click="save" />
      </template>
    </Dialog>

    <Dialog
      :visible="Boolean(deleteTarget)"
      header="删除定时任务"
      modal
      @update:visible="deleteTarget = null"
    >
      <p>确定删除「{{ deleteTarget?.name }}」？</p>
      <label class="task-check"><Checkbox v-model="deleteProject" binary /> 同时删除绑定项目</label>
      <template #footer>
        <Button label="取消" text @click="deleteTarget = null" />
        <Button label="删除" @click="confirmDelete" />
      </template>
    </Dialog>
  </section>
</template>

<style scoped>
.scheduled-panel {
  display: flex;
  width: 100%;
  min-height: 100%;
  flex-direction: column;
  padding: 24px 32px 32px;
}

.scheduled-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.scheduled-head h1 {
  margin: 0;
  font-size: 24px;
}

.scheduled-head p {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.scheduled-empty {
  display: grid;
  min-height: 0;
  flex: 1;
  place-items: center;
  padding: 48px 0;
}

.scheduled-empty-inner {
  display: grid;
  justify-items: center;
  gap: 16px;
  color: var(--muted);
  text-align: center;
}

.scheduled-empty-inner p {
  margin: 0;
}

.scheduled-layout {
  display: grid;
  min-height: 0;
  flex: 1;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: 20px;
  margin-top: 20px;
}

.task-list-panel {
  min-height: 0;
  overflow-y: auto;
  padding: 8px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.task-list-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 10px 12px;
  color: var(--muted);
  font-size: 12px;
}

.task-list-item {
  display: grid;
  width: 100%;
  gap: 5px;
  padding: 12px;
  color: var(--text);
  text-align: left;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 12px;
  cursor: pointer;
}

.task-list-item:hover {
  background: var(--soft);
}

.task-list-item.active {
  background: var(--soft);
  border-color: var(--line-strong);
}

.task-list-item strong {
  overflow: hidden;
  font-size: 13px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-list-item span {
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-list-status {
  color: var(--ink) !important;
}

.task-detail-panel {
  min-height: 0;
  overflow-y: auto;
  padding: 24px;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.task-detail-head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
}

.task-detail-head h2 {
  margin: 0;
  font-size: 20px;
}

.task-detail-head p {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 13px;
}

.task-detail-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 18px;
  padding-bottom: 18px;
  border-bottom: 1px solid var(--line);
}

.task-detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 18px 24px;
  margin-top: 22px;
}

.task-detail-grid .full {
  grid-column: 1 / -1;
}

.task-detail-grid span {
  color: var(--muted);
  font-size: 12px;
}

.task-detail-grid p {
  margin: 6px 0 0;
  font-size: 13px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.create-methods {
  display: grid;
  gap: 12px;
}

.create-method {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 16px;
  color: var(--text);
  text-align: left;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 14px;
  cursor: pointer;
}

.create-method:hover {
  background: var(--soft);
  border-color: var(--line-strong);
}

.create-method i {
  color: var(--blue);
  font-size: 18px;
}

.create-method span {
  display: grid;
  gap: 4px;
}

.create-method small {
  color: var(--muted);
  font-size: 12px;
}

.task-create {
  display: flex;
  align-items: center;
  gap: 10px;
}

.task-ai,
.task-form {
  display: grid;
  gap: 14px;
}

.task-form label {
  display: grid;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.task-check {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  font-size: 13px;
}

.notice-text {
  color: var(--blue);
  font-size: 12px;
}

@media (max-width: 900px) {
  .scheduled-panel {
    padding: 20px 16px 24px;
  }

  .scheduled-layout {
    grid-template-columns: minmax(0, 1fr);
  }

  .task-list-panel {
    max-height: 240px;
  }
}
</style>

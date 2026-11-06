<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Button from 'primevue/button'
import FileUpload from 'primevue/fileupload'
import type { FileUploadSelectEvent } from 'primevue/fileupload'
import InputText from 'primevue/inputtext'
import Select from 'primevue/select'
import { useToast } from 'primevue/usetoast'

import { deleteSkill, getSkills, getUsers, uploadSkill } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import type { AdminSkill, AdminUser, SkillScope, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import { validateSkillScope } from '@/utils/skillAdmin'

const toast = useToast()
const scope = ref<SkillScope>('public')
const users = ref<AdminUser[]>([])
const userId = ref<number | null>(null)
const keyword = ref('')
const skills = ref<AdminSkill[]>([])
const loading = ref(false)
const error = ref('')
const selectedFile = ref<File | null>(null)
const overwrite = ref(false)
const uploading = ref(false)
const deleting = ref('')
const fileUpload = ref<{ clear: () => void } | null>(null)

const userOptions = computed(() =>
  users.value.map((user) => ({ label: `${user.name} (${user.id})`, value: Number(user.id) })),
)

const columns = computed<TableColumn[]>(() => [
  { key: 'name', label: '技能' },
  { key: 'description', label: '描述' },
  { key: 'source', label: '范围', width: '100px' },
  {
    key: 'updatedAt',
    label: '更新时间',
    width: '180px',
    format: (value) => (value ? formatDateTime(value) : '—'),
  },
  { key: 'actions', label: '操作', width: '100px' },
])

function notifyError(err: unknown, fallback: string) {
  toast.add({
    severity: 'error',
    summary: err instanceof Error ? err.message : fallback,
    life: 5000,
  })
}

async function loadUsers() {
  const result = await getUsers({ page: 1, pageSize: 100 })
  users.value = result.items
  if (userId.value === null && users.value.length > 0) {
    userId.value = Number(users.value[0].id)
  }
}

async function load() {
  const message = validateSkillScope(scope.value, userId.value)
  if (message) {
    skills.value = []
    error.value = message
    return
  }
  loading.value = true
  error.value = ''
  try {
    skills.value = await getSkills({
      scope: scope.value,
      userId: scope.value === 'user' ? (userId.value ?? undefined) : undefined,
      keyword: keyword.value.trim() || undefined,
    })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载技能失败'
  } finally {
    loading.value = false
  }
}

function switchScope(value: SkillScope) {
  scope.value = value
  keyword.value = ''
  void load()
}

function chooseFile(event: FileUploadSelectEvent) {
  selectedFile.value = (event.files[0] as File | undefined) ?? null
}

async function upload() {
  const message = validateSkillScope(scope.value, userId.value)
  if (message) {
    notifyError(new Error(message), message)
    return
  }
  if (!selectedFile.value) {
    notifyError(new Error('请选择 ZIP 技能包'), '请选择 ZIP 技能包')
    return
  }
  uploading.value = true
  try {
    await uploadSkill({
      scope: scope.value,
      userId: scope.value === 'user' ? (userId.value ?? undefined) : undefined,
      file: selectedFile.value,
      overwrite: overwrite.value,
    })
    toast.add({ severity: 'success', summary: '技能已上传', life: 3000 })
    selectedFile.value = null
    overwrite.value = false
    fileUpload.value?.clear()
    await load()
  } catch (err) {
    notifyError(err, '上传技能失败')
  } finally {
    uploading.value = false
  }
}

async function remove(skill: AdminSkill) {
  if (!window.confirm(`确定删除技能「${skill.name}」吗？`)) {
    return
  }
  deleting.value = skill.name
  try {
    await deleteSkill({
      scope: scope.value,
      userId: scope.value === 'user' ? (userId.value ?? undefined) : undefined,
      name: skill.name,
    })
    toast.add({ severity: 'success', summary: '技能已删除', life: 3000 })
    await load()
  } catch (err) {
    notifyError(err, '删除技能失败')
  } finally {
    deleting.value = ''
  }
}

function search() {
  void load()
}

onMounted(async () => {
  try {
    await loadUsers()
  } catch (err) {
    notifyError(err, '加载用户失败')
  }
  await load()
})
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>技能</h1>
        <p class="page-desc">管理公共技能和指定用户的私有技能，ZIP 上传后 sandbox 共享生效。</p>
      </div>
      <span class="tag">{{ skills.length }} 个</span>
    </header>

    <div class="filters">
      <div class="scope-row">
        <div class="scope-switch">
          <button
            type="button"
            :class="{ active: scope === 'public' }"
            @click="switchScope('public')"
          >
            公共技能
          </button>
          <button type="button" :class="{ active: scope === 'user' }" @click="switchScope('user')">
            用户技能
          </button>
        </div>
        <Select
          v-if="scope === 'user'"
          v-model="userId"
          :options="userOptions"
          option-label="label"
          option-value="value"
          placeholder="选择用户"
          aria-label="选择用户"
          @change="load"
        />
      </div>

      <div class="action-row">
        <div class="upload-row">
          <FileUpload
            ref="fileUpload"
            mode="basic"
            :choose-label="scope === 'public' ? '选择公共 ZIP' : '选择用户 ZIP'"
            accept=".zip,application/zip"
            :auto="false"
            :disabled="uploading"
            @select="chooseFile"
          >
            <template #filelabel="{ files }">
              {{ files?.length ? files[0].name : '未选择文件' }}
            </template>
          </FileUpload>
          <label class="overwrite">
            <input v-model="overwrite" type="checkbox" />
            覆盖同名
          </label>
          <Button
            label="上传"
            icon="pi pi-upload"
            :loading="uploading"
            :disabled="!selectedFile"
            @click="upload"
          />
        </div>
        <div class="search-row">
          <InputText v-model="keyword" placeholder="搜索技能名称或描述" @keyup.enter="search" />
          <Button label="搜索" icon="pi pi-search" @click="search" />
        </div>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :rows="skills"
      :loading="loading"
      :error="error"
      empty-text="暂无技能"
      @retry="load"
    >
      <template #cell-source="{ value }">
        <span class="tag">{{ value === 'public' ? '公共' : '用户' }}</span>
      </template>
      <template #cell-actions="{ row }">
        <button
          class="link-button"
          type="button"
          :disabled="deleting === (row as AdminSkill).name"
          @click="remove(row as AdminSkill)"
        >
          {{ deleting === (row as AdminSkill).name ? '删除中…' : '删除' }}
        </button>
      </template>
    </DataTable>
  </div>
</template>

<style scoped>
.filters {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.scope-row,
.action-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.action-row {
  justify-content: space-between;
  flex-wrap: wrap;
}

.scope-switch {
  display: inline-flex;
  align-self: flex-start;
  padding: 3px;
  background: var(--soft);
  border: 1px solid var(--line);
  border-radius: 999px;
}

.scope-switch button {
  padding: 7px 16px;
  background: transparent;
  border: 0;
  border-radius: 999px;
  color: var(--muted);
  cursor: pointer;
  font: inherit;
  font-size: 13px;
}

.scope-switch button.active {
  background: var(--surface);
  color: var(--ink);
  box-shadow: 0 1px 4px rgba(17, 17, 17, 0.08);
}

.upload-row,
.search-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 10px;
}

.search-row {
  margin-left: auto;
}

.overwrite {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--muted);
  font-size: 12px;
}

.search-row :deep(input) {
  width: 260px;
}

.link-button {
  padding: 0;
  background: none;
  border: none;
  color: var(--blue);
  cursor: pointer;
  font-size: 13px;
}

.link-button:disabled {
  cursor: default;
  opacity: 0.5;
}
</style>

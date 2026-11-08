<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Dialog from 'primevue/dialog'
import InputText from 'primevue/inputtext'
import Password from 'primevue/password'
import Select from 'primevue/select'
import Textarea from 'primevue/textarea'
import ToggleSwitch from 'primevue/toggleswitch'
import { useToast } from 'primevue/usetoast'

import { createUser, getUser, getUsers, updateUserPassword, updateUserStatus } from '@/api/admin'
import DataTable from '@/components/DataTable.vue'
import TablePagination from '@/components/TablePagination.vue'
import TableSearch from '@/components/TableSearch.vue'
import { useAdminPage } from '@/composables/useAdminPage'
import type { AdminUser, AdminUserDetail, CreateUserInput, TableColumn } from '@/types'
import { formatDateTime } from '@/utils/format'
import {
  createUserErrorMessage,
  validateCreateUser,
  validateDisableReason,
  validateNewPassword,
} from '@/utils/userAdmin'

const toast = useToast()
const { rows, total, page, pageSize, keyword, loading, error, load, search, setPage, setPageSize } =
  useAdminPage(getUsers)

const columns: TableColumn[] = [
  { key: 'name', label: '用户', width: '160px' },
  { key: 'email', label: '邮箱' },
  { key: 'plan', label: '套餐', width: '100px' },
  { key: 'status', label: '状态', width: '110px', tag: true, highlight: ['active'] },
  { key: 'createdAt', label: '创建时间', width: '170px', format: (value) => formatDateTime(value) },
  { key: 'actions', label: '操作', width: '240px' },
]

const busyId = ref('')
const disableVisible = ref(false)
const disableTarget = ref<AdminUser | null>(null)
const disableReason = ref('')
const disableError = ref('')
const disabling = ref(false)

const passwordVisible = ref(false)
const passwordTarget = ref<AdminUser | null>(null)
const newPassword = ref('')
const confirmPassword = ref('')
const passwordError = ref('')
const savingPassword = ref(false)

const detailVisible = ref(false)
const detail = ref<AdminUserDetail | null>(null)
const detailLoading = ref(false)

const planOptions = [
  { label: '免费 free', value: 'free' },
  { label: '专业 pro', value: 'pro' },
  { label: '旗舰 max', value: 'max' },
]

const createVisible = ref(false)
const creating = ref(false)
const createError = ref('')
const createForm = ref({
  username: '',
  password: '',
  confirmPassword: '',
  email: '',
  nickname: '',
  plan: 'free',
  status: 1,
})

function openCreate() {
  createForm.value = {
    username: '',
    password: '',
    confirmPassword: '',
    email: '',
    nickname: '',
    plan: 'free',
    status: 1,
  }
  createError.value = ''
  createVisible.value = true
}

async function submitCreate() {
  const form = createForm.value
  const message = validateCreateUser(form)
  if (message) {
    createError.value = message
    return
  }
  creating.value = true
  createError.value = ''
  try {
    const input: CreateUserInput = {
      username: form.username.trim(),
      password: form.password,
      email: form.email.trim() || undefined,
      nickname: form.nickname.trim() || undefined,
      plan: form.plan,
      status: form.status,
    }
    await createUser(input)
    notifySuccess('用户已创建')
    createVisible.value = false
    page.value = 1
    await load()
  } catch (err) {
    createError.value = createUserErrorMessage(err)
  } finally {
    creating.value = false
  }
}

const detailInitial = computed(() => {
  const name = detail.value?.name || detail.value?.username || ''
  return name.slice(0, 1).toUpperCase() || '?'
})

function userID(row: unknown): string {
  return (row as AdminUser).id
}

function userName(row: unknown): string {
  return (row as AdminUser).name
}

function isActive(row: unknown): boolean {
  return (row as AdminUser).status === 'active'
}

function statusLabel(status: string): string {
  return status === 'active' ? '启用' : '禁用'
}

function notifySuccess(message: string) {
  toast.add({ severity: 'success', summary: message, life: 3000 })
}

function notifyError(err: unknown, fallback: string) {
  toast.add({
    severity: 'error',
    summary: err instanceof Error ? err.message : fallback,
    life: 5000,
  })
}

async function applyStatus(user: AdminUser, status: number, reason = '') {
  busyId.value = user.id
  try {
    await updateUserStatus(user.id, status, reason)
    notifySuccess(status === 1 ? '用户已启用' : '用户已禁用')
    disableVisible.value = false
    await load()
  } catch (err) {
    notifyError(err, '更新用户状态失败')
    await load()
  } finally {
    busyId.value = ''
  }
}

function onDisableHide() {
  disableError.value = ''
  if (!disabling.value) {
    void load()
  }
}

function openDisable(row: unknown) {
  const user = row as AdminUser
  disableTarget.value = user
  disableReason.value = ''
  disableError.value = ''
  disableVisible.value = true
}

function onToggle(row: unknown, active: boolean) {
  const user = row as AdminUser
  if (active) {
    void applyStatus(user, 1)
    return
  }
  openDisable(user)
}

async function submitDisable() {
  const target = disableTarget.value
  if (!target) {
    return
  }
  const message = validateDisableReason(disableReason.value)
  if (message) {
    disableError.value = message
    return
  }
  disabling.value = true
  disableError.value = ''
  try {
    await updateUserStatus(target.id, 0, disableReason.value.trim())
    notifySuccess('用户已禁用')
    disableVisible.value = false
    await load()
  } catch (err) {
    disableError.value = err instanceof Error ? err.message : '禁用用户失败'
  } finally {
    disabling.value = false
  }
}

function openPassword(row: unknown) {
  passwordTarget.value = row as AdminUser
  newPassword.value = ''
  confirmPassword.value = ''
  passwordError.value = ''
  passwordVisible.value = true
}

async function submitPassword() {
  const target = passwordTarget.value
  if (!target) {
    return
  }
  const message = validateNewPassword(newPassword.value, confirmPassword.value)
  if (message) {
    passwordError.value = message
    return
  }
  savingPassword.value = true
  passwordError.value = ''
  try {
    await updateUserPassword(target.id, newPassword.value)
    notifySuccess('密码已修改')
    passwordVisible.value = false
  } catch (err) {
    passwordError.value = err instanceof Error ? err.message : '修改密码失败'
  } finally {
    savingPassword.value = false
  }
}

async function openDetail(row: unknown) {
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await getUser(userID(row))
  } catch (err) {
    detailVisible.value = false
    notifyError(err, '加载用户详情失败')
  } finally {
    detailLoading.value = false
  }
}
</script>

<template>
  <div class="page">
    <header class="page-head">
      <div>
        <h1>用户</h1>
        <p class="page-desc">MySQL 中的业务用户与套餐视图。</p>
      </div>
      <span class="tag">{{ total }} 条</span>
    </header>

    <div class="users-toolbar">
      <Button label="添加用户" icon="pi pi-plus" @click="openCreate" />
      <TableSearch v-model:keyword="keyword" @search="search" />
    </div>

    <DataTable
      :columns="columns"
      :rows="rows"
      :loading="loading"
      :error="error"
      empty-text="暂无用户"
      @retry="load"
    >
      <template #cell-actions="{ row }">
        <div class="row-actions">
          <button class="link-button" type="button" @click="openDetail(row)">详情</button>
          <button class="link-button" type="button" @click="openPassword(row)">修改密码</button>
          <ToggleSwitch
            :model-value="isActive(row)"
            :disabled="busyId === userID(row)"
            :aria-label="`切换 ${userName(row)} 的状态`"
            @update:model-value="(value) => onToggle(row, Boolean(value))"
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
      v-model:visible="createVisible"
      header="添加用户"
      modal
      :closable="!creating"
      :close-on-escape="!creating"
      :dismissable-mask="!creating"
      :style="{ width: '520px' }"
      @hide="createError = ''"
    >
      <div class="form">
        <label class="field">
          <span>用户名</span>
          <InputText
            v-model="createForm.username"
            maxlength="32"
            placeholder="3-32 位字母、数字、_ 或 -"
          />
        </label>
        <label class="field">
          <span>密码</span>
          <Password
            v-model="createForm.password"
            toggle-mask
            :feedback="false"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span>确认密码</span>
          <Password
            v-model="createForm.confirmPassword"
            toggle-mask
            :feedback="false"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span>邮箱</span>
          <InputText v-model="createForm.email" maxlength="255" placeholder="留空自动生成" />
        </label>
        <label class="field">
          <span>昵称</span>
          <InputText v-model="createForm.nickname" maxlength="255" placeholder="留空使用用户名" />
        </label>
        <label class="field">
          <span>套餐</span>
          <Select
            v-model="createForm.plan"
            :options="planOptions"
            option-label="label"
            option-value="value"
            aria-label="套餐"
          />
        </label>
        <label class="field">
          <span>状态</span>
          <Select
            v-model="createForm.status"
            :options="[
              { label: '启用', value: 1 },
              { label: '禁用', value: 0 },
            ]"
            option-label="label"
            option-value="value"
            aria-label="状态"
          />
        </label>
        <p class="hint">邮箱留空将自动生成 &lt;用户名&gt;@test.local。</p>
        <p v-if="createError" class="error-text">{{ createError }}</p>
      </div>
      <template #footer>
        <Button
          label="取消"
          severity="secondary"
          text
          :disabled="creating"
          @click="createVisible = false"
        />
        <Button label="创建" :loading="creating" @click="submitCreate" />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="disableVisible"
      header="禁用用户"
      modal
      :style="{ width: '460px' }"
      @hide="onDisableHide"
    >
      <div class="form">
        <p class="hint">禁用后该用户将无法登录，并会收到站内通知。</p>
        <p class="target-name">{{ disableTarget?.name }}</p>
        <label class="field">
          <span>禁用原因</span>
          <Textarea
            v-model="disableReason"
            rows="4"
            maxlength="500"
            auto-resize
            placeholder="请输入禁用原因（必填）"
          />
        </label>
        <p v-if="disableError" class="error-text">{{ disableError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="disableVisible = false" />
        <Button label="确认禁用" severity="danger" :loading="disabling" @click="submitDisable" />
      </template>
    </Dialog>

    <Dialog
      v-model:visible="passwordVisible"
      header="修改密码"
      modal
      :style="{ width: '460px' }"
      @hide="passwordError = ''"
    >
      <div class="form">
        <p class="target-name">{{ passwordTarget?.name }}</p>
        <label class="field">
          <span>新密码</span>
          <Password
            v-model="newPassword"
            toggle-mask
            :feedback="false"
            autocomplete="new-password"
          />
        </label>
        <label class="field">
          <span>确认密码</span>
          <Password
            v-model="confirmPassword"
            toggle-mask
            :feedback="false"
            autocomplete="new-password"
          />
        </label>
        <p class="hint">密码至少 8 位；修改后现有登录会话仍然有效。</p>
        <p v-if="passwordError" class="error-text">{{ passwordError }}</p>
      </div>
      <template #footer>
        <Button label="取消" severity="secondary" text @click="passwordVisible = false" />
        <Button label="保存" :loading="savingPassword" @click="submitPassword" />
      </template>
    </Dialog>

    <Dialog v-model:visible="detailVisible" header="用户详情" modal :style="{ width: '600px' }">
      <div v-if="detailLoading" class="detail-state">加载中...</div>
      <div v-else-if="detail" class="detail">
        <div class="detail-head">
          <img v-if="detail.avatarUrl" class="avatar" :src="detail.avatarUrl" alt="" />
          <div v-else class="avatar avatar-fallback">{{ detailInitial }}</div>
          <div class="identity">
            <strong>{{ detail.name }}</strong>
            <span>@{{ detail.username }}</span>
          </div>
          <span class="tag" :class="{ 'tag-accent': detail.status === 'active' }">
            {{ statusLabel(detail.status) }}
          </span>
        </div>

        <dl class="detail-grid">
          <div>
            <dt>邮箱</dt>
            <dd>{{ detail.email || '—' }}</dd>
          </div>
          <div>
            <dt>手机号</dt>
            <dd>{{ detail.phoneNumber || '—' }}</dd>
          </div>
          <div>
            <dt>套餐</dt>
            <dd>{{ detail.plan || '—' }}</dd>
          </div>
          <div>
            <dt>来源</dt>
            <dd>{{ detail.source || '—' }}</dd>
          </div>
          <div>
            <dt>注册时间</dt>
            <dd>{{ formatDateTime(detail.createdAt) }}</dd>
          </div>
          <div>
            <dt>邮箱验证</dt>
            <dd>{{ detail.emailVerifiedAt ? '已验证' : '未验证' }}</dd>
          </div>
          <div>
            <dt>最后登录</dt>
            <dd>{{ formatDateTime(detail.lastLoginTime) }}</dd>
          </div>
          <div>
            <dt>最后登录 IP</dt>
            <dd>{{ detail.lastLoginIp || '—' }}</dd>
          </div>
        </dl>

        <div v-if="detail.disabledReason" class="disable-note">
          <h3>最近禁用</h3>
          <p>{{ detail.disabledReason }}</p>
          <p class="hint">
            {{ formatDateTime(detail.disabledAt) }}
            <span v-if="detail.disabledBy"> · {{ detail.disabledBy }}</span>
          </p>
        </div>
      </div>
      <template #footer>
        <Button label="关闭" severity="secondary" text @click="detailVisible = false" />
      </template>
    </Dialog>
  </div>
</template>

<style scoped>
.users-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}

.users-toolbar :deep(.table-search) {
  margin-bottom: 0;
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

.hint {
  margin: 0;
  color: var(--muted);
  font-size: 12px;
}

.target-name {
  margin: 0;
  color: var(--ink);
  font-size: 14px;
  font-weight: 600;
}

.row-actions {
  display: flex;
  align-items: center;
  gap: 14px;
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

.detail-state {
  padding: 24px 0;
  color: var(--muted);
  text-align: center;
}

.detail {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.detail-head {
  display: flex;
  align-items: center;
  gap: 14px;
}

.avatar {
  width: 56px;
  height: 56px;
  border: 1px solid var(--line);
  border-radius: 50%;
  object-fit: cover;
}

.avatar-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--soft);
  color: var(--muted);
  font-size: 20px;
  font-weight: 600;
}

.identity {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 4px;
}

.identity strong {
  color: var(--ink);
  font-size: 16px;
}

.identity span {
  color: var(--muted);
  font-size: 12px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px 24px;
  margin: 0;
}

.detail-grid div {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.detail-grid dt {
  color: var(--muted);
  font-size: 12px;
}

.detail-grid dd {
  margin: 0;
  color: var(--text);
  font-size: 13px;
  overflow-wrap: anywhere;
}

.disable-note {
  padding: 14px 16px;
  background: var(--soft);
  border: 1px solid var(--line);
}

.disable-note h3 {
  margin: 0 0 8px;
  color: var(--ink);
  font-size: 13px;
}

.disable-note p {
  margin: 0 0 6px;
  color: var(--text);
  font-size: 13px;
}

.disable-note p:last-child {
  margin-bottom: 0;
}
</style>

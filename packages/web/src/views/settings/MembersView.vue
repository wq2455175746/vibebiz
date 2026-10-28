<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { computed, onMounted, ref } from 'vue'

import { inviteMember, listMembers } from '@/api/settings'
import { useAuthStore } from '@/stores/auth'
import type { Member } from '@/types'
import { formatDateTime } from '@/utils/storage'

const auth = useAuthStore()
const members = ref<Member[]>([])
const loading = ref(true)
const loadError = ref('')
const email = ref('')
const inviting = ref(false)
const inviteError = ref('')
const notice = ref('')

const isOwner = computed(() => {
  const current = auth.user
  if (!current) {
    return false
  }
  return (
    current.workspaces.find((workspace) => workspace.id === current.currentWorkspaceId)?.isOwner ??
    false
  )
})

function roleLabel(role: string): string {
  return role === 'owner' ? '所有者' : '编辑者'
}

function statusLabel(status: number): string {
  return status === 1 ? '已加入' : '未激活'
}

function initial(member: Member): string {
  const name = member.username.trim()
  return name ? name.charAt(0).toUpperCase() : '?'
}

async function invite() {
  const target = email.value.trim()
  if (!target || inviting.value) {
    return
  }
  inviting.value = true
  inviteError.value = ''
  notice.value = ''
  try {
    const member = await inviteMember(target)
    members.value = [...members.value, member]
    email.value = ''
    notice.value = `已邀请 ${member.username}`
  } catch (err) {
    inviteError.value = err instanceof Error ? err.message : '邀请失败'
  } finally {
    inviting.value = false
  }
}

onMounted(async () => {
  try {
    members.value = await listMembers()
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '成员加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>成员</h1>
        <p>邀请已注册用户加入当前工作区。</p>
      </div>
    </header>

    <form v-if="isOwner" class="settings-card invite-form" @submit.prevent="invite">
      <h2 class="settings-card-title">邀请工作区成员</h2>
      <div class="invite-row">
        <InputText
          id="invite-email"
          v-model="email"
          type="email"
          aria-label="邀请邮箱"
          placeholder="输入已注册邮箱"
          :disabled="inviting"
        />
        <Button
          type="submit"
          label="邀请"
          :loading="inviting"
          :disabled="!email.trim() || inviting"
        />
      </div>
      <p v-if="inviteError" class="settings-error" role="alert">{{ inviteError }}</p>
      <p v-if="notice" class="settings-notice" role="status">{{ notice }}</p>
    </form>

    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>
    <div v-else-if="members.length === 0" class="empty-state">当前工作区还没有成员</div>

    <div v-else class="settings-card settings-table-wrap">
      <table class="settings-table">
        <thead>
          <tr>
            <th>成员</th>
            <th>邮箱</th>
            <th>角色</th>
            <th>状态</th>
            <th>加入时间</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="member in members" :key="member.userId">
            <td>
              <div class="member-cell">
                <img
                  v-if="member.avatarUrl"
                  class="member-avatar"
                  :src="member.avatarUrl"
                  :alt="member.username"
                />
                <span v-else class="member-avatar fallback">{{ initial(member) }}</span>
                <span class="cell-name">{{ member.username }}</span>
              </div>
            </td>
            <td>{{ member.email }}</td>
            <td>{{ roleLabel(member.role) }}</td>
            <td>{{ statusLabel(member.status) }}</td>
            <td>{{ formatDateTime(member.joinedAt) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

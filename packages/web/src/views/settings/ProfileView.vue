<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import { computed, onMounted, ref } from 'vue'

import {
  confirmAvatar,
  getProfile,
  presignAvatar,
  updateProfile,
  uploadAvatar,
  type PresignInput,
} from '@/api/settings'
import AvatarUploader from '@/components/settings/AvatarUploader.vue'
import { useAuthStore } from '@/stores/auth'
import type { Presign } from '@/types'

const auth = useAuthStore()
const loading = ref(true)
const loadError = ref('')
const editing = ref(false)
const saving = ref(false)
const draft = ref('')
const editError = ref('')
const notice = ref('')

const user = computed(() => auth.user)

function startEdit() {
  draft.value = user.value?.username ?? ''
  editError.value = ''
  notice.value = ''
  editing.value = true
}

function cancelEdit() {
  editing.value = false
  editError.value = ''
}

async function saveUsername() {
  const username = draft.value.trim()
  if (username.length < 2 || username.length > 64) {
    editError.value = '用户名长度需为 2-64 个字符'
    return
  }
  saving.value = true
  editError.value = ''
  try {
    auth.applyUser(await updateProfile(username))
    editing.value = false
    notice.value = '用户名已更新'
  } catch (err) {
    editError.value = err instanceof Error ? err.message : '用户名更新失败'
  } finally {
    saving.value = false
  }
}

async function presignUserAvatar(input: PresignInput): Promise<Presign> {
  return presignAvatar(input)
}

async function confirmUserAvatar(key: string): Promise<void> {
  auth.applyUser(await confirmAvatar(key))
}

async function fallbackUserAvatar(file: File): Promise<void> {
  auth.applyUser(await uploadAvatar(file))
}

function onAvatarUploaded() {
  notice.value = '头像已更新'
}

onMounted(async () => {
  try {
    auth.applyUser(await getProfile())
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '资料加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>账户设置</h1>
        <p>管理你的个人资料并个性化你的账户体验。</p>
      </div>
    </header>

    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>

    <div v-else class="settings-card settings-card-flush">
      <div class="settings-row">
        <div class="settings-row-main">
          <h2>头像</h2>
          <p>上传或更换你的头像，展示在账户和协作场景中。</p>
        </div>
        <div class="settings-row-control">
          <AvatarUploader
            :src="user?.avatarUrl ?? null"
            :name="user?.username ?? ''"
            :presign="presignUserAvatar"
            :confirm="confirmUserAvatar"
            :fallback="fallbackUserAvatar"
            @uploaded="onAvatarUploaded"
          />
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-row-main">
          <h2>用户名</h2>
          <p>选择你的名字在 Vibebiz 上的显示方式。</p>
        </div>
        <div class="settings-row-control">
          <div v-if="!editing" class="profile-inline profile-inline-end">
            <strong>{{ user?.username }}</strong>
            <Button label="编辑" icon="pi pi-pencil" size="small" text @click="startEdit" />
          </div>
          <form v-else class="profile-inline profile-inline-end" @submit.prevent="saveUsername">
            <InputText v-model="draft" maxlength="64" :disabled="saving" />
            <Button
              type="submit"
              label="保存"
              size="small"
              :loading="saving"
              :disabled="!draft.trim()"
            />
            <Button label="取消" size="small" text :disabled="saving" @click="cancelEdit" />
          </form>
          <p v-if="editError" class="settings-error" role="alert">{{ editError }}</p>
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-row-main">
          <h2>邮箱</h2>
          <p>邮箱用于登录和账户识别，当前不支持修改。</p>
        </div>
        <div class="settings-row-control">
          <div class="profile-inline profile-inline-end">
            <span>{{ user?.email }}</span>
            <span class="tag">只读</span>
          </div>
        </div>
      </div>

      <p v-if="notice" class="settings-notice settings-row-notice" role="status">{{ notice }}</p>
    </div>
  </section>
</template>

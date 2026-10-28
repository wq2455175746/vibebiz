<script setup lang="ts">
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Textarea from 'primevue/textarea'
import { computed, onMounted, ref } from 'vue'

import {
  confirmWorkspaceAvatar,
  getCurrentWorkspace,
  presignWorkspaceAvatar,
  updateCurrentWorkspace,
  uploadWorkspaceAvatar,
  type PresignInput,
} from '@/api/settings'
import AvatarUploader from '@/components/settings/AvatarUploader.vue'
import type { Presign, Workspace } from '@/types'

const workspace = ref<Workspace | null>(null)
const loading = ref(true)
const loadError = ref('')
const saving = ref(false)
const saveError = ref('')
const notice = ref('')
const name = ref('')
const description = ref('')

const editable = computed(() => workspace.value?.isOwner ?? false)

function applyWorkspace(next: Workspace) {
  workspace.value = next
  name.value = next.name
  description.value = next.description
}

async function save() {
  const trimmed = name.value.trim()
  if (trimmed.length < 1 || trimmed.length > 100) {
    saveError.value = '工作区名称长度需为 1-100 个字符'
    return
  }
  if (description.value.length > 500) {
    saveError.value = '工作区描述不能超过 500 个字符'
    return
  }
  saving.value = true
  saveError.value = ''
  notice.value = ''
  try {
    applyWorkspace(await updateCurrentWorkspace(trimmed, description.value.trim()))
    notice.value = '工作区已保存'
  } catch (err) {
    saveError.value = err instanceof Error ? err.message : '工作区保存失败'
  } finally {
    saving.value = false
  }
}

async function presignWorkspace(input: PresignInput): Promise<Presign> {
  return presignWorkspaceAvatar(input)
}

async function confirmWorkspace(key: string): Promise<void> {
  applyWorkspace(await confirmWorkspaceAvatar(key))
}

async function fallbackWorkspaceAvatar(file: File): Promise<void> {
  applyWorkspace(await uploadWorkspaceAvatar(file))
}

function onAvatarUploaded() {
  notice.value = '工作区头像已更新'
}

onMounted(async () => {
  try {
    applyWorkspace(await getCurrentWorkspace())
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '工作区加载失败'
  } finally {
    loading.value = false
  }
})
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>工作区设置</h1>
        <p>工作区让你和成员协作处理项目。只有所有者可以修改以下信息。</p>
      </div>
      <span v-if="workspace" class="tag">{{ workspace.memberCount }} 位成员</span>
    </header>

    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>

    <form v-else-if="workspace" class="settings-card settings-card-flush" @submit.prevent="save">
      <div class="settings-row">
        <div class="settings-row-main">
          <h2>头像</h2>
          <p>为你的工作区设置头像，方便成员快速识别。</p>
        </div>
        <div class="settings-row-control">
          <AvatarUploader
            :src="workspace.avatarUrl"
            :name="workspace.name"
            :disabled="!editable"
            :presign="presignWorkspace"
            :confirm="confirmWorkspace"
            :fallback="fallbackWorkspaceAvatar"
            @uploaded="onAvatarUploaded"
          />
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-row-main">
          <h2>名称</h2>
          <p>工作区在导航和邀请中展示的名称。</p>
        </div>
        <div class="settings-row-control">
          <InputText v-model="name" maxlength="100" :disabled="!editable || saving" />
          <p class="settings-field-hint">{{ name.length }} / 100 字符</p>
        </div>
      </div>

      <div class="settings-row">
        <div class="settings-row-main">
          <h2>描述</h2>
          <p>简单说明这个工作区的用途或团队。</p>
        </div>
        <div class="settings-row-control">
          <Textarea
            v-model="description"
            rows="3"
            maxlength="500"
            :disabled="!editable || saving"
          />
          <p class="settings-field-hint">{{ description.length }} / 500 字符</p>
        </div>
      </div>

      <p v-if="saveError" class="settings-error settings-row-notice" role="alert">
        {{ saveError }}
      </p>
      <p v-if="notice" class="settings-notice settings-row-notice" role="status">{{ notice }}</p>

      <div v-if="editable" class="settings-actions settings-actions-bordered">
        <Button type="submit" label="保存" :loading="saving" :disabled="!name.trim() || saving" />
      </div>
    </form>
  </section>
</template>

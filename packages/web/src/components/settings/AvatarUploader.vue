<script setup lang="ts">
import axios from 'axios'
import { computed, ref } from 'vue'

import type { PresignInput } from '@/api/settings'
import type { Presign } from '@/types'
import { ACCEPTED_AVATAR_TYPES, validateAvatarFile } from '@/utils/avatar'

const props = defineProps<{
  src: string | null
  name: string
  disabled?: boolean
  presign: (input: PresignInput) => Promise<Presign>
  confirm: (key: string) => Promise<void>
  fallback?: (file: File) => Promise<void>
}>()

const emit = defineEmits<{ (event: 'uploaded'): void }>()

const fileInput = ref<HTMLInputElement | null>(null)
const uploading = ref(false)
const error = ref('')

const initial = computed(() => {
  const name = props.name.trim()
  return name ? name.charAt(0).toUpperCase() : '?'
})

const accept = ACCEPTED_AVATAR_TYPES.join(',')

function pick() {
  if (props.disabled || uploading.value) {
    return
  }
  fileInput.value?.click()
}

async function onChange(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file) {
    return
  }
  const invalid = validateAvatarFile(file)
  if (invalid) {
    error.value = invalid
    return
  }
  error.value = ''
  uploading.value = true
  try {
    const presigned = await props.presign({
      filename: file.name,
      contentType: file.type,
      size: file.size,
    })
    try {
      await axios.put(presigned.uploadUrl, file, { headers: presigned.headers })
    } catch (err) {
      if (!props.fallback) {
        throw err
      }
      await props.fallback(file)
      emit('uploaded')
      return
    }
    await props.confirm(presigned.key)
    emit('uploaded')
  } catch (err) {
    error.value = err instanceof Error ? err.message : '头像上传失败，请重试'
  } finally {
    uploading.value = false
  }
}
</script>

<template>
  <div class="avatar-uploader">
    <input
      ref="fileInput"
      class="avatar-file-input"
      type="file"
      :accept="accept"
      :disabled="disabled || uploading"
      @change="onChange"
    />
    <button
      class="avatar-trigger"
      type="button"
      :disabled="disabled || uploading"
      aria-label="更换头像"
      @click="pick"
    >
      <img v-if="src" class="avatar-image" :src="src" alt="头像" />
      <span v-else class="avatar-fallback">{{ initial }}</span>
      <span class="avatar-overlay">
        <i v-if="!disabled" class="pi pi-camera" />
        {{ disabled ? '只读' : uploading ? '上传中' : '更换' }}
      </span>
    </button>
    <p v-if="error" class="avatar-error" role="alert">{{ error }}</p>
    <p v-else class="avatar-hint">支持 JPG、PNG、WebP，最大 5MB</p>
  </div>
</template>

<script setup lang="ts">
import Button from 'primevue/button'
import { computed, ref, watch } from 'vue'

import { buildPreviewUrl } from '@/utils/preview'

const props = defineProps<{ projectId: string; refreshToken: number }>()

const frameKey = ref(0)
const loading = ref(true)
const src = computed(() => buildPreviewUrl(props.projectId))

function reload() {
  loading.value = true
  frameKey.value += 1
}

function openInNewTab() {
  window.open(src.value, '_blank', 'noopener,noreferrer')
}

watch(
  () => props.refreshToken,
  () => reload(),
)

watch(
  () => props.projectId,
  () => reload(),
)
</script>

<template>
  <div class="preview-panel">
    <div class="preview-toolbar">
      <Button icon="pi pi-refresh" text rounded aria-label="刷新预览" @click="reload" />
      <span class="preview-address">
        <i class="pi pi-lock" />
        {{ src }}
      </span>
      <span class="tag">静态预览</span>
      <Button
        icon="pi pi-external-link"
        text
        rounded
        aria-label="新窗口打开"
        @click="openInNewTab"
      />
    </div>
    <div class="preview-stage">
      <div v-if="loading" class="preview-loading">
        <i class="pi pi-spin pi-spinner" />
        正在加载预览
      </div>
      <iframe
        :key="frameKey"
        :src="src"
        title="应用预览"
        sandbox="allow-scripts allow-forms"
        referrerpolicy="no-referrer"
        @load="loading = false"
      />
    </div>
  </div>
</template>

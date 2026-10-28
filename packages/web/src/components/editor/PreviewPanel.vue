<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'

import { buildPreviewUrl } from '@/utils/preview'

const props = defineProps<{ projectId: string }>()

const frameKey = ref(0)
const src = computed(() => buildPreviewUrl(props.projectId))

function reload() {
  frameKey.value += 1
}
</script>

<template>
  <div class="preview-panel">
    <div class="preview-toolbar">
      <Button icon="pi pi-refresh" text rounded aria-label="刷新预览" @click="reload" />
      <span class="preview-url">{{ src }}</span>
      <span class="tag">iframe · 沙箱静态服务</span>
    </div>
    <div class="preview-stage">
      <iframe :key="frameKey" :src="src" title="应用预览" sandbox="allow-scripts allow-forms" />
    </div>
  </div>
</template>

<script setup lang="ts">
import Button from 'primevue/button'

defineProps<{
  projectName: string
  projectState: string
  consoleOpen: boolean
  modelLabel: string
}>()

const emit = defineEmits<{
  (event: 'back'): void
  (event: 'toggle-console'): void
}>()
</script>

<template>
  <header class="editor-top">
    <div class="editor-identity">
      <Button icon="pi pi-angle-left" text rounded aria-label="返回首页" @click="emit('back')" />
      <strong class="project-title">{{ projectName }}</strong>
      <span class="tag">{{ projectState }}</span>
    </div>

    <div class="editor-meta">
      <span class="model-chip"><i class="pi pi-sparkles" /> {{ modelLabel }}</span>
      <button
        class="console-toggle"
        type="button"
        :class="{ active: consoleOpen }"
        @click="emit('toggle-console')"
      >
        <i class="pi pi-desktop" />
        控制台
      </button>
    </div>

    <div class="editor-actions">
      <Button label="分享" text />
      <Button label="发布" />
    </div>
  </header>
</template>

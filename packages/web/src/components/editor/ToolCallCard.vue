<script setup lang="ts">
import type { ToolState } from '@/types'

defineProps<{ tool: ToolState }>()
</script>

<template>
  <div class="tool-card" :class="{ done: tool.status === 'done' }">
    <span class="tool-icon">
      <i :class="tool.status === 'running' ? 'pi pi-spin pi-spinner' : 'pi pi-check'" />
    </span>
    <span class="tool-name">{{ tool.name }}</span>
    <span class="tool-status">{{ tool.status === 'running' ? '执行中' : '完成' }}</span>
  </div>
</template>

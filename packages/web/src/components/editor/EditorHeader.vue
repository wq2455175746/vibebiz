<script setup lang="ts">
import Button from 'primevue/button'

import { roleDisplayLabel } from '@/utils/agentRoles'
import type { AgentRole } from '@/types'

defineProps<{
  projectName: string
  projectState: string
  modelLabel: string
  roles: AgentRole[]
}>()

const emit = defineEmits<{
  (event: 'back'): void
}>()
</script>

<template>
  <header class="editor-top">
    <div class="editor-identity">
      <Button icon="pi pi-angle-left" text rounded aria-label="返回首页" @click="emit('back')" />
      <strong class="project-title">{{ projectName }}</strong>
      <span v-if="roles.length" class="editor-roles">
        <span v-for="role in roles" :key="role.id" class="editor-role-chip">
          {{ roleDisplayLabel(role) }}
        </span>
      </span>
      <span class="tag">{{ projectState }}</span>
    </div>

    <div class="editor-meta">
      <span class="model-chip"><i class="pi pi-sparkles" /> {{ modelLabel }}</span>
    </div>

    <div class="editor-actions">
      <Button label="分享" text />
      <Button label="发布" />
    </div>
  </header>
</template>

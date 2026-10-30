<script setup lang="ts">
import type { EnterpriseStageDefinition } from '@/utils/enterprise'

defineProps<{
  stage: EnterpriseStageDefinition
  readOnly: boolean
  busy: boolean
  canComplete: boolean
  confirming: boolean
  error: string
}>()

const emit = defineEmits<{
  (event: 'complete'): void
  (event: 'back'): void
}>()
</script>

<template>
  <header class="enterprise-stage-head">
    <div class="enterprise-stage-title">
      <p class="enterprise-stage-eyebrow">
        {{ readOnly ? '历史阶段 · 只读' : `阶段 ${stage.seq}` }}
      </p>
      <h1>{{ stage.title }}空间</h1>
      <div class="enterprise-stage-meta">
        <span>{{ stage.roleLabel }}</span>
        <span v-if="stage.artifactPath" class="enterprise-artifact-path">
          <i class="pi pi-file" />
          {{ stage.artifactPath }}
        </span>
      </div>
    </div>
    <div class="enterprise-stage-actions">
      <button v-if="readOnly" class="enterprise-secondary" type="button" @click="emit('back')">
        返回当前阶段
      </button>
      <button
        v-else
        class="enterprise-primary"
        type="button"
        :disabled="!canComplete || confirming"
        :title="
          busy
            ? 'Agent 正在执行，完成后才能确认'
            : canComplete
              ? '确认当前阶段并进入下一阶段'
              : `等待 ${stage.roleLabel} 写入 ${stage.artifactPath}`
        "
        @click="emit('complete')"
      >
        <i :class="confirming ? 'pi pi-spin pi-spinner' : 'pi pi-check'" />
        {{ confirming ? '确认中…' : busy ? '执行中…' : '确认本阶段' }}
      </button>
    </div>
  </header>
  <p v-if="error" class="enterprise-stage-error">{{ error }}</p>
</template>

<script setup lang="ts">
import type { EnterpriseStageDefinition } from '@/utils/enterprise'

defineProps<{ stage: EnterpriseStageDefinition }>()
</script>

<template>
  <section class="enterprise-placeholder">
    <span class="enterprise-placeholder-icon"><i class="pi pi-lock" /></span>
    <p class="enterprise-placeholder-eyebrow">阶段 {{ stage.seq }}</p>
    <h1>{{ stage.title }}暂未开放</h1>
    <p>当前版本仅开放需求、设计、测试与开发阶段。</p>
    <small>制品构建与云环境交付将在后续版本接入。</small>
  </section>
</template>

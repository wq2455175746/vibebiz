<script setup lang="ts">
import { computed } from 'vue'

import { renderMarkdown } from '@/utils/markdown'

const props = defineProps<{
  content: string
  path: string
  roleLabel: string
  loading: boolean
  error: string
}>()

const html = computed(() => (props.content ? renderMarkdown(props.content) : ''))
</script>

<template>
  <section class="enterprise-document">
    <div v-if="loading" class="enterprise-document-state">
      <i class="pi pi-spin pi-spinner" />
      <strong>正在读取阶段产物</strong>
      <span>{{ path }}</span>
    </div>
    <div v-else-if="error" class="enterprise-document-state">
      <i class="pi pi-exclamation-circle" />
      <strong>产物读取失败</strong>
      <span>{{ error }}</span>
    </div>
    <div v-else-if="!content" class="enterprise-document-state">
      <i class="pi pi-file-edit" />
      <strong>等待 {{ roleLabel }} 输出文档</strong>
      <span>{{ path }}</span>
      <small>文档生成后会自动显示在这里，完成后点击“确认本阶段”。</small>
    </div>
    <article v-else class="enterprise-document-body" v-html="html" />
  </section>
</template>

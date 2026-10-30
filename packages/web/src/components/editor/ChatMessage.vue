<script setup lang="ts">
import { computed, ref } from 'vue'

import ToolCallCard from './ToolCallCard.vue'
import type { ChatMessage } from '@/types'
import { formatDateTime } from '@/utils/storage'
import { summarizeTools } from '@/utils/toolCalls'

const props = defineProps<{ message: ChatMessage }>()
const toolsExpanded = ref(false)
const toolSummary = computed(() => summarizeTools(props.message.tools))
</script>

<template>
  <article class="chat-message" :class="`role-${message.role}`">
    <template v-if="message.role === 'system'">
      <p class="chat-note">{{ message.text }}</p>
    </template>
    <template v-else>
      <p class="chat-role">
        <span>{{ message.role === 'user' ? '你' : 'Vibebiz' }}</span>
        <time v-if="message.createdAt" :datetime="message.createdAt">
          {{ formatDateTime(message.createdAt) }}
        </time>
      </p>
      <div class="chat-bubble">
        <p v-if="message.text" class="chat-text">{{ message.text }}</p>
        <div v-if="message.references?.length" class="chat-reference-list">
          <span v-for="reference in message.references" :key="`${reference.kind}:${reference.id ?? reference.path}`" class="chat-reference-chip">
            <i :class="reference.resourceKind === 'image' ? 'pi pi-image' : 'pi pi-file'" />
            {{ reference.name }}
          </span>
        </div>
        <div v-if="message.tools.length > 0" class="tool-list">
          <button
            v-if="toolSummary.collapsible"
            type="button"
            class="tool-summary"
            :aria-expanded="toolsExpanded"
            @click="toolsExpanded = !toolsExpanded"
          >
            <i :class="toolSummary.running ? 'pi pi-spin pi-spinner' : 'pi pi-wrench'" />
            <span class="tool-summary-title">工具调用 {{ toolSummary.total }} 次</span>
            <span class="tool-summary-detail">{{ toolSummary.detail }}</span>
            <span class="tool-summary-state">
              {{ toolSummary.running ? '执行中' : '已完成' }}
            </span>
            <i :class="toolsExpanded ? 'pi pi-chevron-up' : 'pi pi-chevron-down'" />
          </button>
          <template v-if="!toolSummary.collapsible || toolsExpanded">
            <ToolCallCard v-for="tool in message.tools" :key="tool.id" :tool="tool" />
          </template>
        </div>
        <span v-if="message.pending" class="typing-dots" aria-label="正在生成">
          <i />
          <i />
          <i />
        </span>
      </div>
    </template>
  </article>
</template>

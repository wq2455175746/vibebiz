<script setup lang="ts">
import ToolCallCard from './ToolCallCard.vue'
import type { ChatMessage } from '@/types'

defineProps<{ message: ChatMessage }>()
</script>

<template>
  <article class="chat-message" :class="`role-${message.role}`">
    <template v-if="message.role === 'system'">
      <p class="chat-note">{{ message.text }}</p>
    </template>
    <template v-else>
      <p class="chat-role">{{ message.role === 'user' ? '你' : 'Vibebiz' }}</p>
      <div class="chat-bubble">
        <p v-if="message.text" class="chat-text">{{ message.text }}</p>
        <div v-if="message.tools.length > 0" class="tool-list">
          <ToolCallCard v-for="tool in message.tools" :key="tool.id" :tool="tool" />
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

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'

import ChatMessage from './ChatMessage.vue'
import { useChatStore } from '@/stores/chat'

const props = defineProps<{ projectId: string }>()

const chat = useChatStore()
const prompt = ref('')
const scroller = ref<HTMLElement | null>(null)
let stickToBottom = true

function onScroll() {
  const element = scroller.value
  if (!element) {
    return
  }
  stickToBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 80
}

async function scrollToBottom() {
  await nextTick()
  if (stickToBottom && scroller.value) {
    scroller.value.scrollTop = scroller.value.scrollHeight
  }
}

function send() {
  const text = prompt.value.trim()
  if (!text || chat.streaming) {
    return
  }
  prompt.value = ''
  void chat.send(props.projectId, text)
  stickToBottom = true
  void scrollToBottom()
}

function stop() {
  chat.stop(props.projectId)
}

watch(
  () => [chat.messages.length, chat.messages.at(-1)?.text, chat.messages.at(-1)?.pending],
  () => void scrollToBottom(),
)
</script>

<template>
  <aside class="chat-panel">
    <header class="chat-head">
      <div>
        <p class="chat-eyebrow">任务对话</p>
        <h2>把想法交给 Agent</h2>
      </div>
      <span class="chat-status" :class="{ active: chat.streaming }">
        <i />
        {{ chat.streaming ? '运行中' : '就绪' }}
      </span>
    </header>

    <div ref="scroller" class="chat-scroll" @scroll.passive="onScroll">
      <ChatMessage v-for="message in chat.messages" :key="message.id" :message="message" />
    </div>

    <form class="chat-composer" @submit.prevent="send">
      <textarea
        v-model="prompt"
        rows="3"
        placeholder="描述你想实现或修改的内容…"
        :disabled="chat.streaming"
        @keydown.ctrl.enter.prevent="send"
        @keydown.meta.enter.prevent="send"
      />
      <div class="chat-composer-bar">
        <span class="chat-hint">Ctrl / ⌘ + Enter 发送</span>
        <button
          v-if="chat.streaming"
          class="chat-send chat-stop"
          type="button"
          aria-label="停止生成"
          @click="stop"
        >
          <i class="pi pi-stop" />
        </button>
        <button v-else class="chat-send" type="submit" aria-label="发送" :disabled="!prompt.trim()">
          <i class="pi pi-arrow-up" />
        </button>
      </div>
    </form>
  </aside>
</template>

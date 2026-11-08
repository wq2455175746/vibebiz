<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import type { EvolutionChatMessage, EvolutionProposal, EvolutionVersion } from '@/types/evolution'
import {
  formatShortTime,
  groupMessagesByDay,
  kindLabel,
  roleLabel,
  statusLabel,
} from '@/utils/evolution'

const props = defineProps<{
  messages: EvolutionChatMessage[]
  proposals: EvolutionProposal[]
  isOwn: boolean
  selectedUserName: string
  currentVersionLabel: string
  latestPreviewVersion: EvolutionVersion | null
  busy: boolean
}>()

const emit = defineEmits<{
  (event: 'submit-chat', content: string): void
  (event: 'submit-proposal', content: string): void
  (event: 'confirm-version', versionId: string): void
}>()

type Tab = 'chat' | 'history' | 'queue'
type Mode = 'chat' | 'proposal'

const tab = ref<Tab>('chat')
const mode = ref<Mode>('proposal')
const draft = ref('')
const scroller = ref<HTMLElement | null>(null)

const pendingCount = computed(
  () =>
    props.proposals.filter(
      (proposal) => proposal.status === 'queued' || proposal.status === 'reviewing',
    ).length,
)
const historyGroups = computed(() => groupMessagesByDay(props.messages))
const canSend = computed(() => draft.value.trim().length > 0 && !props.busy)

async function scrollToBottom(): Promise<void> {
  await nextTick()
  const element = scroller.value
  if (element) {
    element.scrollTop = element.scrollHeight
  }
}

function send(): void {
  const content = draft.value.trim()
  if (!content || props.busy) {
    return
  }
  if (mode.value === 'proposal') {
    emit('submit-proposal', content)
  } else {
    emit('submit-chat', content)
  }
  draft.value = ''
  void scrollToBottom()
}

function onKeydown(event: KeyboardEvent): void {
  if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
    event.preventDefault()
    send()
  }
}

function switchMode(next: Mode): void {
  mode.value = next
}

function confirm(version: EvolutionVersion): void {
  emit('confirm-version', version.id)
}

watch(
  () => props.messages.length,
  () => {
    void scrollToBottom()
  },
)

onMounted(() => {
  void scrollToBottom()
})
</script>

<template>
  <aside class="evolution-chat">
    <header class="evolution-chat-head">
      <div>
        <strong>共创对话</strong>
        <span>{{ selectedUserName }}{{ isOwn ? ' · 我的版本' : ' · 只读' }}</span>
      </div>
      <span class="evolution-version-pill">{{ currentVersionLabel }}</span>
    </header>

    <nav class="evolution-tabs" aria-label="集思进化面板">
      <button type="button" :class="{ active: tab === 'chat' }" @click="tab = 'chat'">对话</button>
      <button type="button" :class="{ active: tab === 'history' }" @click="tab = 'history'">
        历史
      </button>
      <button type="button" :class="{ active: tab === 'queue' }" @click="tab = 'queue'">
        提议
        <em v-if="pendingCount">{{ pendingCount }}</em>
      </button>
    </nav>

    <div ref="scroller" class="evolution-chat-body">
      <template v-if="tab === 'chat'">
        <article
          v-for="message in messages"
          :key="message.id"
          class="evolution-message"
          :class="message.role"
        >
          <header>
            <span class="evolution-message-role">{{ roleLabel(message.role) }}</span>
            <time>{{ formatShortTime(message.createdAt) }}</time>
          </header>
          <p>{{ message.content }}</p>
        </article>

        <p v-if="!messages.length" class="evolution-empty">
          还没有对话。描述你遇到的痛点，Agent 会先评估再决定是否生成版本。
        </p>

        <div v-if="isOwn && latestPreviewVersion" class="evolution-confirm-card">
          <strong>候选版本 {{ latestPreviewVersion.label }} 已生成</strong>
          <p>{{ latestPreviewVersion.summary }}</p>
          <button type="button" @click="confirm(latestPreviewVersion)">
            效果没问题，确认这个版本
          </button>
        </div>
      </template>

      <template v-else-if="tab === 'history'">
        <section v-for="group in historyGroups" :key="group.key" class="evolution-history-group">
          <h4>{{ group.label }}</h4>
          <button
            v-for="message in group.messages"
            :key="message.id"
            type="button"
            class="evolution-history-item"
            @click="tab = 'chat'"
          >
            <span class="evolution-history-role">{{ roleLabel(message.role) }}</span>
            <span class="evolution-history-text">{{ message.content }}</span>
            <time>{{ formatShortTime(message.createdAt) }}</time>
          </button>
        </section>
        <p v-if="!historyGroups.length" class="evolution-empty">暂无聊天历史。</p>
      </template>

      <template v-else>
        <article
          v-for="proposal in proposals"
          :key="proposal.id"
          class="evolution-proposal"
          :class="proposal.kind"
        >
          <header>
            <span class="evolution-kind">{{ kindLabel(proposal.kind) }}</span>
            <span class="evolution-status" :class="proposal.status">
              {{ statusLabel(proposal.status) }}
            </span>
            <time>{{ formatShortTime(proposal.createdAt) }}</time>
          </header>
          <p class="evolution-proposal-content">{{ proposal.content }}</p>
          <p class="evolution-proposal-reply">
            <i class="pi pi-sparkles" />
            {{ proposal.agentReply }}
          </p>
        </article>
        <p v-if="!proposals.length" class="evolution-empty">还没有提议，提交第一条痛点试试。</p>
      </template>
    </div>

    <footer v-if="isOwn" class="evolution-composer">
      <div class="evolution-mode">
        <button
          type="button"
          :class="{ active: mode === 'proposal' }"
          @click="switchMode('proposal')"
        >
          提议
        </button>
        <button type="button" :class="{ active: mode === 'chat' }" @click="switchMode('chat')">
          对话
        </button>
        <span>{{ mode === 'proposal' ? '写入提议队列' : '写入对话记录' }}</span>
      </div>
      <p v-if="busy" class="evolution-empty">Agent 正在修改 sandbox 版本…</p>
      <textarea
        v-model="draft"
        rows="3"
        :disabled="busy"
        :placeholder="
          mode === 'proposal' ? '描述你遇到的痛点或改进建议，Agent 会先评估再生成…' : '继续对话…'
        "
        @keydown="onKeydown"
      />
      <div class="evolution-composer-foot">
        <span>Ctrl / ⌘ + Enter 发送</span>
        <button type="button" :disabled="!canSend" @click="send">
          {{ mode === 'proposal' ? '提交提议' : '发送' }}
        </button>
      </div>
    </footer>

    <p v-else class="evolution-readonly">
      <i class="pi pi-lock" />
      正在查看其他用户的版本，内容只读。
    </p>
  </aside>
</template>

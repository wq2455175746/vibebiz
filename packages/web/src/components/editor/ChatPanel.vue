<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import ChatMessage from './ChatMessage.vue'
import SkillsDialog from './SkillsDialog.vue'
import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import type { ChatReferenceInput } from '@/api/chat'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { useResourceStore } from '@/stores/resources'
import { applyMention, collectReferences, findActiveMention, mergeReferences } from '@/utils/mentions'
import {
  commandContextError,
  matchCommands,
  resolveCommand,
  type SlashCommand,
} from '@/utils/slashCommands'

const props = defineProps<{ projectId: string }>()

const chat = useChatStore()
const files = useFileStore()
const resources = useResourceStore()
const prompt = ref('')
const composerNote = ref('')
const resourceInput = ref<HTMLInputElement | null>(null)
const textarea = ref<HTMLTextAreaElement | null>(null)
const attachedIds = ref<string[]>([])
const mentionOpen = ref(false)
const scroller = ref<HTMLElement | null>(null)
const mentionStart = ref(0)
const mentionEnd = ref(0)
const menuIndex = ref(0)
const showSkills = ref(false)
const slashDismissed = ref(false)
let stickToBottom = true

const candidates = computed(() => matchCommands(prompt.value, 'editor'))
const menuOpen = computed(
  () => candidates.value.length > 0 && !chat.busy && !slashDismissed.value,
)

watch(candidates, () => {
  menuIndex.value = 0
  slashDismissed.value = false
})

const attached = computed(() =>
  resources.items.filter((item) => attachedIds.value.includes(item.id)),
)

const mentionOptions = computed(() => {
  const query = prompt.value.slice(mentionStart.value, mentionEnd.value).toLowerCase()
  const resourceOptions = resources.items
    .filter((item) => item.displayName.toLowerCase().includes(query))
    .slice(0, 6)
    .map((item) => ({
      key: `resource:${item.id}`,
      label: item.displayName,
      hint: '资料',
      insert: item.displayName,
    }))
  const fileOptions = files.entries
    .filter((item) => item.type === 'file' && item.path.toLowerCase().includes(query))
    .slice(0, 6)
    .map((item) => ({
      key: `file:${item.path}`,
      label: item.path,
      hint: '项目文件',
      insert: item.path,
    }))
  return [...resourceOptions, ...fileOptions].slice(0, 10)
})

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

function closeMention() {
  mentionOpen.value = false
}

function onEscape() {
  closeMention()
  slashDismissed.value = true
}

function updateMention() {
  const target = textarea.value
  if (!target || chat.busy) {
    closeMention()
    return
  }
  const match = findActiveMention(prompt.value, target.selectionStart ?? prompt.value.length)
  mentionOpen.value = !!match
  if (match) {
    mentionStart.value = match.start
    mentionEnd.value = match.end
  }
}

function selectMention(option: { insert: string }) {
  const match = { start: mentionStart.value, end: mentionEnd.value, query: '' }
  const result = applyMention(prompt.value, match, option.insert)
  prompt.value = result.value
  closeMention()
  void nextTick(() => {
    textarea.value?.focus()
    textarea.value?.setSelectionRange(result.cursor, result.cursor)
  })
}

async function chooseFiles(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  target.value = ''
  if (!file || chat.busy) {
    return
  }
  composerNote.value = ''
  try {
    const item = await resources.upload(props.projectId, file)
    attachedIds.value = mergeReferences([
      ...attachedIds.value.map((id) => ({ kind: 'resource' as const, id })),
      { kind: 'resource', id: item.id },
    ]).map((item) => item.id ?? '')
  } catch (err) {
    composerNote.value = (err as Error).message
  }
}

function removeAttached(resourceId: string) {
  attachedIds.value = attachedIds.value.filter((id) => id !== resourceId)
}

function executeCommand(command: SlashCommand) {
  if (command.disabled) {
    composerNote.value = '模型选择即将支持'
    prompt.value = ''
    return
  }
  if (command.name === 'skills') {
    composerNote.value = ''
    prompt.value = ''
    closeMention()
    showSkills.value = true
  }
}

function selectCommand(command: SlashCommand) {
  executeCommand(command)
}

function onEnter(event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const command = candidates.value[menuIndex.value]
  if (command) {
    selectCommand(command)
  }
}

function onArrow(step: number, event: KeyboardEvent) {
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const total = candidates.value.length
  menuIndex.value = (menuIndex.value + step + total) % total
}

function send() {
  const text = prompt.value.trim()
  if (!text || chat.busy) {
    return
  }
  const command = resolveCommand(text, 'editor')
  if (command) {
    executeCommand(command)
    return
  }
  const contextError = commandContextError(text, 'editor')
  if (contextError) {
    composerNote.value = contextError
    return
  }
  if (files.dirty) {
    composerNote.value = '请先保存或取消当前文件编辑'
    return
  }
  const references: ChatReferenceInput[] = mergeReferences([
    ...collectReferences(prompt.value, resources.items, files.entries),
    ...attached.value.map((item) => ({
      kind: 'resource' as const,
      id: item.id,
      name: item.displayName,
      path: item.path,
      contentType: item.contentType,
      resourceKind: item.kind,
    })),
  ])
  if (references.length > 10) {
    composerNote.value = '一次最多引用 10 个文件'
    return
  }
  composerNote.value = ''
  prompt.value = ''
  attachedIds.value = []
  closeMention()
  void chat.send(props.projectId, text, references)
  stickToBottom = true
  void scrollToBottom()
}

function stop() {
  void chat.stop(props.projectId)
}

async function load() {
  attachedIds.value = []
  closeMention()
  await resources.load(props.projectId)
}

onMounted(load)
watch(
  () => props.projectId,
  () => void load(),
)
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
      <span class="chat-status" :class="{ active: chat.busy }">
        <i />
        {{ chat.busy ? '运行中' : '就绪' }}
      </span>
    </header>

    <div ref="scroller" class="chat-scroll" @scroll.passive="onScroll">
      <ChatMessage v-for="message in chat.messages" :key="message.id" :message="message" />
    </div>

    <form class="chat-composer" @submit.prevent="send">
      <div v-if="attached.length" class="chat-attachments">
        <span v-for="item in attached" :key="item.id" class="chat-reference-chip">
          <i :class="item.kind === 'image' ? 'pi pi-image' : 'pi pi-file'" />
          {{ item.displayName }}
          <button type="button" aria-label="移除附件" @click="removeAttached(item.id)">
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <p v-if="composerNote" class="chat-note chat-composer-note">{{ composerNote }}</p>
      <div class="chat-input-wrap">
        <SlashCommandMenu
          v-if="menuOpen"
          :commands="candidates"
          :active-index="menuIndex"
          @select="selectCommand"
        />
        <textarea
          ref="textarea"
          v-model="prompt"
          rows="3"
          placeholder="描述你想实现或修改的内容… 输入 @ 引用文件"
          :disabled="chat.busy"
          @input="updateMention"
          @click="updateMention"
          @keydown.enter.exact="onEnter"
          @keydown.down.exact="onArrow(1, $event)"
          @keydown.up.exact="onArrow(-1, $event)"
          @keydown.ctrl.enter.prevent="send"
          @keydown.meta.enter.prevent="send"
          @keydown.esc="onEscape"
          @blur="closeMention"
        />
        <div v-if="mentionOpen && mentionOptions.length" class="mention-menu">
          <button
            v-for="option in mentionOptions"
            :key="option.key"
            type="button"
            class="mention-item"
            @mousedown.prevent="selectMention(option)"
          >
            <span>{{ option.label }}</span>
            <small>{{ option.hint }}</small>
          </button>
        </div>
      </div>
      <input
        ref="resourceInput"
        class="chat-file-input"
        type="file"
        accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.md,.html,.css,.js,.ts,.json,.yaml,.yml,.xml,.csv,.png,.jpg,.jpeg,.webp,.gif"
        @change="chooseFiles"
      />
      <div class="chat-composer-bar">
        <button
          class="chat-plus"
          type="button"
          aria-label="添加资料"
          :disabled="chat.busy || resources.uploading"
          @click="resourceInput?.click()"
        >
          <i :class="resources.uploading ? 'pi pi-spin pi-spinner' : 'pi pi-plus'" />
        </button>
        <span class="chat-hint">输入 / 查看可用命令 · Ctrl / ⌘ + Enter 发送</span>
        <button
          v-if="chat.busy"
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
    <SkillsDialog v-if="showSkills" :project-id="props.projectId" @close="showSkills = false" />
  </aside>
</template>

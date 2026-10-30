<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import Popover from 'primevue/popover'

import AgentRoleMenu from '@/components/AgentRoleMenu.vue'
import ChatMessage from './ChatMessage.vue'
import SkillsDialog from './SkillsDialog.vue'
import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import type { ChatReferenceInput } from '@/api/chat'
import { updateProjectAgentRoles } from '@/api/projects'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useChatStore } from '@/stores/chat'
import { useFileStore } from '@/stores/files'
import { useProjectStore } from '@/stores/project'
import { useResourceStore } from '@/stores/resources'
import type { AgentRole } from '@/types'
import {
  consumeRoleTrigger,
  filterRoleOptions,
  findRoleTrigger,
  roleSaveContextMatches,
  type RoleSaveContext,
  toggleRoleSelection,
} from '@/utils/agentRoles'
import {
  applyMention,
  collectReferences,
  findActiveMention,
  mentionQuery,
  mergeReferences,
} from '@/utils/mentions'
import {
  commandContextError,
  matchCommands,
  resolveCommand,
  type SlashCommand,
} from '@/utils/slashCommands'

const props = defineProps<{
  projectId: string
  readonly?: boolean
  requiredRoleCode?: string
}>()

const chat = useChatStore()
const files = useFileStore()
const resources = useResourceStore()
const projectStore = useProjectStore()
const agentRoles = useAgentRolesStore()
const prompt = ref('')
const composerNote = ref('')
const resourceInput = ref<HTMLInputElement | null>(null)
const attachMenu = ref<InstanceType<typeof Popover> | null>(null)
const textarea = ref<HTMLTextAreaElement | null>(null)
const attachedIds = ref<string[]>([])
const mentionOpen = ref(false)
const scroller = ref<HTMLElement | null>(null)
const mentionStart = ref(0)
const mentionEnd = ref(0)
const mentionIndex = ref(0)
const menuIndex = ref(0)
const showSkills = ref(false)
const slashDismissed = ref(false)
const selectedRoleIds = ref<number[]>([])
const roleOpen = ref(false)
const roleQuery = ref('')
const roleIndex = ref(0)
const roleSaving = ref(false)
const roleError = ref('')
const roleCatalogNotice = ref('')
const lastConfirmedRoleIds = ref<number[]>([])
let stickToBottom = true
let roleContextVersion = 0
let roleOpenSeq = 0
let roleWriteQueue: Promise<void> = Promise.resolve()

const candidates = computed(() => matchCommands(prompt.value, 'editor'))
const menuOpen = computed(() => candidates.value.length > 0 && !chat.busy && !slashDismissed.value)

watch(candidates, () => {
  menuIndex.value = 0
  slashDismissed.value = false
})

const attached = computed(() =>
  resources.items.filter((item) => attachedIds.value.includes(item.id)),
)

const roleOptions = computed(() =>
  filterRoleOptions(agentRoles.roles, roleQuery.value).slice(0, 10),
)

const roleCatalogError = computed(() => roleCatalogNotice.value || agentRoles.error)

const selectedRoles = computed(() =>
  selectedRoleIds.value
    .map((id) => agentRoles.byId(id))
    .filter((role): role is AgentRole => Boolean(role)),
)
const requiredRoleId = computed(
  () => agentRoles.roles.find((role) => role.roleCode === props.requiredRoleCode)?.id ?? null,
)

const mentionOptions = computed(() => {
  const query = mentionQuery(prompt.value, mentionStart.value, mentionEnd.value)
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

watch(mentionOptions, () => {
  mentionIndex.value = 0
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
  roleOpen.value = false
  slashDismissed.value = true
}

function updateMention() {
  const target = textarea.value
  if (!target || chat.busy) {
    closeMention()
    return
  }
  const cursor = target.selectionStart ?? target.value.length
  const match = findActiveMention(target.value, cursor)
  mentionOpen.value = !!match
  if (match) {
    mentionStart.value = match.start
    mentionEnd.value = match.end
  }
}

async function ensureRoleCatalog(): Promise<boolean> {
  if (agentRoles.roles.length) {
    roleCatalogNotice.value = ''
    return true
  }
  await agentRoles.load(true)
  if (agentRoles.roles.length) {
    roleCatalogNotice.value = ''
    return true
  }
  roleCatalogNotice.value = agentRoles.error || '角色列表暂不可用，请稍后重试'
  return false
}

async function updateComposer(event: Event): Promise<void> {
  const seq = ++roleOpenSeq
  updateMention()
  const target = event.target as HTMLTextAreaElement
  let match = findRoleTrigger(target.value, target.selectionStart ?? target.value.length)
  if (!match) {
    roleOpen.value = false
    return
  }
  if (!(await ensureRoleCatalog())) {
    if (seq === roleOpenSeq) {
      roleOpen.value = false
    }
    return
  }
  if (seq !== roleOpenSeq) {
    return
  }
  const latest = textarea.value
  const currentMatch = latest
    ? findRoleTrigger(latest.value, latest.selectionStart ?? latest.value.length)
    : null
  if (!latest || !currentMatch) {
    roleOpen.value = false
    return
  }
  match = currentMatch
  const consumed = consumeRoleTrigger(latest.value, match.start, match.end)
  prompt.value = consumed.value
  roleQuery.value = match.query
  roleOpen.value = true
  roleIndex.value = 0
  void nextTick(() => {
    textarea.value?.focus()
    textarea.value?.setSelectionRange(consumed.cursor, consumed.cursor)
  })
}

function currentRoleContext(): RoleSaveContext {
  return { projectId: props.projectId, version: roleContextVersion }
}

function syncRolesFromProject(): void {
  const current = projectStore.current
  if (current && current.id !== props.projectId) {
    return
  }
  const ids = [...(current?.agentRoleIds ?? [])]
  selectedRoleIds.value = ids
  lastConfirmedRoleIds.value = [...ids]
}

async function persistRoles(next: number[]): Promise<void> {
  if (roleSaving.value) {
    return
  }
  const context = currentRoleContext()
  const savedProjectId = context.projectId
  selectedRoleIds.value = [...next]
  roleSaving.value = true
  roleError.value = ''
  const run = async () => {
    try {
      const saved = await updateProjectAgentRoles(savedProjectId, next)
      if (!roleSaveContextMatches(context, currentRoleContext())) {
        return
      }
      const ids = [...saved]
      lastConfirmedRoleIds.value = [...ids]
      selectedRoleIds.value = [...ids]
      if (projectStore.current?.id === savedProjectId) {
        projectStore.current.agentRoleIds = [...ids]
        projectStore.current.defaultAgentRoleId = ids[0] ?? null
      }
    } catch (err) {
      if (!roleSaveContextMatches(context, currentRoleContext())) {
        return
      }
      selectedRoleIds.value = [...lastConfirmedRoleIds.value]
      roleError.value = (err as Error).message
    } finally {
      if (roleSaveContextMatches(context, currentRoleContext())) {
        roleSaving.value = false
      }
    }
  }
  roleWriteQueue = roleWriteQueue.then(run, run)
  await roleWriteQueue
}

async function toggleRole(role: { id: number }): Promise<void> {
  if (roleSaving.value) {
    return
  }
  if (role.id === requiredRoleId.value && selectedRoleIds.value.includes(role.id)) {
    return
  }
  const next = toggleRoleSelection(agentRoles.roles, selectedRoleIds.value, role.id)
  await persistRoles(next)
}

async function removeRole(id: number): Promise<void> {
  if (roleSaving.value || id === requiredRoleId.value) {
    return
  }
  await persistRoles(selectedRoleIds.value.filter((item) => item !== id))
}

async function toggleRoleMenu(): Promise<void> {
  roleQuery.value = ''
  roleIndex.value = 0
  if (roleOpen.value) {
    roleOpen.value = false
    return
  }
  if (!(await ensureRoleCatalog())) {
    roleOpen.value = false
    return
  }
  roleOpen.value = true
}

function onRoleEnter(): boolean {
  if (roleSaving.value) {
    return false
  }
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  const role = roleOptions.value[roleIndex.value]
  if (role) {
    void toggleRole(role)
  }
  return true
}

function onRoleArrow(step: number, event: KeyboardEvent): boolean {
  if (roleSaving.value) {
    return false
  }
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  event.preventDefault()
  const total = roleOptions.value.length
  roleIndex.value = (roleIndex.value + step + total) % total
  return true
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

function toggleAttachMenu(event: Event) {
  attachMenu.value?.toggle(event)
}

function chooseAttachment() {
  attachMenu.value?.hide()
  resourceInput.value?.click()
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
  if (onRoleEnter()) {
    event.preventDefault()
    return
  }
  if (mentionOpen.value && mentionOptions.value.length) {
    event.preventDefault()
    const option = mentionOptions.value[mentionIndex.value]
    if (option) {
      selectMention(option)
    }
    return
  }
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
  if (onRoleArrow(step, event)) {
    return
  }
  if (mentionOpen.value && mentionOptions.value.length) {
    event.preventDefault()
    const total = mentionOptions.value.length
    mentionIndex.value = (mentionIndex.value + step + total) % total
    return
  }
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const total = candidates.value.length
  menuIndex.value = (menuIndex.value + step + total) % total
}

function send() {
  const text = prompt.value.trim()
  if (!text || chat.busy || props.readonly) {
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
  roleOpen.value = false
  void chat.send(props.projectId, text, references)
  stickToBottom = true
  void scrollToBottom()
}

function stop() {
  void chat.stop(props.projectId)
}

function onDocumentMousedown(event: MouseEvent) {
  if (!roleOpen.value) {
    return
  }
  const target = event.target
  if (target instanceof Element && target.closest('.agent-role-menu, .chat-role-chip')) {
    return
  }
  roleOpen.value = false
}

function onDocumentKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    roleOpen.value = false
  }
}

async function load() {
  attachedIds.value = []
  closeMention()
  await Promise.all([resources.load(props.projectId), agentRoles.load()])
  syncRolesFromProject()
}

onMounted(() => {
  document.addEventListener('mousedown', onDocumentMousedown)
  document.addEventListener('keydown', onDocumentKeydown)
  void load()
})

onBeforeUnmount(() => {
  roleContextVersion += 1
  roleOpenSeq += 1
  roleSaving.value = false
  document.removeEventListener('mousedown', onDocumentMousedown)
  document.removeEventListener('keydown', onDocumentKeydown)
})
watch(
  () => props.projectId,
  () => {
    roleContextVersion += 1
    roleOpenSeq += 1
    roleSaving.value = false
    roleError.value = ''
    roleCatalogNotice.value = ''
    roleOpen.value = false
    roleQuery.value = ''
    selectedRoleIds.value = []
    lastConfirmedRoleIds.value = []
    syncRolesFromProject()
    void load()
  },
)
watch(
  () => projectStore.current?.agentRoleIds,
  () => syncRolesFromProject(),
  { deep: true },
)
watch(
  () => [chat.messages.length, chat.messages.at(-1)?.text, chat.messages.at(-1)?.pending],
  () => void scrollToBottom(),
)
</script>

<template>
  <aside class="chat-panel" :class="{ readonly }">
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
      <div v-if="selectedRoles.length" class="chat-roles">
        <span
          v-for="role in selectedRoles"
          :key="role.id"
          class="chat-role-chip"
          @click="toggleRoleMenu"
        >
          #{{ role.roleNameZh }}
          <button
            v-if="role.roleCode !== requiredRoleCode"
            type="button"
            aria-label="移除角色"
            :disabled="roleSaving"
            @click.stop="removeRole(role.id)"
          >
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <p v-if="roleCatalogError" class="chat-note chat-composer-note">{{ roleCatalogError }}</p>
      <p v-if="roleError" class="chat-note chat-composer-note">{{ roleError }}</p>
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
      <p v-if="readonly" class="chat-note chat-composer-note">
        历史阶段只读，返回当前阶段后继续对话。
      </p>
      <div class="chat-input-wrap">
        <AgentRoleMenu
          v-if="roleOpen && roleOptions.length"
          :roles="roleOptions"
          :selected="selectedRoleIds"
          :active-index="roleIndex"
          :disabled="roleSaving"
          @toggle="toggleRole"
        />
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
          :disabled="chat.busy || readonly"
          @input="updateComposer"
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
            v-for="(option, index) in mentionOptions"
            :key="option.key"
            type="button"
            class="mention-item"
            :class="{ active: mentionIndex === index }"
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
          :disabled="chat.busy || readonly || resources.uploading"
          @click="toggleAttachMenu"
        >
          <i :class="resources.uploading ? 'pi pi-spin pi-spinner' : 'pi pi-plus'" />
        </button>
        <span class="chat-hint">{{
          readonly ? '历史阶段只读' : '输入 / 查看可用命令 · Ctrl / ⌘ + Enter 发送'
        }}</span>
        <button
          v-if="chat.busy"
          class="chat-send chat-stop"
          type="button"
          aria-label="停止生成"
          @click="stop"
        >
          <i class="pi pi-stop" />
        </button>
        <button
          v-else
          class="chat-send"
          type="submit"
          aria-label="发送"
          :disabled="!prompt.trim() || readonly"
        >
          <i class="pi pi-arrow-up" />
        </button>
      </div>
      <Popover ref="attachMenu">
        <div class="composer-attach-menu">
          <button type="button" @click="chooseAttachment">
            <i class="pi pi-upload" />
            <span>上传文件</span>
          </button>
        </div>
      </Popover>
    </form>
    <SkillsDialog v-if="showSkills" :project-id="props.projectId" @close="showSkills = false" />
  </aside>
</template>

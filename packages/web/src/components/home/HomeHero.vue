<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'

import Popover from 'primevue/popover'

import AgentRoleMenu from '@/components/AgentRoleMenu.vue'
import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import { useAgentRolesStore } from '@/stores/agentRoles'
import type { AgentRole, Project } from '@/types'
import {
  consumeRoleTrigger,
  filterRoleOptions,
  findRoleTrigger,
  toggleRoleSelection,
} from '@/utils/agentRoles'
import {
  commandContextError,
  matchCommands,
  resolveCommand,
  stripCommand,
  type SlashCommand,
} from '@/utils/slashCommands'

const prompt = defineModel<string>({ required: true })

const props = defineProps<{
  creating: boolean
  error: string
  targetProject?: Project | null
}>()

const emit = defineEmits<{
  submit: [files: File[], agentRoleIds: number[]]
  selectProject: []
  clearTarget: []
}>()

const files = ref<File[]>([])
const input = ref<HTMLInputElement | null>(null)
const attachMenu = ref<InstanceType<typeof Popover> | null>(null)
const textarea = ref<HTMLTextAreaElement | null>(null)
const fileError = ref('')
const commandNote = ref('')
const menuIndex = ref(0)
const slashDismissed = ref(false)
const rolesStore = useAgentRolesStore()
const selectedRoleIds = ref<number[]>([])
const roleOpen = ref(false)
const roleQuery = ref('')
const roleIndex = ref(0)

const candidates = computed(() => matchCommands(prompt.value, 'home'))
const menuOpen = computed(
  () => candidates.value.length > 0 && !props.creating && !slashDismissed.value,
)
const roleOptions = computed(() =>
  filterRoleOptions(rolesStore.roles, roleQuery.value).slice(0, 10),
)

onMounted(() => {
  void rolesStore.load()
})

watch(
  () => props.targetProject,
  (target) => {
    if (target) {
      selectedRoleIds.value = []
      roleOpen.value = false
    }
  },
)

watch(candidates, () => {
  menuIndex.value = 0
  slashDismissed.value = false
})

function chooseFiles(event: Event) {
  const target = event.target as HTMLInputElement
  const selected = Array.from(target.files ?? [])
  const invalid = selected.find((file) => file.size <= 0 || file.size > 20 * 1024 * 1024)
  if (invalid) {
    fileError.value = `${invalid.name} 超过 20MB`
  } else {
    fileError.value = ''
    files.value = [...files.value, ...selected]
  }
  target.value = ''
}

function toggleAttachMenu(event: Event) {
  attachMenu.value?.toggle(event)
}

function chooseAttachment() {
  attachMenu.value?.hide()
  input.value?.click()
}

function chooseScheduledTask() {
  attachMenu.value?.hide()
  commandNote.value = '定时任务即将支持'
}

function removeFile(index: number) {
  files.value = files.value.filter((_, i) => i !== index)
  fileError.value = ''
}

function onEscape() {
  slashDismissed.value = true
  roleOpen.value = false
}

async function openRoleMenu(value: string, cursor: number): Promise<void> {
  if (props.targetProject) {
    roleOpen.value = false
    return
  }
  const match = findRoleTrigger(value, cursor)
  if (!match) {
    roleOpen.value = false
    return
  }
  const consumed = consumeRoleTrigger(value, match.start, match.end)
  prompt.value = consumed.value
  roleQuery.value = match.query
  roleOpen.value = true
  roleIndex.value = 0
  await nextTick()
  textarea.value?.focus()
  textarea.value?.setSelectionRange(consumed.cursor, consumed.cursor)
}

function onInput(event: Event): void {
  commandNote.value = ''
  const target = event.target as HTMLTextAreaElement
  void openRoleMenu(target.value, target.selectionStart ?? target.value.length)
}

function toggleRole(role: { id: number }): void {
  selectedRoleIds.value = toggleRoleSelection(rolesStore.roles, selectedRoleIds.value, role.id)
}

function removeRole(id: number): void {
  selectedRoleIds.value = selectedRoleIds.value.filter((item) => item !== id)
}

function onRoleEnter(): boolean {
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  const role = roleOptions.value[roleIndex.value]
  if (role) {
    toggleRole(role)
  }
  return true
}

function onRoleArrow(step: number, event: KeyboardEvent): boolean {
  if (!roleOpen.value || !roleOptions.value.length) {
    return false
  }
  event.preventDefault()
  const total = roleOptions.value.length
  roleIndex.value = (roleIndex.value + step + total) % total
  return true
}

function roleChips() {
  return selectedRoleIds.value
    .map((id) => rolesStore.byId(id))
    .filter((role): role is AgentRole => Boolean(role))
}

function executeCommand(command: SlashCommand) {
  if (command.name === 'project') {
    commandNote.value = ''
    prompt.value = stripCommand(prompt.value, 'project')
    emit('selectProject')
  }
}

function onEnter(event: KeyboardEvent) {
  if (onRoleEnter()) {
    event.preventDefault()
    return
  }
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const command = candidates.value[menuIndex.value]
  if (command) {
    executeCommand(command)
  }
}

function onArrow(step: number, event: KeyboardEvent) {
  if (onRoleArrow(step, event)) {
    return
  }
  if (!menuOpen.value) {
    return
  }
  event.preventDefault()
  const total = candidates.value.length
  menuIndex.value = (menuIndex.value + step + total) % total
}

function submit() {
  const text = prompt.value.trim()
  const command = resolveCommand(text, 'home')
  if (command) {
    executeCommand(command)
    return
  }
  const contextError = commandContextError(text, 'home')
  if (contextError) {
    commandNote.value = contextError
    return
  }
  commandNote.value = ''
  roleOpen.value = false
  emit('submit', files.value, selectedRoleIds.value)
}
</script>

<template>
  <section class="home-hero">
    <h1 class="hero-title">你想创造什么，wangh11？</h1>

    <div class="hero-composer">
      <div v-if="targetProject" class="composer-target">
        <span>将追加到：{{ targetProject.name }}</span>
        <button type="button" aria-label="取消目标项目" @click="emit('clearTarget')">
          <i class="pi pi-times" />
        </button>
      </div>
      <AgentRoleMenu
        v-if="roleOpen && roleOptions.length"
        :roles="roleOptions"
        :selected="selectedRoleIds"
        :active-index="roleIndex"
        @toggle="toggleRole"
      />
      <SlashCommandMenu
        v-if="menuOpen"
        :commands="candidates"
        :active-index="menuIndex"
        @select="executeCommand"
      />
      <textarea
        ref="textarea"
        v-model="prompt"
        class="hero-input"
        rows="3"
        :placeholder="targetProject ? '描述要追加到该项目的修改…' : '请 Vibebiz 生成你的 LinkHub。'"
        @input="onInput"
        @blur="roleOpen = false"
        @keydown.enter.exact="onEnter"
        @keydown.down.exact="onArrow(1, $event)"
        @keydown.up.exact="onArrow(-1, $event)"
        @keydown.esc="onEscape"
        @keydown.ctrl.enter.prevent="submit"
        @keydown.meta.enter.prevent="submit"
      />
      <div v-if="roleChips().length" class="composer-roles">
        <span v-for="role in roleChips()" :key="role.id" class="composer-role-chip">
          #{{ role.roleNameZh }}
          <button type="button" aria-label="移除角色" @click="removeRole(role.id)">
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <div class="composer-files">
        <span v-for="(file, index) in files" :key="`${file.name}-${index}`" class="composer-file">
          <i class="pi pi-file" />
          <span>{{ file.name }}</span>
          <button type="button" aria-label="移除文件" @click="removeFile(index)">
            <i class="pi pi-times" />
          </button>
        </span>
      </div>
      <input
        ref="input"
        class="composer-file-input"
        type="file"
        multiple
        accept=".pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.md,.html,.css,.js,.ts,.json,.yaml,.yml,.xml,.csv,.png,.jpg,.jpeg,.webp,.gif"
        @change="chooseFiles"
      />
      <p v-if="commandNote" class="error-text composer-file-error">{{ commandNote }}</p>
      <p v-if="fileError" class="error-text composer-file-error">{{ fileError }}</p>
      <div class="composer-bar">
        <button class="composer-plus" type="button" aria-label="添加" @click="toggleAttachMenu">
          <i class="pi pi-plus" />
        </button>
        <span class="composer-hint">输入 / 选择已有项目 · Ctrl / ⌘ + Enter 发送</span>
        <div class="composer-actions">
          <button class="composer-icon" type="button" aria-label="语音输入">
            <i class="pi pi-microphone" />
          </button>
          <button
            class="composer-send"
            type="button"
            :disabled="!prompt.trim() || creating"
            aria-label="生成"
            @click="submit"
          >
            <i :class="creating ? 'pi pi-spin pi-spinner' : 'pi pi-arrow-up'" />
          </button>
        </div>
      </div>
      <Popover ref="attachMenu">
        <div class="composer-attach-menu">
          <button type="button" @click="chooseAttachment">
            <i class="pi pi-upload" />
            <span>上传文件</span>
          </button>
          <button type="button" @click="chooseScheduledTask">
            <i class="pi pi-clock" />
            <span>定时任务</span>
          </button>
        </div>
      </Popover>
    </div>

    <p v-if="error" class="error-text hero-error">{{ error }}</p>
  </section>
</template>

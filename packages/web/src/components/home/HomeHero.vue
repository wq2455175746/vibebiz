<script setup lang="ts">
import { computed, ref, watch } from 'vue'

import SlashCommandMenu from '@/components/SlashCommandMenu.vue'
import type { Project } from '@/types'
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
  submit: [files: File[]]
  selectProject: []
  clearTarget: []
}>()

const files = ref<File[]>([])
const input = ref<HTMLInputElement | null>(null)
const fileError = ref('')
const commandNote = ref('')
const menuIndex = ref(0)
const slashDismissed = ref(false)

const candidates = computed(() => matchCommands(prompt.value, 'home'))
const menuOpen = computed(
  () => candidates.value.length > 0 && !props.creating && !slashDismissed.value,
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

function removeFile(index: number) {
  files.value = files.value.filter((_, i) => i !== index)
  fileError.value = ''
}

function onEscape() {
  slashDismissed.value = true
}

function executeCommand(command: SlashCommand) {
  if (command.name === 'project') {
    commandNote.value = ''
    prompt.value = stripCommand(prompt.value, 'project')
    emit('selectProject')
  }
}

function onEnter(event: KeyboardEvent) {
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
  emit('submit', files.value)
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
      <SlashCommandMenu
        v-if="menuOpen"
        :commands="candidates"
        :active-index="menuIndex"
        @select="executeCommand"
      />
      <textarea
        v-model="prompt"
        class="hero-input"
        rows="3"
        :placeholder="targetProject ? '描述要追加到该项目的修改…' : '请 Vibebiz 生成你的 LinkHub。'"
        @input="commandNote = ''"
        @keydown.enter.exact="onEnter"
        @keydown.down.exact="onArrow(1, $event)"
        @keydown.up.exact="onArrow(-1, $event)"
        @keydown.esc="onEscape"
        @keydown.ctrl.enter.prevent="submit"
        @keydown.meta.enter.prevent="submit"
      />
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
        <button class="composer-plus" type="button" aria-label="添加附件" @click="input?.click()">
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
    </div>

    <p v-if="error" class="error-text hero-error">{{ error }}</p>
  </section>
</template>

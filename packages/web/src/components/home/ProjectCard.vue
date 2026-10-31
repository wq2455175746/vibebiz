<script setup lang="ts">
import Popover from 'primevue/popover'
import { computed, ref } from 'vue'

import type { Project } from '@/types'
import { resolveEditorPath } from '@/utils/enterprise'
import { formatDate } from '@/utils/storage'

const props = defineProps<{
  project: Project
  shareUrl?: string
  hideDelete?: boolean
  deleteLabel?: string
}>()

const emit = defineEmits<{
  (event: 'open', id: string): void
  (event: 'delete', project: Project): void
}>()

const menu = ref<InstanceType<typeof Popover> | null>(null)
const coverFailed = ref(false)
const link = computed(
  () => props.shareUrl || `${window.location.origin}${resolveEditorPath(props.project)}`,
)

function toggleMenu(event: Event) {
  menu.value?.toggle(event)
}

function openProject() {
  menu.value?.hide()
  emit('open', props.project.id)
}

async function copyLink() {
  menu.value?.hide()
  try {
    await navigator.clipboard?.writeText(link.value)
  } catch {
    return
  }
}

function requestDelete() {
  menu.value?.hide()
  emit('delete', props.project)
}
</script>

<template>
  <article class="project-card">
    <button class="project-card-open" type="button" @click="openProject">
      <span class="project-cover" :class="{ empty: !project.coverUrl || coverFailed }">
        <img
          v-if="project.coverUrl && !coverFailed"
          :src="project.coverUrl"
          alt=""
          @error="coverFailed = true"
        />
        <span v-else class="project-cover-empty">
          <i class="pi pi-image" />
          <span>暂无预览图</span>
        </span>
      </span>
      <span class="project-info">
        <strong>{{ project.name }}</strong>
        <span>{{ formatDate(project.createdAt) }}</span>
      </span>
    </button>
    <button
      class="project-menu-trigger"
      type="button"
      aria-label="项目操作"
      @click.stop="toggleMenu"
    >
      <i class="pi pi-ellipsis-h" />
    </button>
    <Popover ref="menu">
      <div class="project-menu">
        <button type="button" @click="openProject">
          <i class="pi pi-external-link" />
          {{ shareUrl ? '打开分享页' : '打开项目' }}
        </button>
        <button type="button" @click="copyLink">
          <i class="pi pi-link" />
          {{ shareUrl ? '复制分享链接' : '复制链接' }}
        </button>
        <button v-if="!hideDelete" class="danger" type="button" @click="requestDelete">
          <i class="pi pi-trash" />
          {{ deleteLabel || '删除' }}
        </button>
      </div>
    </Popover>
  </article>
</template>

<script setup lang="ts">
import Popover from 'primevue/popover'
import { ref } from 'vue'

import type { Project } from '@/types'
import { formatDate } from '@/utils/storage'

const props = defineProps<{ project: Project }>()

const emit = defineEmits<{
  (event: 'open', id: string): void
  (event: 'delete', project: Project): void
}>()

const menu = ref<InstanceType<typeof Popover> | null>(null)
const coverFailed = ref(false)

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
    await navigator.clipboard?.writeText(`${window.location.origin}/editor/${props.project.id}`)
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
          打开项目
        </button>
        <button type="button" @click="copyLink">
          <i class="pi pi-link" />
          复制链接
        </button>
        <button class="danger" type="button" @click="requestDelete">
          <i class="pi pi-trash" />
          删除
        </button>
      </div>
    </Popover>
  </article>
</template>

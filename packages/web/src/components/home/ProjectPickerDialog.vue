<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import type { Project } from '@/types'

const props = defineProps<{ projects: Project[] }>()
const emit = defineEmits<{ close: []; select: [project: Project] }>()

const query = ref('')
const filtered = computed(() => {
  const keyword = query.value.trim().toLowerCase()
  if (!keyword) {
    return props.projects
  }
  return props.projects.filter((project) => project.name.toLowerCase().includes(keyword))
})

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    emit('close')
  }
}

function formatTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? '' : date.toLocaleString()
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<template>
  <div class="dialog-backdrop" @click.self="emit('close')">
    <section class="dialog-card project-picker" role="dialog" aria-modal="true" aria-label="选择项目">
      <header class="dialog-head">
        <div>
          <p class="dialog-eyebrow">SELECT PROJECT</p>
          <h2>追加到已有项目</h2>
        </div>
        <button type="button" class="dialog-close" aria-label="关闭" @click="emit('close')">
          <i class="pi pi-times" />
        </button>
      </header>

      <input v-model="query" class="project-search" type="search" placeholder="搜索项目名称…" />

      <p v-if="!filtered.length" class="dialog-state">
        {{ props.projects.length ? '没有匹配的项目' : '还没有项目，直接发送即可新建' }}
      </p>
      <ul v-else class="project-list">
        <li v-for="project in filtered" :key="project.id">
          <button type="button" class="project-option" @click="emit('select', project)">
            <strong>{{ project.name }}</strong>
            <span>{{ formatTime(project.updatedAt) }}</span>
          </button>
        </li>
      </ul>
    </section>
  </div>
</template>

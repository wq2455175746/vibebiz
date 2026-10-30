<script setup lang="ts">
import Popover from 'primevue/popover'
import { ref } from 'vue'

import type { Template } from '@/types'

const props = defineProps<{ template: Template }>()

const emit = defineEmits<{
  (event: 'use', template: Template): void
}>()

const menu = ref<InstanceType<typeof Popover> | null>(null)

function toggleMenu(event: Event) {
  menu.value?.toggle(event)
}

function useTemplate() {
  menu.value?.hide()
  emit('use', props.template)
}
</script>

<template>
  <article class="project-card">
    <button class="project-card-open" type="button" @click="useTemplate">
      <span class="project-cover" :class="{ empty: !template.coverUrl }">
        <img v-if="template.coverUrl" :src="template.coverUrl" alt="" />
        <span v-else class="project-cover-empty">
          <i class="pi pi-image" />
          <span>暂无预览图</span>
        </span>
      </span>
      <span class="project-info">
        <strong>{{ template.templateName }}</strong>
        <span>{{ template.parentTopicName }} / {{ template.topicName }}</span>
      </span>
    </button>
    <button
      class="project-menu-trigger"
      type="button"
      aria-label="模板操作"
      @click.stop="toggleMenu"
    >
      <i class="pi pi-ellipsis-h" />
    </button>
    <Popover ref="menu">
      <div class="project-menu">
        <button type="button" @click="useTemplate">
          <i class="pi pi-plus" />
          使用模板
        </button>
      </div>
    </Popover>
  </article>
</template>

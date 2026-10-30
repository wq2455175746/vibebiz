<script setup lang="ts">
import { computed, ref } from 'vue'

import ProjectCard from './ProjectCard.vue'
import type { Project } from '@/types'

const props = defineProps<{ projects: Project[]; limit?: number }>()

const emit = defineEmits<{
  (event: 'open-project', id: string): void
  (event: 'delete-project', project: Project): void
  (event: 'view-all'): void
}>()

type DiscoveryTab = 'projects' | 'templates'

const activeTab = ref<DiscoveryTab>('projects')
const visibleProjects = computed(() =>
  props.limit ? props.projects.slice(0, props.limit) : props.projects,
)

const tabs: { key: DiscoveryTab; label: string }[] = [
  { key: 'projects', label: '我的项目' },
  { key: 'templates', label: '模板' },
]
</script>

<template>
  <section class="home-discovery">
    <div class="discovery-head">
      <div class="discovery-tabs">
        <button
          v-for="tab in tabs"
          :key="tab.key"
          class="discovery-tab"
          :class="{ active: activeTab === tab.key }"
          type="button"
          @click="activeTab = tab.key"
        >
          {{ tab.label }}
        </button>
      </div>
      <button
        v-if="activeTab === 'projects' && limit && projects.length > limit"
        class="discovery-more"
        type="button"
        @click="emit('view-all')"
      >
        查看全部
        <i class="pi pi-arrow-right" />
      </button>
    </div>

    <template v-if="activeTab === 'projects'">
      <p v-if="visibleProjects.length === 0" class="discovery-empty">暂无项目</p>
      <div v-else class="project-grid">
        <ProjectCard
          v-for="project in visibleProjects"
          :key="project.id"
          :project="project"
          @open="emit('open-project', $event)"
          @delete="emit('delete-project', $event)"
        />
      </div>
    </template>

    <p v-else class="discovery-empty">暂无模板</p>
  </section>
</template>

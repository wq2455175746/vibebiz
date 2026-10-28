<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'

import type { Project } from '@/types'
import { formatDateTime } from '@/utils/storage'

defineProps<{ projects: Project[] }>()

type DiscoveryTab = 'projects' | 'templates'

const router = useRouter()
const activeTab = ref<DiscoveryTab>('projects')

const tabs: { key: DiscoveryTab; label: string }[] = [
  { key: 'projects', label: '我的项目' },
  { key: 'templates', label: '模板' },
]

function openProject(id: string) {
  void router.push({ name: 'editor', params: { id } })
}
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
    </div>

    <template v-if="activeTab === 'projects'">
      <p v-if="projects.length === 0" class="discovery-empty">暂无项目</p>
      <div v-else class="project-grid">
        <button
          v-for="project in projects"
          :key="project.id"
          class="project-card"
          type="button"
          @click="openProject(project.id)"
        >
          <span class="project-icon"><i class="pi pi-folder" /></span>
          <span class="project-body">
            <strong>{{ project.name }}</strong>
            <span>{{ formatDateTime(project.createdAt) }}</span>
          </span>
          <i class="pi pi-angle-right project-arrow" />
        </button>
      </div>
    </template>

    <p v-else class="discovery-empty">暂无模板</p>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

import ProjectCard from './ProjectCard.vue'
import type { Project, Template } from '@/types'

const props = withDefaults(
  defineProps<{
    projects: Project[]
    templates?: Template[]
    templatesLoading?: boolean
    templatesError?: string
    limit?: number
  }>(),
  { templates: () => [], templatesLoading: false, templatesError: '' },
)

const emit = defineEmits<{
  (event: 'open-project', id: string): void
  (event: 'delete-project', project: Project): void
  (event: 'view-all'): void
  (event: 'use-template', template: Template): void
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

    <template v-else>
      <p v-if="templatesError" class="error-text">{{ templatesError }}</p>
      <p v-else-if="templatesLoading" class="discovery-empty">加载中...</p>
      <p v-else-if="templates.length === 0" class="discovery-empty">暂无模板</p>
      <div v-else class="project-grid">
        <article v-for="template in templates" :key="template.id" class="template-card">
          <span class="template-cover" :class="{ empty: !template.coverUrl }">
            <img v-if="template.coverUrl" :src="template.coverUrl" alt="" />
            <i v-else class="pi pi-image" />
          </span>
          <div class="template-body">
            <div class="template-title-row">
              <strong>{{ template.templateName }}</strong>
            </div>
            <p class="template-desc">{{ template.description || '暂无描述' }}</p>
            <div class="template-meta">
              <span>{{ template.parentTopicName }} / {{ template.topicName }}</span>
              <button type="button" @click="emit('use-template', template)">使用模板</button>
            </div>
          </div>
        </article>
      </div>
    </template>
  </section>
</template>

<style scoped>
.template-card {
  overflow: hidden;
  background: var(--surface);
  border: 1px solid var(--line);
  border-radius: 16px;
}

.template-cover {
  display: grid;
  place-items: center;
  height: 150px;
  background: var(--soft);
  color: var(--muted);
}

.template-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.template-body {
  padding: 14px;
}

.template-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.template-desc {
  min-height: 36px;
  margin: 8px 0;
  color: var(--muted);
  font-size: 12px;
}

.template-meta {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  color: var(--muted);
  font-size: 12px;
}

.template-meta button {
  padding: 6px 14px;
  background: var(--ink);
  border: none;
  border-radius: 999px;
  color: var(--surface);
  cursor: pointer;
  font-size: 12px;
}
</style>

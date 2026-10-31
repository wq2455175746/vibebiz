<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { deleteStaging } from '@/api/resources'
import { createProject, deleteProject, listProjects } from '@/api/projects'
import { listTemplates, useTemplate as createFromTemplate } from '@/api/templates'
import { fetchChatStatus } from '@/api/chat'
import EnterpriseRoadmap from '@/components/enterprise/EnterpriseRoadmap.vue'
import HomeDiscovery from '@/components/home/HomeDiscovery.vue'
import HomeHero from '@/components/home/HomeHero.vue'
import HomeSidebar from '@/components/home/HomeSidebar.vue'
import ProjectCard from '@/components/home/ProjectCard.vue'
import TemplateCard from '@/components/home/TemplateCard.vue'
import ProjectPickerDialog from '@/components/home/ProjectPickerDialog.vue'
import ScheduledTasksPanel from '@/components/scheduled/ScheduledTasksPanel.vue'
import { useNotificationSocket } from '@/composables/useNotificationSocket'
import { useAuthStore } from '@/stores/auth'
import { useNotificationsStore } from '@/stores/notifications'
import { useResourceStore } from '@/stores/resources'
import {
  DEFAULT_ENTERPRISE_PROMPT,
  markEnterpriseProject,
  resolveEditorPath,
} from '@/utils/enterprise'
import { canAppendToProject, resolveHomeSubmit } from '@/utils/homeSubmit'
import '@/styles/home.css'
import type { Project, Template } from '@/types'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const notifications = useNotificationsStore()
const resources = useResourceStore()
const prompt = ref('')
const creating = ref(false)
const error = ref('')
const projects = ref<Project[]>([])
const templates = ref<Template[]>([])
const templatesLoading = ref(false)
const templatesError = ref('')
const targetProject = ref<Project | null>(null)
const pickerOpen = ref(false)
const sidebarCollapsed = ref(localStorage.getItem('vibebiz:sidebar-collapsed') === '1')
const isProjectsPage = computed(() => route.name === 'projects')
const isResourcesPage = computed(() => route.name === 'resources')
const isScheduledPage = computed(() => route.name === 'scheduled-tasks')
const isEnterprisePage = computed(() => route.name === 'enterprise')
let wasEnterprise = false
let homeDraft = ''
let enterpriseDraft = ''

const { connect, disconnect } = useNotificationSocket(() => {
  void notifications.load(true)
})

watch(
  isEnterprisePage,
  (enterprise) => {
    if (enterprise && !wasEnterprise) {
      homeDraft = prompt.value
      prompt.value = enterpriseDraft || DEFAULT_ENTERPRISE_PROMPT
    } else if (!enterprise && wasEnterprise) {
      enterpriseDraft = prompt.value
      prompt.value = homeDraft
    }
    wasEnterprise = enterprise
  },
  { immediate: true },
)

watch(
  () => auth.isAuthenticated,
  (authenticated) => {
    if (authenticated) {
      connect()
    } else {
      disconnect()
      notifications.clear()
    }
  },
)

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
  localStorage.setItem('vibebiz:sidebar-collapsed', sidebarCollapsed.value ? '1' : '0')
}

async function create(files: File[] = [], agentRoleIds: number[] = []) {
  const text = prompt.value.trim()
  if (!text || creating.value) {
    return
  }
  creating.value = true
  error.value = ''
  const staged: string[] = []
  let projectId = ''
  try {
    const plan = resolveHomeSubmit(targetProject.value?.id ?? null)
    if (plan.mode === 'append') {
      const status = await fetchChatStatus(plan.projectId)
      if (!canAppendToProject(status.active)) {
        error.value = '该项目正在执行，请稍后再试'
        return
      }
      for (const file of files) {
        staged.push(await resources.stage(file))
      }
      const references = []
      for (const resourceId of staged) {
        const item = await resources.adopt(plan.projectId, resourceId)
        references.push({
          kind: 'resource' as const,
          id: item.id,
          name: item.displayName,
          path: item.path,
          contentType: item.contentType,
          resourceKind: item.kind,
        })
      }
      projectId = plan.projectId
      sessionStorage.setItem(`vibebiz:initial-prompt:${plan.projectId}`, text)
      sessionStorage.setItem(
        `vibebiz:initial-references:${plan.projectId}`,
        JSON.stringify(references),
      )
      sessionStorage.setItem(`vibebiz:initial-append:${plan.projectId}`, '1')
      void router.push({
        name: 'editor',
        params: { id: plan.projectId },
        state: { initialPrompt: text },
      })
      return
    }

    for (const file of files) {
      staged.push(await resources.stage(file))
    }
    const project = await createProject(
      text,
      agentRoleIds,
      isEnterprisePage.value ? 'enterprise' : undefined,
    )
    projectId = project.id
    if (isEnterprisePage.value) {
      markEnterpriseProject(project.id)
    }
    const references = []
    for (const resourceId of staged) {
      const item = await resources.adopt(project.id, resourceId)
      references.push({
        kind: 'resource' as const,
        id: item.id,
        name: item.displayName,
        path: item.path,
        contentType: item.contentType,
        resourceKind: item.kind,
      })
    }
    sessionStorage.setItem(`vibebiz:initial-prompt:${project.id}`, text)
    sessionStorage.setItem(`vibebiz:initial-references:${project.id}`, JSON.stringify(references))
    void router.push({
      name: isEnterprisePage.value ? 'enterprise-editor' : 'editor',
      params: { id: project.id },
      state: { initialPrompt: text },
    })
  } catch (err) {
    if (!projectId) {
      await Promise.allSettled(staged.map((id) => deleteStaging(id)))
    }
    error.value = (err as Error).message
  } finally {
    creating.value = false
  }
}

function openProjectPicker() {
  pickerOpen.value = true
  void loadProjects()
}

function selectProject(project: Project) {
  targetProject.value = project
  pickerOpen.value = false
}

async function loadProjects() {
  try {
    projects.value = await listProjects()
  } catch {
    projects.value = []
  }
}

async function loadTemplates() {
  templatesLoading.value = true
  templatesError.value = ''
  try {
    const result = await listTemplates({ page: 1, pageSize: 12, sort: 'latest' })
    templates.value = result.templates
  } catch (err) {
    templatesError.value = err instanceof Error ? err.message : '加载模板失败'
    templates.value = []
  } finally {
    templatesLoading.value = false
  }
}

async function useTemplate(template: Template) {
  error.value = ''
  try {
    const project = await createFromTemplate(template.id)
    await router.push({ name: 'editor', params: { id: project.id } })
  } catch (err) {
    error.value = err instanceof Error ? err.message : '使用模板失败'
  }
}

function openProject(id: string) {
  const project = projects.value.find((item) => item.id === id)
  if (project && resolveEditorPath(project).startsWith('/enterprise/')) {
    void router.push(`/enterprise/editor/${id}`)
    return
  }
  void router.push({ name: 'editor', params: { id } })
}

async function removeProject(project: Project) {
  if (!window.confirm(`确定删除项目「${project.name}」吗？`)) {
    return
  }
  try {
    await deleteProject(project.id)
    projects.value = projects.value.filter((item) => item.id !== project.id)
  } catch (err) {
    error.value = err instanceof Error ? err.message : '删除项目失败'
  }
}

onMounted(async () => {
  resources.reset()
  await auth.ensure()
  if (auth.isAuthenticated) {
    connect()
  }
  await Promise.allSettled([loadProjects(), loadTemplates()])
})
</script>

<template>
  <div class="home-shell" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <HomeSidebar
      :recent-projects="projects.slice(0, 10)"
      :collapsed="sidebarCollapsed"
      @toggle-collapse="toggleSidebar"
      @workspace-changed="loadProjects"
    />

    <main class="home-main">
      <div v-if="isProjectsPage" class="projects-page">
        <header class="projects-head">
          <h1>我的项目</h1>
          <span>{{ projects.length }} 个项目</span>
        </header>
        <p v-if="error" class="error-text">{{ error }}</p>
        <p v-if="projects.length === 0" class="discovery-empty">暂无项目</p>
        <div v-else class="project-grid">
          <ProjectCard
            v-for="project in projects"
            :key="project.id"
            :project="project"
            @open="openProject"
            @delete="removeProject"
          />
        </div>
      </div>

      <div v-else-if="isResourcesPage" class="projects-page">
        <header class="projects-head">
          <h1>资源 · 模板</h1>
          <span>{{ templates.length }} 个模板</span>
        </header>
        <p v-if="templatesError" class="error-text">{{ templatesError }}</p>
        <p v-else-if="templatesLoading" class="discovery-empty">加载中...</p>
        <p v-else-if="templates.length === 0" class="discovery-empty">暂无模板</p>
        <div v-else class="project-grid">
          <TemplateCard
            v-for="template in templates"
            :key="template.id"
            :template="template"
            @use="useTemplate"
          />
        </div>
      </div>

      <div v-else-if="isEnterprisePage" class="enterprise-home">
        <HomeHero
          v-model="prompt"
          title="企业工程开发"
          subtitle="由团队角色按阶段完成需求梳理、设计、测试与开发，每一步确认后才会继续。"
          placeholder="描述你的企业级工程需求…"
          composer-hint="企业模式将自动加载 Harness 工程模板 · Ctrl / ⌘ + Enter 发送"
          required-role-code="team"
          :input-rows="5"
          :creating="creating"
          :error="error"
          @submit="create"
        />
        <EnterpriseRoadmap />
        <p class="enterprise-home-note">
          <i class="pi pi-info-circle" />
          企业项目会强制使用团队角色，并按阶段锁定工作区。
        </p>
      </div>

      <ScheduledTasksPanel v-else-if="isScheduledPage" />

      <div v-else class="home-main-inner">
        <HomeHero
          v-model="prompt"
          :creating="creating"
          :error="error"
          :target-project="targetProject"
          @submit="create"
          @select-project="openProjectPicker"
          @clear-target="targetProject = null"
        />
        <HomeDiscovery
          :projects="projects"
          :templates="templates"
          :templates-loading="templatesLoading"
          :templates-error="templatesError"
          :limit="4"
          @open-project="openProject"
          @delete-project="removeProject"
          @view-all="router.push({ name: 'projects' })"
          @use-template="useTemplate"
        />
      </div>
    </main>

    <ProjectPickerDialog
      v-if="pickerOpen"
      :projects="projects"
      @close="pickerOpen = false"
      @select="selectProject"
    />
  </div>
</template>

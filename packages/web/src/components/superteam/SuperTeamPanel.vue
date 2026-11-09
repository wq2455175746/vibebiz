<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'

import { createProject, listProjects } from '@/api/projects'
import { createScheduledTask, listScheduledTasks } from '@/api/scheduledTasks'
import EventSourceCard from '@/components/superteam/EventSourceCard.vue'
import EventSourceDialog from '@/components/superteam/EventSourceDialog.vue'
import StepCard from '@/components/superteam/StepCard.vue'
import { useAgentRolesStore } from '@/stores/agentRoles'
import { useSuperTeamStore } from '@/stores/superTeam'
import type { Project, ScheduledTask } from '@/types'
import type { EventSourceConfig, EventSourceKind, SuperStepKey } from '@/types/superTeam'
import { markEnterpriseProject } from '@/utils/enterprise'
import '@/styles/superteam.css'

const REQUIREMENTS_PROMPT =
  '你是超级团队的需求挖掘 Agent。基于已接入的飞书、Webhook、Email 事件，生成需求思维导图、知识图谱与经济可行性分析文档，并写入当前项目工作区。'
const DEVELOPMENT_PROMPT =
  '你是超级团队的系统开发 Agent。请基于需求挖掘产物，按企业七阶段推进需求梳理、概要设计、详细设计、测试用例、全栈开发、制品与交付。'
const OPERATIONS_TASK = {
  name: '超级团队观测运维',
  description: '每小时巡检超级团队事件与指标，生成可预览的数据图表与指标报表。',
  scheduleType: 'cron' as const,
  time: '',
  weekdays: [],
  cronExpression: '0 * * * *',
  timezone: 'Asia/Shanghai',
  source: 'manual' as const,
}

const router = useRouter()
const store = useSuperTeamStore()
const agentRoles = useAgentRolesStore()

const dialogVisible = ref(false)
const activeSource = ref<EventSourceConfig | null>(null)
const projects = ref<Project[]>([])
const tasks = ref<ScheduledTask[]>([])
const pending = ref<SuperStepKey | null>(null)
const actionError = ref('')

const connected = computed(
  () => store.sources.filter((source) => source.status === 'connected').length,
)
const routed = computed(() => store.events.filter((event) => event.stage === 'routed').length)

async function loadBindings(): Promise<void> {
  const [projectResult, taskResult] = await Promise.allSettled([
    listProjects(),
    listScheduledTasks(),
  ])
  projects.value = projectResult.status === 'fulfilled' ? projectResult.value : []
  tasks.value = taskResult.status === 'fulfilled' ? taskResult.value : []
  store.reconcile(projects.value, tasks.value)
}

function openSource(source: EventSourceConfig): void {
  if (source.status === 'coming_soon') {
    return
  }
  activeSource.value = source
  dialogVisible.value = true
}

function saveSource(kind: EventSourceKind, fields: Record<string, string>): void {
  store.updateSource(kind, { fields, enabled: true })
}

async function startStep(key: SuperStepKey): Promise<void> {
  const step = store.steps.find((item) => item.key === key)
  if (!step || step.status === 'locked' || pending.value) {
    return
  }
  pending.value = key
  actionError.value = ''
  try {
    if (key === 'requirements') {
      let id = store.bindings.requirementsProjectId
      if (!id) {
        const project = await createProject(REQUIREMENTS_PROMPT)
        id = project.id
        store.setBinding('requirementsProjectId', id)
      }
      sessionStorage.setItem(`vibebiz:initial-prompt:${id}`, REQUIREMENTS_PROMPT)
      await router.push({
        name: 'editor',
        params: { id },
        query: { from: 'super-team', step: 'requirements' },
      })
      return
    }
    if (key === 'development') {
      let id = store.bindings.developmentProjectId
      if (!id) {
        await agentRoles.load()
        const team = agentRoles.team
        if (!team) {
          actionError.value = '缺少团队角色，无法创建企业级项目'
          return
        }
        const project = await createProject(DEVELOPMENT_PROMPT, [team.id], 'enterprise')
        markEnterpriseProject(project.id)
        id = project.id
        store.setBinding('developmentProjectId', id)
      }
      sessionStorage.setItem(`vibebiz:initial-prompt:${id}`, DEVELOPMENT_PROMPT)
      await router.push({
        name: 'enterprise-editor',
        params: { id },
        query: { from: 'super-team', step: 'development' },
      })
      return
    }
    let task = tasks.value.find((item) => item.id === store.bindings.operationsTaskId)
    if (!task) {
      task = await createScheduledTask(OPERATIONS_TASK)
      store.setBinding('operationsTaskId', task.id)
      store.setBinding('operationsProjectId', task.projectId)
      tasks.value = [task, ...tasks.value]
    }
    await router.push({
      name: 'scheduled-tasks',
      query: { from: 'super-team', step: 'operations', taskId: String(task.id) },
    })
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '创建失败'
  } finally {
    pending.value = null
  }
}

onMounted(loadBindings)
</script>

<template>
  <section class="superteam-page">
    <header class="superteam-head">
      <p class="superteam-eyebrow">SUPER TEAM</p>
      <h1>超级团队</h1>
      <p class="superteam-subtitle">
        由事件驱动的多 Agent 团队，覆盖需求挖掘、系统开发与观测运维。
      </p>
      <div class="superteam-stats">
        <span
          ><strong>{{ store.events.length }}</strong> 今日事件</span
        >
        <span
          ><strong>{{ connected }}</strong> 已连接入口</span
        >
        <span
          ><strong>{{ routed }}</strong> 已路由</span
        >
      </div>
    </header>

    <p v-if="actionError" class="superteam-error" role="alert">{{ actionError }}</p>

    <section class="superteam-section">
      <div class="superteam-section-head">
        <h2>事件入口</h2>
        <span>配置归属当前用户，ops-web 只读</span>
      </div>
      <div class="superteam-source-grid">
        <EventSourceCard
          v-for="source in store.sources"
          :key="source.kind"
          :source="source"
          @configure="openSource(source)"
        />
      </div>
    </section>

    <section class="superteam-section">
      <div class="superteam-section-head">
        <h2>执行流程</h2>
        <span>必须按顺序完成</span>
      </div>
      <div class="superteam-step-row">
        <StepCard
          v-for="(step, index) in store.steps"
          :key="step.key"
          :step="step"
          :index="index"
          :busy="pending === step.key"
          @open="startStep(step.key)"
        />
      </div>
    </section>

    <EventSourceDialog v-model:visible="dialogVisible" :source="activeSource" @save="saveSource" />
  </section>
</template>

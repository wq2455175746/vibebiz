<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'metrics' | 'reports' | 'tasks' | 'events'

interface MetricCard {
  title: string
  value: string
  caption: string
  values: number[]
}

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('metrics')

const step = computed(() => store.steps.find((item) => item.key === 'operations'))

const metrics: MetricCard[] = [
  { title: '今日事件', value: '128', caption: '较昨日 +12%', values: [18, 24, 20, 32, 28, 36, 42] },
  { title: '有效事件', value: '36', caption: '噪音过滤后', values: [6, 8, 7, 12, 10, 14, 16] },
  { title: '已路由', value: '24', caption: '进入三步流程', values: [4, 5, 6, 8, 7, 9, 11] },
  { title: '告警', value: '3', caption: '待处理', values: [2, 1, 3, 2, 1, 2, 3] },
]

const tasks = [
  {
    name: '每小时巡检',
    schedule: '每 1 小时',
    status: '启用',
    lastRun: '2026-11-09 09:00',
    nextRun: '2026-11-09 10:00',
  },
  {
    name: '每日指标汇总',
    schedule: '每日 08:00',
    status: '启用',
    lastRun: '2026-11-09 08:00',
    nextRun: '2026-11-10 08:00',
  },
  {
    name: '周报生成',
    schedule: '每周一 09:00',
    status: '暂停',
    lastRun: '2026-11-03 09:00',
    nextRun: '2026-11-10 09:00',
  },
]

const reports = [
  { name: '支付可用性日报', updatedAt: '2026-11-09 09:00', status: '可预览' },
  { name: '需求转化周报', updatedAt: '2026-11-08 18:00', status: '可预览' },
  { name: '运维告警月报', updatedAt: '2026-11-01 09:00', status: '可预览' },
]

const events = computed(() => store.events)

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('operations')
  back()
}

function linePoints(values: number[], width = 300, height = 80): string {
  const max = Math.max(...values, 1)
  return values
    .map((value, index) => {
      const x = values.length === 1 ? 0 : (index / (values.length - 1)) * width
      const y = height - (value / max) * height
      return `${x.toFixed(1)},${y.toFixed(1)}`
    })
    .join(' ')
}

function areaPoints(values: number[], width = 300, height = 80): string {
  return `0,${height} ${linePoints(values, width, height)} ${width},${height}`
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="观测运维"
      :project-state="step?.status ?? 'ready'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">运维 Agent</span>
          默认每 1 小时巡检一次事件源与指标，异常时生成可预览报表。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">最近一次巡检</span>
          2026-11-09 09:00 · 发现 3 条告警 · 已生成支付可用性日报。
        </div>
        <div class="super-task-list">
          <div v-for="task in tasks" :key="task.name" class="super-task-item">
            <div>
              <strong>{{ task.name }}</strong>
              <small>{{ task.schedule }} · 上次 {{ task.lastRun }}</small>
            </div>
            <span class="tag">{{ task.status }}</span>
          </div>
        </div>
      </aside>

      <main class="superteam-workbench">
        <div class="superteam-tabs">
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'metrics' }"
            @click="tab = 'metrics'"
          >
            指标看板
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'reports' }"
            @click="tab = 'reports'"
          >
            可预览报表
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'tasks' }"
            @click="tab = 'tasks'"
          >
            定时任务
          </button>
          <button
            type="button"
            class="superteam-tab"
            :class="{ active: tab === 'events' }"
            @click="tab = 'events'"
          >
            事件源
          </button>
        </div>

        <div class="superteam-tab-body">
          <div v-if="tab === 'metrics'" class="super-chart-grid">
            <article v-for="metric in metrics" :key="metric.title" class="super-chart-card">
              <h4>{{ metric.title }}</h4>
              <span>{{ metric.caption }}</span>
              <strong>{{ metric.value }}</strong>
              <svg viewBox="0 0 300 80" preserveAspectRatio="none" aria-hidden="true">
                <polygon class="super-chart-area" :points="areaPoints(metric.values)" />
                <polyline class="super-chart-line" :points="linePoints(metric.values)" />
              </svg>
            </article>
          </div>

          <div v-else-if="tab === 'reports'" class="super-task-list">
            <div v-for="report in reports" :key="report.name" class="super-task-item">
              <div>
                <strong>{{ report.name }}</strong>
                <small>更新于 {{ report.updatedAt }}</small>
              </div>
              <span class="tag">{{ report.status }}</span>
            </div>
          </div>

          <div v-else-if="tab === 'tasks'" class="super-task-list">
            <div v-for="task in tasks" :key="task.name" class="super-task-item">
              <div>
                <strong>{{ task.name }}</strong>
                <small>{{ task.schedule }} · 下次 {{ task.nextRun }}</small>
              </div>
              <span class="tag">{{ task.status }}</span>
            </div>
          </div>

          <div v-else class="super-event-list">
            <article v-for="event in events" :key="event.id" class="super-event-row">
              <span class="tag">{{ event.source }}</span>
              <div>
                <strong>{{ event.title }}</strong>
                <small>{{ event.id }}</small>
              </div>
              <div class="super-event-scores">
                <span>噪音 {{ event.noiseScore }}</span>
                <span>语义 {{ event.semanticScore }}</span>
              </div>
            </article>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后整个超级团队流程结束。</p>
          <button
            class="superteam-primary"
            type="button"
            :disabled="step?.status === 'locked'"
            @click="complete"
          >
            完成全流程
          </button>
        </div>
      </main>
    </div>
  </div>
</template>

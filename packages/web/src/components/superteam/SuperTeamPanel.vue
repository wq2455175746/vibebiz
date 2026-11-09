<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EventSourceCard from '@/components/superteam/EventSourceCard.vue'
import EventSourceDialog from '@/components/superteam/EventSourceDialog.vue'
import StepCard from '@/components/superteam/StepCard.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import type { EventSourceConfig, EventSourceKind, SuperStepKey } from '@/types/superTeam'
import '@/styles/superteam.css'

const router = useRouter()
const store = useSuperTeamStore()

const dialogVisible = ref(false)
const activeSource = ref<EventSourceConfig | null>(null)

const connected = computed(
  () => store.sources.filter((source) => source.status === 'connected').length,
)
const routed = computed(() => store.events.filter((event) => event.stage === 'routed').length)

const stepRoutes: Record<SuperStepKey, string> = {
  requirements: 'super-team-requirements',
  development: 'super-team-development',
  operations: 'super-team-operations',
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

function openStep(key: SuperStepKey): void {
  void router.push({ name: stepRoutes[key] })
}
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
          @open="openStep(step.key)"
        />
      </div>
    </section>

    <EventSourceDialog v-model:visible="dialogVisible" :source="activeSource" @save="saveSource" />
  </section>
</template>

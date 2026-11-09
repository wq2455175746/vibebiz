<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'preview' | 'code' | 'terminal'

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('preview')

const step = computed(() => store.steps.find((item) => item.key === 'development'))

const stages = [
  { key: 'requirements', title: '需求梳理', status: 'done' },
  { key: 'architecture', title: '概要设计', status: 'done' },
  { key: 'detail', title: '详细设计', status: 'active' },
  { key: 'test', title: '测试用例', status: 'pending' },
  { key: 'development', title: '全栈开发', status: 'pending' },
  { key: 'artifacts', title: '制品空间', status: 'pending' },
  { key: 'delivery', title: '交付空间', status: 'pending' },
]

const injectedEvents = computed(() =>
  store.events.filter((event) => event.stage === 'routed' || event.stage === 'evaluated'),
)

const tabs: { key: TabKey; label: string }[] = [
  { key: 'preview', label: '应用查看器' },
  { key: 'code', label: '代码' },
  { key: 'terminal', label: '终端' },
]

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('development')
  back()
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="系统开发"
      :project-state="step?.status ?? 'ready'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="super-stage-bar">
      <span
        v-for="stage in stages"
        :key="stage.key"
        class="super-stage-chip"
        :class="{ 'is-done': stage.status === 'done', 'is-active': stage.status === 'active' }"
      >
        {{ stage.title }}
      </span>
    </div>

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">架构师</span>
          已采纳需求挖掘产出的支付可用性需求，正在拆解支付网关与重试策略。
        </div>
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">测试工程师</span>
          已补充支付失败与导出超时的回归用例。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">事件注入</span>
          当前有 {{ injectedEvents.length }} 条事件进入开发流程，均已通过噪音过滤与语义评估。
        </div>
        <div class="superteam-chat-input">
          <input type="text" placeholder="本轮为演示效果，输入已禁用" disabled />
        </div>
      </aside>

      <main class="superteam-workbench">
        <div class="superteam-tabs">
          <button
            v-for="item in tabs"
            :key="item.key"
            type="button"
            class="superteam-tab"
            :class="{ active: tab === item.key }"
            @click="tab = item.key"
          >
            {{ item.label }}
          </button>
        </div>

        <div class="superteam-tab-body">
          <div v-if="tab === 'preview'" class="super-doc">
            <h3>应用查看器（演示）</h3>
            <p>此处复用编辑器预览区效果；本轮为静态占位，不加载真实项目。</p>
          </div>
          <div v-else-if="tab === 'code'" class="super-doc">
            <h3>代码（演示）</h3>
            <p>后续接入真实工作区文件树与只读代码视图。</p>
          </div>
          <div v-else class="super-doc">
            <h3>终端（演示）</h3>
            <p>构建、测试与运行日志将在这里实时输出。</p>
          </div>

          <div class="super-inject-panel">
            <h4>事件注入</h4>
            <div v-for="event in injectedEvents" :key="event.id" class="super-inject-item">
              <span>{{ event.title }}</span>
              <span class="tag">已进入开发</span>
            </div>
            <p v-if="injectedEvents.length === 0" class="superteam-subtitle">
              暂无事件进入开发流程。
            </p>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后解锁「观测运维」。</p>
          <button
            class="superteam-primary"
            type="button"
            :disabled="step?.status === 'locked'"
            @click="complete"
          >
            完成本阶段，解锁观测运维
          </button>
        </div>
      </main>
    </div>
  </div>
</template>

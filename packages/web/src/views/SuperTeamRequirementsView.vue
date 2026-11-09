<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'

import EditorHeader from '@/components/editor/EditorHeader.vue'
import KnowledgeGraph from '@/components/superteam/KnowledgeGraph.vue'
import MindMap from '@/components/superteam/MindMap.vue'
import { useSuperTeamStore } from '@/stores/superTeam'
import type { SuperEvent } from '@/types/superTeam'
import { eventStageLabel, noiseBand } from '@/utils/superTeam'
import '@/styles/editor.css'
import '@/styles/superteam.css'

type TabKey = 'mindmap' | 'graph' | 'doc' | 'events'

const router = useRouter()
const store = useSuperTeamStore()
const tab = ref<TabKey>('mindmap')

const tabs: { key: TabKey; label: string }[] = [
  { key: 'mindmap', label: '思维导图' },
  { key: 'graph', label: '知识图谱' },
  { key: 'doc', label: '可行性文档' },
  { key: 'events', label: '事件流' },
]

const step = computed(() => store.steps.find((item) => item.key === 'requirements'))
const events = computed(() => store.events)

function back(): void {
  void router.push({ name: 'super-team' })
}

function complete(): void {
  store.advance('requirements')
  back()
}

function eventClass(event: SuperEvent): Record<string, boolean> {
  return { 'is-ignored': event.stage === 'ignored' || noiseBand(event.noiseScore) === 'high' }
}

function formatTime(value: string): string {
  return new Date(value).toLocaleString('zh-CN')
}
</script>

<template>
  <div class="superteam-editor">
    <EditorHeader
      project-name="需求挖掘"
      :project-state="step?.status ?? 'active'"
      model-label="DeepSeek Flash"
      :roles="[]"
      :show-actions="false"
      @back="back"
    />

    <div class="superteam-editor-body">
      <aside class="superteam-chat">
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">需求分析师</span>
          已接收 128 条事件，过滤 92 条噪音；当前聚焦支付失败与导出超时两类问题。
        </div>
        <div class="superteam-chat-bubble agent">
          <span class="superteam-chat-role">产品经理</span>
          已生成需求思维导图与知识图谱，正在补充经济可行性分析。
        </div>
        <div class="superteam-chat-bubble">
          <span class="superteam-chat-role">事件摘要</span>
          支付失败影响下单转化，建议优先排查支付网关可用性。
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
          <MindMap v-if="tab === 'mindmap'" />
          <KnowledgeGraph v-else-if="tab === 'graph'" />
          <div v-else-if="tab === 'doc'" class="super-doc">
            <h3>经济可行性分析</h3>
            <p>
              事件驱动需求挖掘可把客户反馈、监控告警与邮件线索统一收敛，减少人工整理成本。 以当前
              128 条/日事件量估算，噪音过滤可减少约 70% 的无效阅读。
            </p>
            <ul>
              <li>市场规模：企业协作与运维自动化持续增长。</li>
              <li>成本结构：模型调用 + 人工确认，按事件量线性增长。</li>
              <li>收益：缩短需求响应时间，提升续费率与客户满意度。</li>
              <li>风险：数据隐私、误判与模型成本需要持续监控。</li>
            </ul>
            <p>建议先以小范围事件源试点，验证噪音阈值与语义路由准确率后再扩大接入。</p>
          </div>
          <div v-else class="super-event-list">
            <article
              v-for="event in events"
              :key="event.id"
              class="super-event-row"
              :class="eventClass(event)"
            >
              <span class="tag">{{ event.source }}</span>
              <div>
                <strong>{{ event.title }}</strong>
                <small>{{ event.id }} · {{ formatTime(event.receivedAt) }}</small>
              </div>
              <div class="super-event-scores">
                <span>噪音 {{ event.noiseScore }}</span>
                <span>语义 {{ event.semanticScore }}</span>
                <span>{{ eventStageLabel(event.stage) }}</span>
              </div>
            </article>
          </div>
        </div>

        <div class="superteam-footer-action">
          <p>完成后解锁「系统开发」。</p>
          <button class="superteam-primary" type="button" @click="complete">
            完成本阶段，解锁系统开发
          </button>
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { listChannelGuides } from '@/api/channelPackages'
import type { ChannelGuide } from '@/types'
import { renderMarkdown } from '@/utils/markdown'

const route = useRoute()
const guides = ref<ChannelGuide[]>([])
const selectedChannel = ref('')
const loading = ref(true)
const loadError = ref('')

const selected = computed<ChannelGuide | null>(() => {
  return guides.value.find((guide) => guide.channel === selectedChannel.value) ?? null
})

const renderedMarkdown = computed(() =>
  selected.value ? renderMarkdown(selected.value.markdown) : '',
)

function requestedChannel() {
  const query = route.query.channel
  return typeof query === 'string' ? query : ''
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    guides.value = await listChannelGuides()
    const requested = requestedChannel()
    const match = guides.value.find((guide) => guide.channel === requested)
    selectedChannel.value = match?.channel ?? guides.value[0]?.channel ?? ''
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '发布渠道指引加载失败'
  } finally {
    loading.value = false
  }
}

function select(channel: string) {
  selectedChannel.value = channel
}

onMounted(load)
</script>

<template>
  <section class="settings-page">
    <header class="settings-head">
      <div>
        <h1>发布渠道指引</h1>
        <p>查看微信小游戏、抖音小游戏和 Steam 桌面渠道的接入要求与发布步骤。</p>
      </div>
    </header>

    <p v-if="loading" class="settings-state">加载中…</p>
    <p v-else-if="loadError" class="settings-error" role="alert">{{ loadError }}</p>
    <div v-else-if="guides.length === 0" class="settings-state channel-guide-empty">
      <i class="pi pi-share-alt" />
      <strong>暂无发布渠道指引</strong>
      <span>当前没有可用的渠道接入说明。</span>
    </div>

    <div v-else class="channel-guide-layout">
      <nav class="channel-guide-nav" aria-label="发布渠道">
        <button
          v-for="guide in guides"
          :key="guide.channel"
          class="channel-guide-tab"
          :class="{ active: guide.channel === selected?.channel }"
          :aria-pressed="guide.channel === selected?.channel"
          type="button"
          @click="select(guide.channel)"
        >
          <span>{{ guide.title }}</span>
          <small>{{ guide.channel }}</small>
        </button>
      </nav>

      <article v-if="selected" class="channel-guide-panel">
        <header class="channel-guide-head">
          <div>
            <h2>{{ selected.title }}</h2>
            <p>{{ selected.summary }}</p>
          </div>
          <span v-if="selected.updatedAt" class="channel-guide-updated">
            更新于 {{ selected.updatedAt }}
          </span>
        </header>

        <section v-if="selected.requirements.length > 0" class="channel-guide-section">
          <h3>资质要求</h3>
          <ul class="channel-guide-requirements">
            <li v-for="item in selected.requirements" :key="item">
              <i class="pi pi-check" />
              <span>{{ item }}</span>
            </li>
          </ul>
        </section>

        <section v-if="selected.steps.length > 0" class="channel-guide-section">
          <h3>发布流程</h3>
          <ol class="channel-guide-steps">
            <li v-for="(step, index) in selected.steps" :key="step" class="channel-guide-step">
              <span class="channel-guide-step-index">{{ index + 1 }}</span>
              <span class="channel-guide-step-label">{{ step }}</span>
            </li>
          </ol>
        </section>

        <section class="channel-guide-section">
          <h3>接入说明</h3>
          <div class="channel-guide-markdown" v-html="renderedMarkdown" />
        </section>
      </article>
    </div>
  </section>
</template>

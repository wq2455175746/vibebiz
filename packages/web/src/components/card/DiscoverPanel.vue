<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import CardTile from './CardTile.vue'
import { useCardStore } from '@/stores/card'
import type { CardFeatured } from '@/types'
import '@/styles/card.css'

const store = useCardStore()

const finished = computed(
  () =>
    store.discoverTotal === 0 || store.discoverPage * store.discoverPageSize >= store.discoverTotal,
)
const error = computed(() => store.error)

async function loadMore() {
  if (store.discoverLoading || finished.value) return
  const next = store.discoverCards.length === 0 ? 1 : store.discoverPage + 1
  try {
    await store.loadDiscover(next, next > 1)
  } catch {
    // error is surfaced through store.error
  }
}

function open(id: string) {
  window.open(`/share/card/${encodeURIComponent(id)}`, '_blank', 'noopener,noreferrer')
}

function openFeatured(id: string) {
  const card = store.discoverCards.find((item) => item.id === id)
  if (!card) return
  const featured: CardFeatured =
    card.featured ??
    (card.items[0] ? { kind: 'project', projectId: card.items[0].projectId } : null)
  if (!featured) return
  if (featured.kind === 'project') {
    window.open(
      `/share/project/${encodeURIComponent(featured.projectId)}`,
      '_blank',
      'noopener,noreferrer',
    )
    return
  }
  window.open(`/share/card/${encodeURIComponent(card.id)}/file`, '_blank', 'noopener,noreferrer')
}

const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  void store.loadDiscover(1, false).catch(() => undefined)
  if (typeof IntersectionObserver !== 'undefined' && sentinel.value) {
    observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) void loadMore()
    })
    observer.observe(sentinel.value)
  }
})

onBeforeUnmount(() => observer?.disconnect())
</script>

<template>
  <div class="discover-page">
    <header class="discover-head">
      <div>
        <h1>发现 · 编码伙伴</h1>
        <p>看看其他开发者正在做什么，找到同频的人。</p>
      </div>
      <span class="tag">{{ store.discoverCards.length }} 张名片</span>
    </header>

    <p v-if="error" class="error-text">{{ error }}</p>
    <p
      v-if="!store.discoverLoading && store.discoverCards.length === 0 && store.discoverTotal === 0"
      class="card-empty"
    >
      还没有人发布名片。
    </p>

    <div class="discover-grid">
      <CardTile
        v-for="card in store.discoverCards"
        :key="card.id"
        :card="card"
        @open="open"
        @open-featured="openFeatured"
      />
    </div>

    <div ref="sentinel" class="discover-sentinel" />
    <div class="discover-more-wrap">
      <button
        v-if="!finished || store.discoverLoading"
        class="discover-more"
        type="button"
        :disabled="store.discoverLoading"
        @click="loadMore"
      >
        {{ store.discoverLoading ? '加载中…' : '加载更多' }}
      </button>
      <span v-else-if="store.discoverTotal > 0" class="discover-end">已到底</span>
    </div>
  </div>
</template>

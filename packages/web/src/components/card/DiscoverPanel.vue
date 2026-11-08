<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

import CardTile from './CardTile.vue'
import { useCardStore } from '@/stores/card'
import type { BusinessCard } from '@/types'
import '@/styles/card.css'

const store = useCardStore()
const cards = ref<BusinessCard[]>([])
const page = ref(1)
const loading = ref(false)
const finished = ref(false)
const error = ref('')

async function loadMore() {
  if (loading.value || finished.value) return
  loading.value = true
  error.value = ''
  try {
    await new Promise((resolve) => setTimeout(resolve, 300))
    const result = store.listDiscover(page.value, 6)
    cards.value = [...cards.value, ...result.items]
    finished.value = cards.value.length >= result.total
    page.value += 1
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载名片失败'
  } finally {
    loading.value = false
  }
}

function open(id: string) {
  window.open(`/share/card/${encodeURIComponent(id)}`, '_blank', 'noopener,noreferrer')
}

const sentinel = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  store.load()
  void loadMore()
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
      <span class="tag">{{ cards.length }} 张名片</span>
    </header>

    <p v-if="error" class="error-text">{{ error }}</p>
    <p v-if="!loading && cards.length === 0" class="card-empty">还没有人发布名片。</p>

    <div class="discover-grid">
      <CardTile v-for="card in cards" :key="card.id" :card="card" @open="open" />
    </div>

    <div ref="sentinel" class="discover-sentinel" />
    <div class="discover-more-wrap">
      <button
        v-if="!finished"
        class="discover-more"
        type="button"
        :disabled="loading"
        @click="loadMore"
      >
        {{ loading ? '加载中…' : '加载更多' }}
      </button>
      <span v-else class="discover-end">已到底</span>
    </div>
  </div>
</template>

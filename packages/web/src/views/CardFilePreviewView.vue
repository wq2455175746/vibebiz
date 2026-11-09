<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { getPublicCard } from '@/api/cards'
import { useBlobUrl } from '@/composables/useBlobUrl'
import type { BusinessCard } from '@/types'
import '@/styles/card.css'

const route = useRoute()
const card = ref<BusinessCard | null>(null)
const loading = ref(true)
const loadError = ref('')

onMounted(async () => {
  try {
    card.value = await getPublicCard(String(route.params.id ?? ''))
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '预览加载失败'
  } finally {
    loading.value = false
  }
})

const featured = computed(() =>
  card.value?.featured?.kind === 'file' ? card.value.featured : null,
)
const pdfSource = computed(() => (featured.value?.fileType === 'pdf' ? featured.value.url : ''))
const pdfUrl = useBlobUrl(pdfSource)
const backHref = computed(() =>
  card.value ? `/share/card/${encodeURIComponent(card.value.id)}` : '/',
)
</script>

<template>
  <main class="share-page">
    <div v-if="loading" class="share-empty">
      <h1>加载中…</h1>
    </div>
    <div v-else-if="loadError || !featured" class="share-empty">
      <h1>{{ loadError || '暂无可预览内容' }}</h1>
      <a class="card-primary" :href="backHref">返回名片</a>
    </div>
    <template v-else>
      <header class="share-head">
        <a class="share-back" :href="backHref">← 返回名片</a>
        <div>
          <h1>{{ featured.name }}</h1>
          <p v-if="card?.ownerName">来自 {{ card.ownerName }} 的名片</p>
        </div>
      </header>

      <section class="share-stage">
        <img v-if="featured.fileType === 'image'" :src="featured.url" :alt="featured.name" />
        <iframe v-else :src="pdfUrl" title="PDF 预览" referrerpolicy="no-referrer" />
      </section>
    </template>
  </main>
</template>

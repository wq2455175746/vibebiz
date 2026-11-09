<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'

import { getPublicCardProject } from '@/api/cards'
import { useBlobUrl } from '@/composables/useBlobUrl'
import type { CardContentType, PublicProjectCard } from '@/types'
import { CARD_TYPE_LABELS } from '@/utils/card'
import { renderMarkdown } from '@/utils/markdown'
import '@/styles/card.css'

const route = useRoute()
const project = ref<PublicProjectCard | null>(null)
const loading = ref(true)
const loadError = ref('')

onMounted(async () => {
  try {
    project.value = await getPublicCardProject(String(route.params.projectId ?? ''))
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '项目加载失败'
  } finally {
    loading.value = false
  }
})

const item = computed(() => project.value?.item ?? null)
const card = computed(() => project.value?.card ?? null)
const type = computed<CardContentType>(() => item.value?.type ?? 'html')
const htmlSource = computed(() => {
  const url = item.value?.shareUrl ?? ''
  return url.startsWith('http') || url.startsWith('/p/') ? url : ''
})
const mediaSource = computed(() => item.value?.previewUrl ?? '')
const pdfSource = computed(() => (type.value === 'pdf' ? mediaSource.value : ''))
const pdfUrl = useBlobUrl(pdfSource)
const documentHTML = computed(() =>
  item.value?.previewUrl ? renderMarkdown(item.value.previewUrl) : '',
)
</script>

<template>
  <main class="share-page">
    <div v-if="loading" class="share-empty">
      <h1>加载中…</h1>
    </div>
    <div v-else-if="loadError || !item" class="share-empty">
      <h1>{{ loadError || '项目不存在或已下线' }}</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-head">
        <a class="share-back" href="/">← 返回</a>
        <div>
          <h1>{{ item.name }}</h1>
          <p>
            <span class="tag">{{ CARD_TYPE_LABELS[type] }}</span>
            <span v-if="card?.ownerName">来自 {{ card.ownerName }} 的名片</span>
          </p>
        </div>
      </header>

      <section class="share-stage">
        <iframe
          v-if="type === 'html' && htmlSource"
          :src="htmlSource"
          title="HTML 游戏预览"
          sandbox="allow-scripts allow-forms"
        />
        <img v-else-if="type === 'image' && mediaSource" :src="mediaSource" :alt="item.name" />
        <iframe
          v-else-if="type === 'pdf' && mediaSource"
          :src="pdfUrl"
          title="PDF 预览"
          referrerpolicy="no-referrer"
        />
        <video
          v-else-if="type === 'video' && mediaSource"
          :src="mediaSource"
          controls
          playsinline
        />
        <article
          v-else-if="type === 'document' && mediaSource"
          class="share-document"
          v-html="documentHTML"
        />
        <div v-else class="share-empty">
          <p>暂无可预览内容</p>
        </div>
      </section>

      <footer class="share-foot">
        <a
          v-if="htmlSource"
          class="card-secondary"
          :href="htmlSource"
          target="_blank"
          rel="noopener noreferrer"
        >
          打开原项目
        </a>
        <a v-if="card?.id" class="card-primary" :href="`/share/card/${card.id}`">查看名片</a>
      </footer>
    </template>
  </main>
</template>

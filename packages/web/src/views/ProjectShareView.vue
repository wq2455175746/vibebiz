<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'

import { DEMO_DOCUMENT, DEMO_HTML, DEMO_IMAGE, DEMO_PDF, DEMO_VIDEO } from '@/mocks/cards'
import { useCardStore } from '@/stores/card'
import type { CardContentType } from '@/types'
import { CARD_TYPE_LABELS } from '@/utils/card'
import { renderMarkdown } from '@/utils/markdown'
import '@/styles/card.css'

const route = useRoute()
const store = useCardStore()

store.load()

const found = computed(() => store.findProject(String(route.params.projectId ?? '')))
const item = computed(() => found.value?.item ?? null)
const card = computed(() => found.value?.card ?? null)
const type = computed<CardContentType>(() => item.value?.type ?? 'html')

const htmlExternal = computed(() => {
  const url = item.value?.shareUrl ?? ''
  return url.startsWith('http') || url.startsWith('/p/')
})

const htmlSource = computed(() => (htmlExternal.value ? (item.value?.shareUrl ?? '') : DEMO_HTML))

const mediaSource = computed(() => {
  if (!item.value) return ''
  if (item.value.previewUrl) return item.value.previewUrl
  if (type.value === 'image') return DEMO_IMAGE
  if (type.value === 'pdf') return DEMO_PDF
  if (type.value === 'video') return DEMO_VIDEO
  return ''
})

const documentHTML = computed(() => renderMarkdown(item.value?.previewUrl || DEMO_DOCUMENT))
</script>

<template>
  <main class="share-page">
    <div v-if="!item" class="share-empty">
      <h1>项目不存在或已下线</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-head">
        <a class="share-back" href="/">← 返回</a>
        <div>
          <h1>{{ item.name }}</h1>
          <p>
            <span class="tag">{{ CARD_TYPE_LABELS[type] }}</span>
            <span v-if="card">来自 {{ card.ownerName }} 的名片</span>
          </p>
        </div>
      </header>

      <section class="share-stage">
        <iframe
          v-if="type === 'html' && htmlExternal"
          :src="htmlSource"
          title="HTML 游戏预览"
          sandbox="allow-scripts allow-forms"
        />
        <iframe
          v-else-if="type === 'html'"
          :srcdoc="htmlSource"
          title="HTML 演示预览"
          sandbox="allow-scripts allow-forms"
        />
        <img v-else-if="type === 'image'" :src="mediaSource" :alt="item.name" />
        <iframe v-else-if="type === 'pdf'" :src="mediaSource" title="PDF 预览" sandbox="" />
        <video v-else-if="type === 'video'" :src="mediaSource" controls playsinline />
        <article v-else class="share-document" v-html="documentHTML" />
      </section>

      <footer class="share-foot">
        <a v-if="item.shareUrl" class="card-secondary" :href="item.shareUrl" target="_blank" rel="noopener noreferrer">
          打开原项目
        </a>
        <a v-if="card" class="card-primary" :href="`/share/card/${card.id}`">查看名片</a>
      </footer>
    </template>
  </main>
</template>

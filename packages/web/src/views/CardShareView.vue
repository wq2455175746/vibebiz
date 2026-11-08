<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import CardProjectItem from '@/components/card/CardProjectItem.vue'
import CardPreview from '@/components/card/CardPreview.vue'
import { useCardStore } from '@/stores/card'
import type { CardFeatured } from '@/types'
import { dataUrlToBlob } from '@/utils/media'
import '@/styles/card.css'

const route = useRoute()
const router = useRouter()
const store = useCardStore()
const notice = ref('')
const copied = ref(false)

store.load()

const card = computed(() => store.getCardById(String(route.params.id ?? '')))
const isOwnCard = computed(() => route.params.id === 'me')
const shareUrl = computed(() => window.location.href)
const effectiveFeatured = computed<CardFeatured>(() => {
  if (!card.value) return null
  if (card.value.featured) return card.value.featured
  const first = card.value.items[0]
  return first ? { kind: 'project', projectId: first.projectId } : null
})
const isFileFeatured = computed(() => card.value?.featured?.kind === 'file')

function openFeatured() {
  const featured = effectiveFeatured.value
  if (!featured) return
  if (featured.kind === 'project') {
    void router.push({ name: 'project-share', params: { projectId: featured.projectId } })
    return
  }
  const url = URL.createObjectURL(dataUrlToBlob(featured.dataUrl))
  window.open(url, '_blank', 'noopener,noreferrer')
  window.setTimeout(() => URL.revokeObjectURL(url), 60_000)
}

function openProject(projectId: string) {
  window.open(`/share/project/${encodeURIComponent(projectId)}`, '_blank', 'noopener,noreferrer')
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(shareUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    notice.value = '复制失败，请手动复制链接'
  }
}
</script>

<template>
  <main class="share-page">
    <div v-if="!card" class="share-empty">
      <h1>名片不存在或已下线</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-card-head">
        <div class="share-person">
          <img v-if="card.avatarUrl" class="share-avatar" :src="card.avatarUrl" :alt="card.ownerName" />
          <span v-else class="share-avatar">{{ (card.ownerName || '?').charAt(0).toUpperCase() }}</span>
          <div>
            <h1>{{ card.ownerName || '未命名开发者' }}</h1>
            <p>{{ card.headline || '这个人还没有写简介' }}</p>
            <div class="share-tags">
              <span v-for="tag in card.tags" :key="tag" class="tag">{{ tag }}</span>
            </div>
          </div>
        </div>
        <div class="card-share-actions">
          <a v-if="isOwnCard" class="card-primary" href="/my-card">编辑名片</a>
          <button class="card-secondary" type="button" @click="copyLink">
            <i class="pi pi-copy" />
            {{ copied ? '已复制' : '复制分享链接' }}
          </button>
        </div>
      </header>

      <p v-if="notice" class="share-notice">{{ notice }}</p>

      <section
        class="share-featured"
        role="button"
        tabindex="0"
        aria-label="打开预览内容"
        @click="openFeatured"
        @keydown.enter="openFeatured"
      >
        <CardPreview :featured="card.featured" :items="card.items" large />
        <span v-if="isFileFeatured" class="share-featured-hit" aria-hidden="true" />
      </section>

      <section class="share-projects">
        <h2>项目</h2>
        <p v-if="card.items.length === 0" class="card-empty">还没有添加项目。</p>
        <div v-else class="card-project-list">
          <CardProjectItem
            v-for="item in card.items"
            :key="item.projectId"
            :item="item"
            :removable="false"
            @open="openProject(item.projectId)"
          />
        </div>
      </section>

      <footer class="share-social">
        <button class="card-social-comment" type="button" disabled>
          <i class="pi pi-comment" />
          评论 · {{ card.commentCount }}
        </button>
        <button class="card-social-message" type="button" disabled>
          <i class="pi pi-send" />
          私信
        </button>
        <span class="tag">即将上线</span>
      </footer>
    </template>
  </main>
</template>

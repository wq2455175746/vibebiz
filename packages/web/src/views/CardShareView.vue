<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { getPublicCard } from '@/api/cards'
import CardProjectItem from '@/components/card/CardProjectItem.vue'
import CardPreview from '@/components/card/CardPreview.vue'
import { useAuthStore } from '@/stores/auth'
import type { BusinessCard } from '@/types'
import '@/styles/card.css'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const notice = ref('')
const copied = ref(false)
const card = ref<BusinessCard | null>(null)
const loading = ref(true)
const loadError = ref('')

onMounted(async () => {
  try {
    card.value = await getPublicCard(String(route.params.id ?? ''))
  } catch (err) {
    loadError.value = err instanceof Error ? err.message : '名片加载失败'
  } finally {
    loading.value = false
  }
  await auth.ensure().catch(() => undefined)
})

const isOwnCard = computed(() => Boolean(card.value && auth.user?.id === card.value.ownerUserId))
const shareUrl = computed(() => window.location.href)
const isFileFeatured = computed(() => card.value?.featured?.kind === 'file')

function openFeatured() {
  const current = card.value
  if (!current) return
  const featured =
    current.featured ??
    (current.items[0] ? { kind: 'project' as const, projectId: current.items[0].projectId } : null)
  if (!featured) return
  if (featured.kind === 'project') {
    void router.push({ name: 'project-share', params: { projectId: featured.projectId } })
    return
  }
  window.open(featured.url, '_blank', 'noopener,noreferrer')
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
    <div v-if="loading" class="share-empty">
      <h1>加载中…</h1>
    </div>
    <div v-else-if="loadError || !card" class="share-empty">
      <h1>{{ loadError || '名片不存在或已下线' }}</h1>
      <a class="card-primary" href="/">返回首页</a>
    </div>
    <template v-else>
      <header class="share-card-head">
        <div class="share-person">
          <img
            v-if="card.avatarUrl"
            class="share-avatar"
            :src="card.avatarUrl"
            :alt="card.ownerName"
          />
          <span v-else class="share-avatar">{{
            (card.ownerName || '?').charAt(0).toUpperCase()
          }}</span>
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

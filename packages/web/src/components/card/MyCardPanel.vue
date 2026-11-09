<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'

import AddProjectDialog from './AddProjectDialog.vue'
import CardProfilePanel from './CardProfilePanel.vue'
import CardProjectItem from './CardProjectItem.vue'
import { useAuthStore } from '@/stores/auth'
import { useCardStore } from '@/stores/card'
import type { CardProjectItem as CardProjectItemType, PublishedProject } from '@/types'
import { emptyCard } from '@/utils/card'
import '@/styles/card.css'

const auth = useAuthStore()
const store = useCardStore()
const addOpen = ref(false)
const shareOpen = ref(false)
const copied = ref(false)
const actionError = ref('')
const shareUrl = ref('')
const card = computed(() => store.mine ?? emptyCard())

onMounted(async () => {
  await auth.ensure().catch(() => undefined)
  await store.loadMine()
})
onBeforeUnmount(() => void store.flushProfile())

function openProject(item: CardProjectItemType) {
  window.open(`/share/project/${encodeURIComponent(item.projectId)}`, '_blank', 'noopener,noreferrer')
}

async function addProject(published: PublishedProject) {
  actionError.value = ''
  try {
    await store.addProject(published.projectId)
    actionError.value = ''
    addOpen.value = false
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '添加项目失败'
  }
}

async function share() {
  actionError.value = ''
  try {
    const link = await store.publish()
    shareUrl.value = `${window.location.origin}${link}`
    copied.value = false
    shareOpen.value = true
  } catch (err) {
    actionError.value = err instanceof Error ? err.message : '发布失败'
  }
}

async function copyLink() {
  try {
    await navigator.clipboard.writeText(shareUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1500)
  } catch {
    actionError.value = '复制失败，请手动复制链接'
  }
}
</script>

<template>
  <div class="card-page">
    <div v-if="store.loading" class="dialog-state">加载中...</div>

    <template v-else-if="store.mine">
      <header class="card-page-head">
        <div>
          <h1>我的名片</h1>
          <p>{{ store.saving ? '保存中…' : '已保存' }}</p>
        </div>
        <div class="card-page-actions">
          <span v-if="store.error" class="error-text">{{ store.error }}</span>
          <span v-if="!store.hasShareContent" class="card-share-hint">请先添加项目</span>
          <button
            class="card-share-btn"
            type="button"
            :disabled="!store.hasShareContent"
            :title="store.hasShareContent ? '分享名片' : '请先添加项目'"
            @click="share"
          >
            <i class="pi pi-share-alt" />
            分享
          </button>
        </div>
      </header>

      <p v-if="actionError" class="error-text" role="alert">{{ actionError }}</p>

      <div class="card-page-grid">
        <section class="card-content-panel">
          <header class="card-panel-head">
            <div>
              <h2>我的项目</h2>
              <p>从自己已发布的项目中挑选最多 5 个。</p>
            </div>
            <button class="card-add-btn" type="button" :disabled="store.items.length >= 5" @click="addOpen = true">
              <i class="pi pi-plus" />
              添加项目
            </button>
          </header>

          <p v-if="store.items.length === 0" class="card-empty">还没有项目，添加自己已发布的作品吧。</p>
          <div v-else class="card-project-list">
            <CardProjectItem
              v-for="item in store.items"
              :key="item.projectId"
              :item="item"
              @open="openProject"
              @remove="store.removeItem"
            />
          </div>
        </section>

        <CardProfilePanel
          :card="card"
          :uploading-featured="store.uploadingFeatured"
          @update:headline="store.setHeadline"
          @add-tag="store.addTag"
          @remove-tag="store.removeTag"
          @set-featured="store.setFeatured"
          @upload-featured="store.uploadFeatured"
          @clear-featured="store.clearFeatured"
        />
      </div>
    </template>

    <div v-else class="dialog-state">
      <span class="error-text" role="alert">{{ store.error || '名片加载失败' }}</span>
    </div>

    <AddProjectDialog
      v-if="addOpen"
      :existing-ids="store.items.map((item) => item.projectId)"
      :full="store.items.length >= 5"
      :error="actionError"
      @close="addOpen = false"
      @add="addProject"
    />

    <div v-if="shareOpen" class="dialog-backdrop" @click.self="shareOpen = false">
      <div class="dialog-card card-share-dialog">
        <header class="dialog-head">
          <div>
            <p class="dialog-eyebrow">分享名片</p>
            <h2>名片分享页已生成</h2>
          </div>
          <button class="dialog-close" type="button" aria-label="关闭" @click="shareOpen = false">
            <i class="pi pi-times" />
          </button>
        </header>
        <p class="card-share-note">把这个链接发给朋友，就能看到你的项目和个人信息。</p>
        <div class="card-share-link">
          <span>{{ shareUrl }}</span>
          <button type="button" @click="copyLink">
            <i class="pi pi-copy" />
            {{ copied ? '已复制' : '复制' }}
          </button>
        </div>
        <div class="card-share-actions">
          <button type="button" class="card-secondary" @click="shareOpen = false">关闭</button>
          <a class="card-primary" :href="shareUrl" target="_blank" rel="noopener noreferrer">打开分享页</a>
        </div>
      </div>
    </div>
  </div>
</template>

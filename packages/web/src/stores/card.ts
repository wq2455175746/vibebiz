import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import { getProject } from '@/api/projects'
import { mockCards } from '@/mocks/cards'
import { useAuthStore } from '@/stores/auth'
import type {
  BusinessCard,
  CardContentType,
  CardFeatured,
  CardProjectItem,
  PublishedProject,
} from '@/types'
import {
  addCardTag,
  canAddCardItem,
  cardContentType,
  decodeCard,
  decodeCards,
  emptyCard,
  isPdfFile,
  normalizeCard,
  pageCards,
  readFileAsDataUrl,
  sortDiscoverCards,
  validateCardUpload,
} from '@/utils/card'

const MINE_KEY = 'vibebiz:cards:mine:v1'
const DISCOVER_KEY = 'vibebiz:cards:discover:v1'

export const useCardStore = defineStore('card', () => {
  const mine = ref<BusinessCard>(emptyCard())
  const discoverMock = ref<BusinessCard[]>([])
  const loading = ref(false)
  const saving = ref(false)
  const error = ref('')
  let saveTimer: number | undefined

  const items = computed(() => mine.value.items)
  const hasShareContent = computed(() => mine.value.items.length > 0)
  const discoverCards = computed(() =>
    sortDiscoverCards([...(mine.value.published ? [mine.value] : []), ...discoverMock.value]),
  )

  function persistMine() {
    try {
      localStorage.setItem(MINE_KEY, JSON.stringify(mine.value))
      error.value = ''
    } catch {
      error.value = '本地存储空间不足，名片未能保存'
    }
  }

  function persistDiscover() {
    try {
      localStorage.setItem(DISCOVER_KEY, JSON.stringify(discoverMock.value))
    } catch {
      error.value = '本地存储空间不足，发现页数据未能保存'
    }
  }

  function saveNow() {
    saving.value = true
    try {
      mine.value = { ...mine.value, updatedAt: new Date().toISOString() }
      persistMine()
    } finally {
      saving.value = false
    }
  }

  function scheduleSave() {
    saving.value = true
    if (saveTimer) window.clearTimeout(saveTimer)
    saveTimer = window.setTimeout(saveNow, 300)
  }

  function load() {
    loading.value = true
    const auth = useAuthStore()
    const stored = decodeCard(localStorage.getItem(MINE_KEY)) ?? emptyCard()
    mine.value = normalizeCard({
      ...stored,
      ownerUserId: auth.user?.id ?? stored.ownerUserId,
      ownerName: auth.user?.username ?? stored.ownerName,
      avatarUrl: auth.user?.avatarUrl ?? stored.avatarUrl,
    })
    const storedMock = decodeCards(localStorage.getItem(DISCOVER_KEY))
    discoverMock.value = storedMock.length
      ? storedMock
      : mockCards.map((card) => normalizeCard(card))
    if (!storedMock.length) persistDiscover()
    loading.value = false
  }

  function setHeadline(value: string) {
    mine.value = { ...mine.value, headline: value.slice(0, 80) }
    scheduleSave()
  }

  function addTag(raw: string): string {
    const result = addCardTag(mine.value.tags, raw)
    if (!result.error) {
      mine.value = { ...mine.value, tags: result.tags }
      saveNow()
    }
    return result.error
  }

  function removeTag(tag: string) {
    mine.value = { ...mine.value, tags: mine.value.tags.filter((item) => item !== tag) }
    saveNow()
  }

  function removeItem(projectId: string) {
    const nextItems = mine.value.items.filter((item) => item.projectId !== projectId)
    const featured =
      mine.value.featured?.kind === 'project' && mine.value.featured.projectId === projectId
        ? null
        : mine.value.featured
    mine.value = { ...mine.value, items: nextItems, featured }
    saveNow()
  }

  async function addPublishedProject(published: PublishedProject): Promise<CardProjectItem> {
    if (!canAddCardItem(mine.value.items, published.projectId)) {
      throw new Error('项目已添加或已达到 5 个上限')
    }
    let type: CardContentType = 'html'
    try {
      const project = await getProject(published.projectId)
      type = cardContentType(project.previewPath)
    } catch {
      type = 'html'
    }
    if (!canAddCardItem(mine.value.items, published.projectId)) {
      throw new Error('项目已添加或已达到 5 个上限')
    }
    const item: CardProjectItem = {
      projectId: published.projectId,
      name: published.name,
      description: published.description,
      coverUrl: published.coverUrl,
      type,
      shareUrl: published.url,
      addedAt: new Date().toISOString(),
    }
    mine.value = { ...mine.value, items: [...mine.value.items, item] }
    saveNow()
    return item
  }

  function setFeatured(featured: CardFeatured) {
    if (featured?.kind === 'project') {
      const exists = mine.value.items.some((item) => item.projectId === featured.projectId)
      if (!exists) throw new Error('主预览项目必须来自已添加项目')
    }
    mine.value = { ...mine.value, featured }
    saveNow()
  }

  async function setFeaturedFile(file: File): Promise<void> {
    const message = validateCardUpload(file)
    if (message) throw new Error(message)
    const dataUrl = await readFileAsDataUrl(file)
    const isPdf = isPdfFile(file)
    setFeatured({
      kind: 'file',
      fileType: isPdf ? 'pdf' : 'image',
      name: file.name,
      size: file.size,
      dataUrl,
    })
  }

  function clearFeatured() {
    mine.value = { ...mine.value, featured: null }
    saveNow()
  }

  function publish(): BusinessCard {
    if (!hasShareContent.value) throw new Error('请先添加项目')
    mine.value = { ...mine.value, published: true }
    saveNow()
    return mine.value
  }

  function getCardById(id: string): BusinessCard | null {
    if (id === 'me') return mine.value
    return discoverMock.value.find((card) => card.id === id) ?? null
  }

  function findProject(projectId: string): { card: BusinessCard; item: CardProjectItem } | null {
    for (const card of [mine.value, ...discoverMock.value]) {
      const item = card.items.find((entry) => entry.projectId === projectId)
      if (item) return { card, item }
    }
    return null
  }

  function listDiscover(page: number, pageSize = 6): { items: BusinessCard[]; total: number } {
    const all = discoverCards.value
    return { items: pageCards(all, page, pageSize), total: all.length }
  }

  return {
    mine,
    discoverMock,
    loading,
    saving,
    error,
    items,
    hasShareContent,
    discoverCards,
    load,
    setHeadline,
    addTag,
    removeTag,
    removeItem,
    addPublishedProject,
    setFeatured,
    setFeaturedFile,
    clearFeatured,
    publish,
    getCardById,
    findProject,
    listDiscover,
  }
})

import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import {
  addCardProject,
  clearCardFeatured,
  getDiscoverCards,
  getMyCard,
  getPublicCard,
  getPublicCardProject,
  publishMyCard,
  removeCardProject,
  setCardFeaturedProject,
  updateMyCard,
  uploadCardFeatured,
} from '@/api/cards'
import type { BusinessCard, CardFeatured, CardProjectItem, PublicProjectCard } from '@/types'
import { addCardTag, validateCardUpload } from '@/utils/card'

const PROFILE_DEBOUNCE_MS = 400
const DISCOVER_PAGE_SIZE = 12

function messageOf(error: unknown, fallback: string): string {
  return error instanceof Error && error.message ? error.message : fallback
}

export const useCardStore = defineStore('card', () => {
  const mine = ref<BusinessCard | null>(null)
  const loading = ref(false)
  const saving = ref(false)
  const uploadingFeatured = ref(false)
  const error = ref('')

  const discoverCards = ref<BusinessCard[]>([])
  const discoverTotal = ref(0)
  const discoverPage = ref(1)
  const discoverLoading = ref(false)

  const items = computed<CardProjectItem[]>(() => mine.value?.items ?? [])
  const hasShareContent = computed(() => (mine.value?.items.length ?? 0) > 0)

  let profileTimer: number | undefined
  let profilePending = false

  async function flushProfile(): Promise<void> {
    if (profileTimer !== undefined) {
      window.clearTimeout(profileTimer)
      profileTimer = undefined
    }
    if (!profilePending || !mine.value) return
    profilePending = false
    const input = { headline: mine.value.headline, tags: [...mine.value.tags] }
    try {
      mine.value = await updateMyCard(input)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '名片保存失败')
      throw cause
    } finally {
      saving.value = false
    }
  }

  function scheduleProfile(): void {
    if (!mine.value) return
    profilePending = true
    saving.value = true
    if (profileTimer !== undefined) window.clearTimeout(profileTimer)
    profileTimer = window.setTimeout(() => {
      profileTimer = undefined
      void flushProfile().catch(() => undefined)
    }, PROFILE_DEBOUNCE_MS)
  }

  async function loadMine(): Promise<void> {
    loading.value = true
    try {
      mine.value = await getMyCard()
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '名片加载失败')
    } finally {
      loading.value = false
    }
  }

  function setHeadline(value: string): void {
    if (!mine.value) return
    mine.value = { ...mine.value, headline: value.slice(0, 80) }
    scheduleProfile()
  }

  function addTag(raw: string): string {
    if (!mine.value) return ''
    const result = addCardTag(mine.value.tags, raw)
    if (!result.error) {
      mine.value = { ...mine.value, tags: result.tags }
      scheduleProfile()
    }
    return result.error
  }

  function removeTag(tag: string): void {
    if (!mine.value) return
    mine.value = { ...mine.value, tags: mine.value.tags.filter((item) => item !== tag) }
    scheduleProfile()
  }

  async function addProject(projectId: string): Promise<void> {
    try {
      mine.value = await addCardProject(projectId)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '添加项目失败')
      throw cause
    }
  }

  async function removeProject(projectId: string): Promise<void> {
    try {
      mine.value = await removeCardProject(projectId)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '移除项目失败')
      throw cause
    }
  }

  async function setFeatured(featured: CardFeatured): Promise<void> {
    if (featured?.kind !== 'project') throw new Error('unsupported featured')
    try {
      mine.value = await setCardFeaturedProject(featured.projectId)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '设置主预览失败')
      throw cause
    }
  }

  async function uploadFeatured(file: File): Promise<void> {
    const message = validateCardUpload(file)
    if (message) throw new Error(message)
    uploadingFeatured.value = true
    try {
      mine.value = await uploadCardFeatured(file)
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '上传主预览失败')
      throw cause
    } finally {
      uploadingFeatured.value = false
    }
  }

  async function clearFeatured(): Promise<void> {
    try {
      mine.value = await clearCardFeatured()
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '清除主预览失败')
      throw cause
    }
  }

  async function publish(): Promise<string> {
    try {
      const published = await publishMyCard()
      mine.value = published
      error.value = ''
      return `/share/card/${published.id}`
    } catch (cause) {
      error.value = messageOf(cause, '发布失败')
      throw cause
    }
  }

  async function loadDiscover(page = 1, append = false): Promise<void> {
    discoverLoading.value = true
    try {
      const result = await getDiscoverCards({ page, pageSize: DISCOVER_PAGE_SIZE })
      discoverCards.value = append ? [...discoverCards.value, ...result.cards] : result.cards
      discoverTotal.value = result.total
      discoverPage.value = result.page
      error.value = ''
    } catch (cause) {
      error.value = messageOf(cause, '发现页加载失败')
      throw cause
    } finally {
      discoverLoading.value = false
    }
  }

  function fetchPublicCard(id: string): Promise<BusinessCard> {
    return getPublicCard(id)
  }

  function fetchPublicProject(projectId: string): Promise<PublicProjectCard> {
    return getPublicCardProject(projectId)
  }

  return {
    mine,
    loading,
    saving,
    uploadingFeatured,
    error,
    discoverCards,
    discoverTotal,
    discoverPage,
    discoverLoading,
    items,
    hasShareContent,
    loadMine,
    flushProfile,
    setHeadline,
    addTag,
    removeTag,
    addProject,
    removeProject,
    removeItem: removeProject,
    setFeatured,
    setFeaturedProject: (projectId: string) => setFeatured({ kind: 'project', projectId }),
    uploadFeatured,
    clearFeatured,
    publish,
    loadDiscover,
    fetchPublicCard,
    fetchPublicProject,
  }
})
